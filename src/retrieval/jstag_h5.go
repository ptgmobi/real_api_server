package retrieval

import (
	"math/rand"
	"net/http"
	"strings"

	dnf "github.com/brg-liuwei/godnf"

	"ad"
	"http_context"
	"raw_ad"
	"ssp"
	"util"
)

func (s *Service) searchEnable(raw *raw_ad.RawAdObj, ctx *http_context.Context, tpl *ssp.SlotInfo) bool {
	if len(ctx.Channel) != 0 {
		if raw.Channel != ctx.Channel {
			return false
		}
	}

	// 渠道流量比控制
	if rand.Float64() > raw.TrafficRate {
		return false
	}

	// black channel
	if tpl.AdNetworkBlackMap != nil && tpl.AdNetworkBlackMap[raw.Channel] {
		return false
	}

	// 广告类型过滤（目前有googleplaydownload, subscription, ddl三类）
	if !tpl.InWhiteList(raw.ProductCategory) {
		return false
	}

	// black slot
	if tpl.OfferBlackMap != nil && tpl.OfferBlackMap[raw.UniqId] {
		return false
	}

	if !raw.IsHitSlot(ctx.SlotId) {
		return false
	}

	icons := raw.Icons["ALL"]
	for i := 0; i < len(icons); i++ {
		if s.IsDefIcon(raw.Platform, icons[i].Url) {
			return false
		}
	}

	return true
}

func (s *Service) getIntegralWallDocs(ctx *http_context.Context,
	handler *dnf.Handler, conds []dnf.Cond) (docs []int) {

	tpl := s.getTpl(ctx.SlotId)
	ctx.Estimate("GetTpl: " + ctx.SlotId)
	if tpl == nil {
		ctx.Phase = "JstagH5WallTplNil"
		s.l.Println("[JstagH5] Wall, no match tpl, slotId: ", ctx.SlotId)
		s.stat.GetJstagH5Stat().IncrTplNoMatch()
		return
	}

	if tpl.SlotSwitch == 2 {
		// slot switch 2: closed
		ctx.Phase = "JstagH5WallTplClosed"
		s.l.Println("[JstagH5] Wall, slot closed, SlotId: ", ctx.SlotId)
		s.stat.GetJstagH5Stat().IncrTplNoMatch()
		return
	}

	docs, _ = handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)

		if !s.searchEnable(raw, ctx, tpl) {
			return false
		}

		return true
	})

	if len(docs) == 0 {
		ctx.Phase = "JstagH5WallNoAds"
	} else {
		ctx.Phase = "JstagH5WallHit"
	}

	return
}

func (s *Service) getPicDocs(ctx *http_context.Context,
	handler *dnf.Handler, conds []dnf.Cond) (docs []int) {

	tpl := s.getTpl(ctx.SlotId)
	ctx.Estimate("GetTpl: " + ctx.SlotId)
	if tpl == nil {
		ctx.Phase = "JstagH5TplNil"
		s.l.Println("[JstagH5] no match tpl, slotId: ", ctx.SlotId)
		s.stat.GetJstagH5Stat().IncrTplNoMatch()
		return
	}

	if tpl.SlotSwitch == 2 {
		// slot switch 2: closed
		ctx.Phase = "JstagH5TplClosed"
		s.l.Println("[JstagH5] slot closed, slotid: ", ctx.SlotId)
		s.stat.GetJstagH5Stat().IncrTplNoMatch()
		return
	}

	if !tpl.SlotImpInCap(tpl.MaxImpression) {
		s.l.Println("[JstagH5] slot reach impression cap, slotid: ", ctx.SlotId)
		return
	}

	docs, _ = handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)

		if !s.searchEnable(raw, ctx, tpl) {
			return false
		}

		// 大图不要tym
		if raw.Channel == "tym" {
			return false
		}

		// 必须要有素材
		if !raw.HasMatchedCreative(ctx) {
			return false
		}

		// 不要glp的默认图片
		if raw.Channel == "glp" || raw.Channel == "nglp" {
			imgs := raw.Creatives["ALL"]
			for i := 0; i < len(imgs); i++ {
				if strings.Contains(imgs[i].Url, "default/featured_image") {
					return false
				}
			}
		}

		return true
	})

	if len(docs) == 0 {
		ctx.Phase = "JstagH5RetrievalNoAds"
	} else {
		ctx.Phase = "JstagH5RetrievalHit"
	}

	return
}

func (s *Service) getOnePicAd(ctx *http_context.Context,
	handler *dnf.Handler, conds []dnf.Cond) *ad.NativeAdObj {

	if ctx.ImgW*ctx.ImgH == 0 {
		s.l.Println("[JstagH5] missing imgw or imgh, use 950x500 format")
		ctx.ImgW = 950
		ctx.ImgH = 500
	}

	docs := s.getPicDocs(ctx, handler, conds)
	ndocs := len(docs)
	if ndocs == 0 {
		ctx.Phase = "JstagH5AdsNoPics"
		s.l.Println("[jstag_h5] ads no pics, SlotId: ", ctx.SlotId, " imgw: ", ctx.ImgW, " imgh: ", ctx.ImgH)
		return nil
	}

	idx := rand.Intn(ndocs)
	rawAdInter, _ := handler.DocId2Attr(docs[idx])
	raw := rawAdInter.(*raw_ad.RawAdObj)

	if raw == nil {
		ctx.Phase = "JstagH5AdsZero"
		return nil
	}

	adv := raw.ToNativeAd(ctx, 0)
	if adv == nil {
		ctx.Phase = "JstagH5ToAdFail"
	}
	return adv
}

func (s Service) getIntegralWallAds(ctx *http_context.Context,
	handler *dnf.Handler, conds []dnf.Cond, adnum int) []*ad.NativeAdObj {

	docs := s.getIntegralWallDocs(ctx, handler, conds)
	util.Shuffle(docs)

	ndocs := len(docs)
	if ndocs == 0 {
		ctx.Phase = "JstagH5AdsNoPics"
		s.l.Println("[jstag_h5] ads no pics, slotid: ", ctx.SlotId)
		return nil
	}

	listCap := ndocs
	if listCap > adnum {
		listCap = adnum
	}

	advList := make([]*ad.NativeAdObj, 0, listCap)
	appWallMap := make(map[string]bool, ndocs)
	for i := 0; i < ndocs && i < adnum; i++ {
		rawAdInter, _ := handler.DocId2Attr(docs[i])
		raw := rawAdInter.(*raw_ad.RawAdObj)
		if appWallMap[raw.AppDownload.PkgName] {
			continue
		}
		appWallMap[raw.AppDownload.PkgName] = true
		if adv := raw.ToNativeAd(ctx, i); adv != nil {
			advList = append(advList, adv)
		}
	}
	return advList

}

func (s *Service) jstagH5Handler(w http.ResponseWriter, r *http.Request) {
	s.stat.IncrTot()
	s.stat.GetJstagH5Stat().IncrTot()

	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		s.l.Println("jstagH5Handler new context err: ", err)
		resp := NewRtvResp("url parameters error", 42, nil)
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[jstag_h5] context err resp write: ", n, ", error:", err)
		}
		s.stat.GetJstagH5Stat().IncrCtxErr()
		return
	}

	ctx.Estimate("BeginJstagH5Ad: " + ctx.Platform)
	defer func() {
		ctx.Estimate("End")
		ctx.LogEstimate()
	}()

	handler := dnf.GetHandler()
	if handler == nil {
		ctx.Phase = "JstagH5DnfLoading"
		resp := NewRtvResp("dnf_handler nil", 2, ctx)
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[jstag_h5] dnf nil resp write: ", n, ", error:", err)
		}
		s.stat.GetJstagH5Stat().IncrDnfNil()
		return
	}

	conds := s.makeRetrievalConditions(ctx)
	ctx.Estimate("GenRetrievalConditions: " + dnf.ConditionsToString(conds))

	s.SetCtxTks(ctx)

	jstagH5Ads := make([]*ad.NativeAdObj, 0, ctx.AdNum)
	picAd := s.getOnePicAd(ctx, handler, conds)
	if picAd == nil {
		resp := NewRtvResp("No pic ads", 1, ctx)
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[JstagH5] no pic ads resp write: ", n, ", error:", err)
		}
		ctx.Phase = "JstagH5NoPicAds"
		s.stat.GetJstagH5Stat().IncrRetrievalFilted()
		return
	}
	jstagH5Ads = append(jstagH5Ads, picAd)

	if ctx.IntegralWall {
		integralWallAds := s.getIntegralWallAds(ctx, handler, conds, ctx.AdNum-1)
		if len(integralWallAds) == 0 {
			s.l.Println("[JstagH5] no integralWallAds ads, SlotId: ", ctx.SlotId)
			ctx.Phase = "JstagH5NoAppWallAds"
			s.stat.GetJstagH5Stat().IncrRetrievalFilted()
		}
		jstagH5Ads = append(jstagH5Ads, integralWallAds...)
	} else {
		fuyuAds := s.getFuyuAds(ctx, handler, conds, ctx.AdNum-1)
		if len(fuyuAds) == 0 {
			s.l.Println("[JstagH5] no fuyu ads, SlotId: ", ctx.SlotId)
			ctx.Phase = "JstagH5NoFuyuAds"
			s.stat.GetJstagH5Stat().IncrRetrievalFilted()
		}
		jstagH5Ads = append(jstagH5Ads, fuyuAds...)
	}

	ctx.Phase = "JstagH5OK"
	resp := NewRtvResp("ok", 0, ctx)

	for _, ad := range jstagH5Ads {
		resp.AdList = append(resp.AdList, ad)
		resp.offers = append(resp.offers, ad.UniqId)
	}

	if len(resp.AdList) > 0 {
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[jstag_h5] resp write: ", n, ", error:", err)
		}
		ctx.Estimate("WriteTo")
		s.stat.GetJstagH5Stat().IncrImp()
		return
	}

	ctx.Phase = "JstagH5Zero"
	resp = NewRtvResp("no jstag_h5 ad suggestted", 6, ctx)
	if n, err := resp.WriteTo(w); err != nil {
		s.l.Println("[jstag_h5] no wugan ad resp write: ", n, ", error:", err)
	}
	s.stat.GetJstagH5Stat().IncrWuganFilted()
	return
}
