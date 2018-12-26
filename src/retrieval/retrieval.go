package retrieval

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"net/http"
	"strconv"

	dnf "github.com/brg-liuwei/godnf"

	"aes"
	"cpt"
	"http_context"
	"rank"
	"raw_ad"
	"status"
	"util"
)

type tplResp struct {
	Err      string `json:"error"`
	Tpl      string `json:"template"`
	PreClick bool   `json:"pre_click"`
}

type rtvResp struct {
	ErrMsg string        `json:"err_msg"`
	ErrNo  int           `json:"err_no"`
	AdList []interface{} `json:"ad_list"`
	CK     string        `json:"ck,omitempty"`

	Subscriptions []interface{} `json:"subscription,omitempty"`

	Headers map[string]string `json:"-"`

	offers []string
	ctx    *http_context.Context

	Demsg interface{} `json:"demsg,omitempty"`
}

func NewRtvResp(errMsg string, errNo int, ctx *http_context.Context) *rtvResp {
	return &rtvResp{
		ErrMsg:  errMsg,
		ErrNo:   errNo,
		AdList:  make([]interface{}, 0),
		Headers: make(map[string]string),
		ctx:     ctx,
		offers:  make([]string, 0),
	}
}

func (resp *rtvResp) WriteTo(w http.ResponseWriter) (int, error) {
	status.RecordReq(resp.ctx, resp.offers)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Err-Msg", resp.ErrMsg)
	if resp.ctx != nil {
		w.Header().Set("No-Fuyu", strconv.Itoa(resp.ctx.Nfy))
		w.Header().Set("No-Rtv", strconv.Itoa(resp.ctx.Nrtv))
	}
	w.Header().Set("No-Rep", strconv.Itoa(len(resp.AdList)))

	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}

	if resp.ctx != nil {
		resp.Demsg = resp.ctx.GetDemsg()
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

	if resp.ctx.UseGzip && len(b) > 1000 {
		enc := gzip.NewWriter(w)
		defer enc.Close()
		w.Header().Set("Content-Encoding", "gzip")
		return enc.Write(b)
	}
	return w.Write(b)
}

func (s *Service) retrievalHandler(w http.ResponseWriter, r *http.Request) {
	s.stat.GetSdkStat().IncrTot()
	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		s.l.Println("retrievalHandler new context err: ", err)
		if n, err := NewRtvResp(err.Error(), 42, nil).WriteTo(w); err != nil {
			s.l.Println("[retrieval] context resp write", n, "error: ", err)
		}
		s.stat.GetSdkStat().IncrCtxErr()
		return
	}

	ctx.Estimate("BeginAd: " + ctx.Platform)
	defer func() {
		ctx.Estimate("End")
		ctx.LogEstimate()
	}()

	handler := dnf.GetHandler()
	if handler == nil {
		ctx.Phase = "RenderDnfLoading"
		if n, err := NewRtvResp("dnf_handler nil", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[retrieval] dnf nil resp write", n, "error: ", err)
		}
		s.stat.GetSdkStat().IncrDnfNil()
		return
	}

	tpl := s.getTpl(ctx.SlotId)
	ctx.Estimate("GetTpl: " + ctx.SlotId)

	if tpl == nil {
		ctx.Phase = "RenderNoMatchTpl"
		if n, err := NewRtvResp("no match tpl", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[retrieval] tpl resp write: ", n, ", error:", err)
		}
		s.stat.GetSdkStat().IncrTplNoMatch()
		return
	}

	if tpl.SlotSwitch == 2 {
		// slot switch 2: closed
		ctx.Phase = "RenderSlotClosed"
		if n, err := NewRtvResp("slot closed", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[retrieval] slot closed resp write: ", n, ", error:", err)
		}
		s.stat.GetSdkStat().IncrTplNoMatch()
		return
	}

	// slot层级的曝光控制
	if ctx.AdType == "0" || ctx.AdType == "1" || ctx.AdType == "2" {
		if rand.Float64() > tpl.ImpressionRate {
			ctx.Phase = "RenderImpCtrl"
			if n, err := NewRtvResp("no match ad", 5, ctx).WriteTo(w); err != nil {
				s.l.Println("[retrieval] slot improssion rate control resp write: ", n, ", error:", err)
			}
			s.stat.GetSdkStat().IncrImpRateFilted()
			return
		}
	}

	if len(tpl.Templates) == 0 {
		ctx.Phase = "RenderTplSizeNoMatch"
		if n, err := NewRtvResp("tpl size 0", 6, ctx).WriteTo(w); err != nil {
			s.l.Println("[retrieval] tpl size 0, resp write: ", n, ", error:", err)
		}
		s.stat.GetSdkStat().IncrTplSizeErr()
		return
	}

	ctx.Template = &tpl.Templates[0]
	ctx.H5tpl = []byte(tpl.Templates[0].H5)
	ctx.ImgW, ctx.ImgH = tpl.Templates[0].Size()

	if !tpl.SlotImpInCap(tpl.MaxImpression) {
		if n, err := NewRtvResp("slot reach impression cap", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[retrieval] slot reach impression cap resp write: ", n, ", error:", err)
		}
		return
	}

	conds := s.makeRetrievalConditions(ctx)
	ctx.Estimate("GenRetrievalConditions: " + dnf.ConditionsToString(conds))

	pacing := s.LoadPacing()

	if err := ctx.GetFreq(); err != nil {
		s.l.Println("[retrieval] GetFreq error: ", err)
	}

	docs, _ := handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)

        if raw.IsT {
            return false
        }

		// 通用广告搜索
		if msg, ok := s.adSearch(raw, ctx, tpl); !ok {
			ctx.Debug(raw.UniqId, msg)
			return false
		}

		if ctx.IntegralWall {
			icons := raw.Icons["ALL"]
			for i := 0; i < len(icons); i++ {
				if s.IsDefIcon(raw.Platform, icons[i].Url) {
					ctx.Debug(raw.UniqId, "icon invalid")
					return false
				}
			}
		}

		if len(ctx.Carrier) > 0 {
			if !raw.IsHitCarrier(ctx.Carrier) {
				ctx.Debug(raw.UniqId, "carrier not hit")
				return false
			}
		}

		if ctx.IsWugan() {
			if raw.PayoutType == "CPL" {
				// wugan disable cpl ads
				return false
			}
			// preclick概率控制
			if rand.Float64() >= raw.PreRate {
				return false
			}

			// preclick频次控制
			preKey := raw.AppDownload.PkgName
			if !ctx.PreClickInFreqCap(preKey, 1) {
				return false
			}

			// 点击速度控制
			if pacing.OverCap(raw.UniqId, ctx.Country, raw.Pacing, ctx.Now) {
				return false
			}
		} else {

			// 曝光频次
			if inCap, _ := ctx.InFreqCap(raw.AppDownload.PkgName, 1); !inCap {
				ctx.Debug(raw.UniqId, "pkg frequency")
				return false
			}
			// 必须要有素材
			if !raw.HasMatchedCreative(ctx) {
				ctx.Debug(raw.UniqId, "has no matched creatives")
				return false
			}
			// 搜索接口
			if !s.matchKeyWords(raw, ctx) {
				ctx.Debug(raw.UniqId, "not matched keywords")
				return false
			}

			cat := "all"
			if len(raw.AppCategory) > 0 {
				cat = raw.AppCategory[0]
			}

			// adcat == 1代表请求游戏广告
			if ctx.AdCat == "1" && cat != "game" {
				ctx.Debug(raw.UniqId, "not game cat")
				return false
			}

			// adcat == 2代表请求工具广告
			if ctx.AdCat == "2" && cat != "tool" {
				ctx.Debug(raw.UniqId, "not tool cat")
				return false
			}
		}

		return true
	})

	useCpt := false
	if !ctx.IsWugan() {
		cptDocs := cpt.GetCptDocs(conds, ctx)
		if len(cptDocs) > 0 {
			useCpt = true
			docs = cptDocs
		}
	}

	if !useCpt {
		util.Shuffle(docs)
	}

	ctx.Nrtv = len(docs)

	ctx.Estimate("SearchNDocs: " + strconv.Itoa(len(docs)) + ", uid: " + ctx.UserId)
	ctx.Phase = "RenderRetrieval"

	if len(docs) == 0 && ctx.IsWugan() {
		pacing = s.LoadFuyuPacing() // 更新pacing计数器，免得把服预的点击记到正常投放的pacing中了
		docs = s.getFuyuDocsWithShuffle(ctx, handler, conds)
	}

	ndocs := len(docs)

	if ndocs == 0 {
		if n, err := NewRtvResp("No ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[retrieval] no ads, resp write: ", n, ", error:", err)
		}
		if ctx.IsWugan() && ctx.Platform == "iOS" && ctx.Country == "CN" {
			ctx.Phase = "CNiOSWuganEmpty"
		}
		s.stat.GetSdkStat().IncrRetrievalFilted()
		return
	}

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
		rawList = append(rawList, rawAdInter.(*raw_ad.RawAdObj))
	}
	ctx.Estimate("RawListPointerCopy-" + strconv.Itoa(ndocs))

	raws := rank.Select(rawList, ctx)
	ctx.Estimate("Rank-" + strconv.Itoa(len(raws)))

	if len(raws) == 0 {
		ctx.Phase = "RenderRankZero"
		if n, err := NewRtvResp("No ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[retrieval] rank get no ads, resp write: ", n, ", error:", err)
		}
		s.stat.GetSdkStat().IncrRankFilted()
		return
	}

	s.SetCtxTks(ctx)

	ctx.Phase = "RenderOK"
	resp := NewRtvResp("ok", 0, ctx)
	for _, raw := range raws {
		if adv := raw.ToAd(ctx); adv != nil {
			if ctx.IsWugan() {
				ctx.FreqInfo.PFreqFields = append(ctx.FreqInfo.PFreqFields, raw.AppDownload.PkgName)
				// 添加速度计数
				pacing.Add(raw.UniqId, ctx.Now, 1)
			}
			// 增加曝光频次
			ctx.AdPkgName = append(ctx.AdPkgName, raw.AppDownload.PkgName)
			resp.AdList = append(resp.AdList, adv)
			resp.offers = append(resp.offers, raw.UniqId)
		}
	}
	ctx.Estimate("ToAds")

	if err := ctx.IncrPreClickFreq(); err != nil {
		s.l.Println("[retrieval] IncrPreClickFreq error: ", err)
	}

	if len(resp.AdList) > 0 {
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[retrieval] resp write: ", n, ", error:", err)
		}
		ctx.Estimate("WriteTo")
		s.stat.GetSdkStat().IncrImp()
		return
	}

	ctx.Phase = "RenderZero"
	if n, err := NewRtvResp("no ad suggestted", 6, ctx).WriteTo(w); err != nil {
		s.l.Println("[retrieval] no wugan ad resp write: ", n, ", error:", err)
	}
	s.stat.GetSdkStat().IncrWuganFilted()
	return
}
