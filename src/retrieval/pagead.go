package retrieval

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"

	dnf "github.com/brg-liuwei/godnf"

	"http_context"
	"rank"
	"raw_ad"
)

type pageadResp struct {
	ErrMsg string        `json:"err_msg"`
	ErrNo  int           `json:"err_no"`
	AdList []interface{} `json:"ad_list"`

	UseGzip bool `json:"-"`

	body   []byte
	offers []string
	ctx    *http_context.Context

	Demsg interface{} `json:"demsg,omitempty"`
}

func NewPageadResp(errMsg string, errNo int, ctx *http_context.Context) *pageadResp {
	return &pageadResp{
		ErrMsg: errMsg,
		ErrNo:  errNo,
		ctx:    ctx,
	}
}

func PageadFormat(adtype string, w, h int) (key string) {
	switch adtype {
	case "16":
		key = fmt.Sprintf("banner_%dx%d", w, h)
	case "17":
		if w > h {
			key = "interstitial_landscape"
		} else {
			key = "interstitial_portrait"
		}
	}
	return
}

func (p *pageadResp) WriteTo(w http.ResponseWriter) (int, error) {
	w.Header().Set("Content-Type", "application/json; charset=utf8")
	w.Header().Set("Err-Msg", p.ErrMsg)
	if p.ErrNo != 0 {
		b, _ := json.Marshal(p)
		return w.Write(b)
	}
	if p.ctx != nil && p.ctx.Output == "html" {
		w.Header().Set("Content-Type", "text/html; charset=utf8")
		return w.Write(p.body)
	}

	if p.ctx != nil {
		p.Demsg = p.ctx.GetDemsg()
	}

	b, _ := json.Marshal(p)
	if p.ctx != nil && p.ctx.UseGzip && len(b) > 1000 {
		enc := gzip.NewWriter(w)
		defer enc.Close()
		w.Header().Set("Content-Encoding", "gzip")
		return enc.Write(b)
	}
	return w.Write(b)
}

func (s *Service) pageadHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		s.l.Println("page handler new context err: ", err)
		if n, err := NewPageadResp("page handler new context error "+err.Error(), 42, nil).WriteTo(w); err != nil {
			s.l.Println("[pagead] context err resp write: ", n, ", error: ", err)
		}
		return
	}

	handler := dnf.GetHandler()
	if handler == nil {
		if n, err := NewPageadResp("dnf handler nil", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[pagead] dnf handler nil resp write: ", n, ", error: ", err)
		}
		return
	}

	tpl := s.getTpl(ctx.SlotId)
	if tpl == nil {
		if n, err := NewPageadResp("no match tpl", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[pagead] no match tpl resp write: ", n, ", error: ", err)
		}
		return
	}

	if tpl.SlotSwitch == 2 {
		if n, err := NewPageadResp("slot closed", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[pagead] slot closed resp write: ", n, ", error: ", err)
		}
		return
	}

	if ctx.AdType != "16" && ctx.AdType != "17" {
		s.l.Println("page not support adtype ", ctx.AdType)
		if n, err := NewPageadResp("page not support adtype "+ctx.AdType, 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[pagead] not support adtype ", ctx.AdType, " resp write: ", n, " error: ", err)
		}
		return
	}

	// pagead广告类型
	ctx.Format = PageadFormat(ctx.AdType, ctx.AdW, ctx.AdH)
	// 获取pagead适配器
	ctx.Adapters, ctx.Control = tpl.GetPageadTpl(ctx.Format)
	if len(ctx.Adapters) == 0 {
		if n, err := NewPageadResp("ad has no adapter", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[pagead] slot has no adapters ", ctx.SlotId, " resp write: ", n, " error: ", err)
		}
		return
	}

	if rand.Float64() > tpl.ImpressionRate {
		if n, err := NewPageadResp("no matched ad", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[pagead] slot impression rate resp write: ", n, ", error: ", err)
		}
		return
	}

	// 获取广告频控数据
	if err := ctx.GetFreq(); err != nil {
		s.l.Println("[pagead] get frequency error ", err)
	}

	// channel 类型
	channelType := raw_ad.CT_BANNER
	if ctx.AdType == "17" {
		channelType = raw_ad.CT_INTERSTITIAL
	}

	conds := s.makeRetrievalConditions(ctx)
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

		// 曝光频次
		if inCap, _ := ctx.InFreqCap(raw.AppDownload.PkgName, 5); !inCap {
			ctx.Debug(raw.UniqId, "imp frequency")
			return false
		}

		// slot black cat
		if tpl.AppCategoryInBlackList(raw.AppCategory) {
			ctx.Debug(raw.UniqId, "slot balck cat")
			return false
		}

		if !raw.IsHitChannelType(channelType) {
			ctx.Debug(raw.UniqId, "channel type")
			return false
		}

		//  根据tpl里的模板匹配，匹配一种即可
		if !raw.HasMatchedAdapter(ctx) {
			ctx.Debug(raw.UniqId, "not matched creatives")
			return false
		}

		return true
	})

	ndocs := len(docs)
	if ndocs == 0 {
		if n, err := NewPageadResp("no ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[pagead] banner no ads, resp write: ", n, ", error: ", err)
		}
		return
	}

	docCnt, listCap := 0, ndocs
	rawList := make([]*raw_ad.RawAdObj, 0, ndocs)
	for i := 0; i != ndocs && docCnt < listCap; i++ {
		docCnt++
		rawAdInter, _ := handler.DocId2Attr(docs[i])
		rawList = append(rawList, rawAdInter.(*raw_ad.RawAdObj))
	}

	// rank 排序
	raws := rank.Select(rawList, ctx)
	if len(raws) == 0 {
		if n, err := NewPageadResp("no rank ads", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[pagead] no rank ads, resp write: ", n, ", error: ", err)
		}
		return
	}

	ctx.StaticBaseUrl = s.conf.PageadStaticBaseUrl
	s.SetCtxTks(ctx)

	resp := NewPageadResp("ok", 0, ctx)
	for _, raw := range raws {
		if adv := raw.ToPageadBanner(ctx); adv != nil {
			resp.body = adv.PageadObj.HtmlBody
			resp.AdList = append(resp.AdList, adv)
			resp.offers = append(resp.offers, raw.UniqId)
			break
		}
	}

	if len(resp.offers) > 0 {
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[pagead] banner resp write: ", n, ", error: ", err)
		}
		return
	}

	if n, err := NewPageadResp("no ad suggested", 6, ctx).WriteTo(w); err != nil {
		s.l.Println("[pagead] banner no ad suggested resp write: ", n, ", error: ", err)
	}
	return
}
