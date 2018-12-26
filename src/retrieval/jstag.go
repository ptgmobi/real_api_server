package retrieval

import (
	"ad"
	"net/http"

	dnf "github.com/brg-liuwei/godnf"

	"http_context"
)

var touTiaoOfferPkgs = []string{"517166184", "529092160", "1086047750", "1142110895", "11334496215"}

// 判断offer是否是头条系offer
func findTouTiaoOffer(ad *ad.NativeAdObj) bool {
	for i := 0; i < len(touTiaoOfferPkgs); i++ {
		if ad.PkgName == touTiaoOfferPkgs[i] {
			return true
		}
	}
	return false
}

func (s *Service) jstagHandler(w http.ResponseWriter, r *http.Request) {
	s.stat.IncrTot()
	s.stat.GetJstagStat().IncrTot()

	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		s.l.Println("jstagHandler new context err: ", err)
		resp := NewRtvResp("url parameters error", 42, nil)
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[jstag] context err resp write: ", n, ", error:", err)
		}
		s.stat.GetJstagStat().IncrCtxErr()
		return
	}

	ctx.Estimate("BeginJstagAd: " + ctx.Platform)
	defer func() {
		ctx.Estimate("End")
		ctx.LogEstimate()
	}()

	tpl := s.getTpl(ctx.SlotId)
	ctx.Estimate("GetTpl: " + ctx.SlotId)
	if tpl == nil {
		ctx.Phase = "JstagTplNil"
		if n, err := NewRtvResp("tpl empty", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[jstag] no match tpl resp write: ", n, ", error:", err)
		}
		s.stat.GetJstagStat().IncrTplNoMatch()
		return
	}

	if tpl.SlotSwitch == 2 {
		// slot switch 2: closed
		ctx.Phase = "JstagTplClosed"
		if n, err := NewRtvResp("slot closed", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[jstag] slot closed resp write: ", n, ", error:", err)
		}
		s.stat.GetJstagStat().IncrTplNoMatch()
		return
	}
	handler := dnf.GetHandler()
	if handler == nil {
		ctx.Phase = "JstagDnfLoading"
		resp := NewRtvResp("dnf_handler nil", 2, ctx)
		resp.CK = ctx.Ck
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[jstag] dnf nil resp write: ", n, ", error:", err)
		}
		s.stat.GetJstagStat().IncrDnfNil()
		return
	}

	if !tpl.SlotImpInCap(tpl.MaxImpression) {
		if n, err := NewRtvResp("slot reach impression cap", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[Jstag] slot reach impression cap resp write: ", n, ", error:", err)
		}
		return
	}

	conds := s.makeRetrievalConditions(ctx)

	ctx.Estimate("GenRetrievalConditions: " + dnf.ConditionsToString(conds))

	s.SetCtxTks(ctx)

	fuyuAds := s.getFuyuAds(ctx, handler, conds, ctx.AdNum)

	if len(fuyuAds) == 0 {
		resp := NewRtvResp("No ads (fy zero)", 1, ctx)
		resp.CK = ctx.Ck
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[Jstag] no ads resp write: ", n, ", error:", err)
		}
		ctx.Phase = "JstagNoAds"
		s.stat.GetJstagStat().IncrRetrievalFilted()
		return
	}

	ctx.Phase = "JstagOK"
	resp := NewRtvResp("ok", 0, ctx)
	resp.CK = ctx.Ck

	noTouTiaoOfferIndex := 0

	for i := 0; i < len(fuyuAds); i++ {
		if noTouTiaoOfferIndex == 0 && !findTouTiaoOffer(fuyuAds[i]) { // 找到非头条的offer
			noTouTiaoOfferIndex = i
		}
		resp.AdList = append(resp.AdList, fuyuAds[i])
		resp.offers = append(resp.offers, fuyuAds[i].UniqId)
	}

	if noTouTiaoOfferIndex != 0 { // 将非头条offer放在第一位
		resp.AdList[0], resp.AdList[noTouTiaoOfferIndex] = resp.AdList[noTouTiaoOfferIndex], resp.AdList[0]
	}

	if len(resp.AdList) > 0 {
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[jstag] resp write: ", n, ", error:", err)
		}
		ctx.Estimate("WriteTo")
		s.stat.GetJstagStat().IncrImp()
		return
	}

	ctx.Phase = "JstagZero"
	resp = NewRtvResp("no jstag ad suggestted", 6, ctx)
	resp.CK = ctx.Ck
	if n, err := resp.WriteTo(w); err != nil {
		s.l.Println("[jstag] no wugan ad resp write: ", n, ", error:", err)
	}
	s.stat.GetJstagStat().IncrWuganFilted()
	return
}
