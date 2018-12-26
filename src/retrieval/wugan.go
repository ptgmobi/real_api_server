package retrieval

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	dnf "github.com/brg-liuwei/godnf"

	"ad"
	"aes"
	"http_context"
	"rank"
	"raw_ad"
	"status"
	"subscription"
	"util"
)

type wuganResp struct {
	Err     string            `json:"error"`
	ErrNo   int               `json:"err_no"`
	Conf    wuganConf         `json:"conf"`
	AdLists [][]interface{}   `json:"ad_lists"`
	Headers map[string]string `json:"-"`
	natList []interface{}     `json:"-"`

	Subscriptions []interface{} `json:"subscription,omitempty"`
	SubsDone      int           `json:"-"` //

	ctx    *http_context.Context
	offers []string
}

type wuganConf struct {
	NViews       int      `json:"views"`
	BlackDomains []string `json:"black_domains"`
}

func NewWuganResp(errMsg string, errNo int, ctx *http_context.Context) *wuganResp {
	nWebViews := 3 // default value
	if errNo != 0 {
		nWebViews = 0
	}

	conf := wuganConf{
		NViews:       nWebViews,
		BlackDomains: []string{"briskads.go2affise.com", "ads.gold", "b.trafficad.net", "rnpol.adsb4trk.com"},
	}

	return &wuganResp{
		Err:     errMsg,
		ErrNo:   errNo,
		Conf:    conf,
		AdLists: make([][]interface{}, conf.NViews),
		Headers: make(map[string]string),
		natList: make([]interface{}, 0, 32),

		ctx:    ctx,
		offers: make([]string, 0),
	}
}

func (resp *wuganResp) WriteTo(w http.ResponseWriter) (int, error) {
	status.RecordReq(resp.ctx, resp.offers)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Err-Msg", resp.Err)
	if resp.ctx != nil {
		w.Header().Set("No-Fuyu", strconv.Itoa(resp.ctx.Nfy))
		w.Header().Set("No-Rtv", strconv.Itoa(resp.ctx.Nrtv))
	}
	w.Header().Set("Subs-Done", strconv.Itoa(resp.SubsDone))

	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}

	totalAds := len(resp.natList)
	w.Header().Set("No-Rep", strconv.Itoa(totalAds))

	adsPerView := 0
	if resp.Conf.NViews > 0 {
		adsPerView = totalAds / resp.Conf.NViews
		if totalAds%resp.Conf.NViews != 0 {
			adsPerView += 1
		}
	}

	nAds := 0
	for view := 0; view != resp.Conf.NViews; view++ {
		for i := 0; i < adsPerView; i++ {
			if nAds >= totalAds {
				break
			}
			resp.AdLists[view] = append(resp.AdLists[view], resp.natList[nAds])
			nAds++
		}
	}

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

		if resp.ctx.UseAes { // 加密
			b = aes.EncryptBytes(buf.Bytes())
			if resp.ctx.TestEncode != "hex" {
				w.Header().Set("CT-Encrypt", "binary")
				decodeBytes := make([]byte, hex.DecodedLen(len(b)))
				hex.Decode(decodeBytes, b)
				return w.Write(decodeBytes)
			} else {
				w.Header().Set("CT-Encrypt", "hex")
			}
		}
		return w.Write(b)
	}

	if resp.ctx != nil && resp.ctx.UseAes {
		b = aes.EncryptBytes(b)
	}
	if resp.ctx != nil && resp.ctx.UseGzip && len(b) > 1000 {
		enc := gzip.NewWriter(w)
		defer enc.Close()
		w.Header().Set("Content-Encoding", "gzip")
		return enc.Write(b)
	}
	return w.Write(b)
}

/*
See (https://github.com/cloudadrd/Document/blob/master/adserver_cache.md) for detail
*/
func (s *Service) wuganHandler(w http.ResponseWriter, r *http.Request) {
	s.stat.GetWuganStat().IncrTot()
	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		s.l.Println("wuganHandler new context err: ", err)
		if n, err := NewWuganResp("url parameters error", 42, nil).WriteTo(w); err != nil {
			s.l.Println("[wugan] context err resp write: ", n, ", error:", err)
		}
		s.stat.GetWuganStat().IncrCtxErr()
		return
	}

	ctx.Estimate("BeginWuganAd: " + ctx.Platform)
	defer func() {
		ctx.Estimate("End")
		ctx.LogEstimate()
	}()

	handler := dnf.GetHandler()
	if handler == nil {
		ctx.Phase = "WuganDnfLoading"
		if n, err := NewWuganResp("dnf_handler nil", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[wugan] dnf nil resp write:", n, ", error:", err)
		}
		s.stat.GetWuganStat().IncrDnfNil()
		return
	}

	tpl := s.getTpl(ctx.SlotId)
	ctx.Estimate("GetTpl:" + ctx.SlotId)
	if tpl == nil {
		NewWuganResp("slot error", 1, ctx).WriteTo(w)
		return
	}

	numPerView := tpl.GetPreNum()
	if numPerView == 0 {
		NewWuganResp("no tpl ads", 1, ctx).WriteTo(w)
		return
	}
	ctx.AdNum = numPerView * 3 // 3 WebViews

	if tpl == nil {
		ctx.Phase = "WuganTplNil"
		if n, err := NewWuganResp("no match tpl", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[wugan] no match tpl resp write:", n, ", error:", err)
		}
		s.stat.GetWuganStat().IncrTplNoMatch()
		return
	}

	if tpl.SlotSwitch == 2 {
		// slot switch 2: closed
		ctx.Phase = "WuganSlotClosed"
		if n, err := NewWuganResp("slot closed", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[wugan] slot closed resp write:", n, ", error:", err)
		}
		s.stat.GetWuganStat().IncrTplNoMatch()
		return
	}

	if tpl.PreClick != 1 {
		s.stat.GetWuganStat().IncrWuganFilted()
		ctx.Phase = "WuganSlotPreclickClosed"
		if n, err := NewWuganResp("slot cache closed", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[wugan] slot wugan closed, resp write:", n, ", error:", err)
		}
		return
	}

	conds := s.makeRetrievalConditions(ctx)
	ctx.Estimate("GenRetrievalConditions: " + dnf.ConditionsToString(conds))

	condsBak := conds
	if ctx.Platform == "Android" { // XXX 安卓无感不要了， 这里省的dnf检索
		conds = make([]dnf.Cond, 0, 1)
	}

	pacing := s.LoadPacing()

	if err := ctx.GetFreq(); err != nil {
		s.l.Println("[wugan] GetFreq error: ", err)
	}

	docs, _ := handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)

		if _, ok := s.adSearch(raw, ctx, tpl); !ok {
			return false
		}

		if raw.PkgPreClickEnable == 2 { // 报名预点击控制
			return false
		}

		if ctx.Country == "CN" && ctx.Platform == "iOS" {
			if !util.WuganHitChannel(raw.Channel) {
				return false
			}
			// XXX 临时定向
			if raw.Channel == "gdt" && ctx.SdkVersion < "i-2.7.3" {
				return false
			}
		}

		if pacing.OverCap(raw.UniqId, ctx.Country, raw.Pacing, ctx.Now) {
			return false
		}

		// preclick频次控制
		preKey := raw.AppDownload.PkgName
		if !ctx.PreClickInFreqCap(preKey, 1) ||
			rand.Float64() >= raw.PreRate {
			return false
		}

		return true
	})
	ctx.Estimate("SearchNDocs: " + strconv.Itoa(len(docs)) + ", uid: " + ctx.UserId)
	ctx.Phase = "WuganRetrieval"

	ndocs := len(docs)
	ctx.Nrtv = ndocs

	util.Shuffle(docs)

	s.SetCtxTks(ctx)

	wuganAds := make([]*ad.NativeAdObj, 0, ctx.AdNum)
	if ndocs > 0 {
		docCnt := 0

		listCap := ndocs
		if listCap > rankLimitCap {
			listCap = rankLimitCap
		}
		if listCap > rankUseGzipCap {
			ctx.RankUseGzip = true
		} else {
			ctx.RankUseGzip = false
		}
		rawList := make([]*raw_ad.RawAdObj, 0, listCap)

		for i := 0; i != ndocs && docCnt < listCap; i++ {
			docCnt++
			rawAdInter, _ := handler.DocId2Attr(docs[i])
			raw := rawAdInter.(*raw_ad.RawAdObj)
			rawList = append(rawList, raw)
		}
		ctx.Estimate("RawListPointerCopy-" + strconv.Itoa(ndocs))

		// XXX 临时策略：减少ym的几率
		rawNotym := make([]*raw_ad.RawAdObj, 0, len(rawList))
		for _, raw := range rawList {
			if !strings.HasSuffix(raw.Channel, "ym") {
				rawNotym = append(rawNotym, raw)
			}
		}

		// XXX 30%的概率不出ym系列
		if len(rawNotym) > 0 && rand.Float32() < 0.3 {
			rawList = rawNotym
		}

		raws := rank.SelectWg(rawList, ctx)

		ctx.Estimate("Rank: " + strconv.Itoa(len(rawList)))

		for _, raw := range raws {
			if adv := raw.ToNativeAd(ctx, 0); adv != nil {
				adv.Core = ad.NativeAdObjCore{}
				// 添加到pre名单中，增加频次
				ctx.FreqInfo.PFreqFields = append(ctx.FreqInfo.PFreqFields, raw.AppDownload.PkgName)
				// 增加速度计数
				pacing.Add(raw.UniqId, ctx.Now, 1)
				wuganAds = append(wuganAds, adv)
			}
		}
		ctx.Estimate("ToWuganAds")
	}

	fuyuCap := ctx.AdNum - len(wuganAds)
	fuyuAds := s.getFuyuAds(ctx, handler, conds, fuyuCap)
	wuganAds = append(wuganAds, fuyuAds...)

	moreCap := ctx.AdNum - len(wuganAds)
	if ctx.Platform == "iOS" && len(fuyuAds) > 0 && moreCap > 0 {
		// len(fuyuAds) > 0说明该地区有fuyu offer
		fuyuIds := util.GetFuyuDevices(ctx.Platform, ctx.Country, 1)
		if len(fuyuIds) > 0 {
			ctx.FuyuUserId = fuyuIds[0]
			moreFuyuAds := s.getFuyuAds(ctx, handler, conds, moreCap)
			wuganAds = append(wuganAds, moreFuyuAds...)
		}
	}

	ctx.Estimate("GetFuyuAds")

	var subs []*subscription.Subscription
	var redirectUrl string
	if ctx.Subs && ctx.Platform == "Android" && len(ctx.UA) > 0 {
		conds = condsBak
		subs, redirectUrl = subscription.GetMoreSubJs(ctx, conds)
	}

	if len(redirectUrl) > 0 { // 需要重定向
		http.Redirect(w, r, redirectUrl+ctx.GetRawPath(), http.StatusFound)
		return
	}

	if len(wuganAds) == 0 && len(subs) == 0 {
		if n, err := NewWuganResp("No ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[wugan] no ads resp write: ", n, ", error:", err)
		}
		ctx.Phase = "WuganNoAds"
		s.stat.GetWuganStat().IncrRetrievalFilted()
		return
	}

	ctx.Phase = "WuganOK"
	resp := NewWuganResp("ok", 0, ctx)

	for _, ad := range wuganAds {
		resp.natList = append(resp.natList, ad)
		resp.offers = append(resp.offers, ad.UniqId)
	}

	if len(subs) > 0 {
		resp.SubsDone = 1
	}
	for _, k := range subs {
		resp.Subscriptions = append(resp.Subscriptions, k)
	}

	if err := ctx.IncrPreClickFreq(); err != nil {
		s.l.Println("[wugan] IncrPreClickFreq error: ", err)
	}

	if len(resp.natList) > 0 || len(resp.Subscriptions) > 0 {
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[wugan] resp write: ", n, ", error:", err)
		}
		ctx.Estimate("WriteTo")
		s.stat.GetWuganStat().IncrImp()
		return
	}

	if n, err := NewWuganResp("no cached ad suggestted", 6, ctx).WriteTo(w); err != nil {
		s.l.Println("[wugan] no wugan ads, resp write: ", n, ", error:", err)
	}
	s.stat.GetWuganStat().IncrWuganFilted()
	return
}
