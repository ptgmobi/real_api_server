package retrieval

import (
	"math/rand"
	"net/http"

	dnf "github.com/brg-liuwei/godnf"

	"http_context"
	"rank"
	"raw_ad"
	"ssp"
)

func (s *Service) interstitialHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		s.l.Println("interstitialHandler new context err: ", err)
		if n, err := NewRtvResp(err.Error(), 42, nil).WriteTo(w); err != nil {
			s.l.Println("[interstitial] context resp write: ", n, ", error: ", err)
		}
		return
	}

	handler := dnf.GetHandler()
	if handler == nil {
		if n, err := NewRtvResp("dnf handler nil", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[interstitial] dnf nil resp write: ", n, ", error: ", err)
		}
		return
	}

	tpl := s.getTpl(ctx.SlotId)
	if tpl == nil {
		if n, err := NewRtvResp("no match tpl", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[interstitial] tpl resp write: ", n, ", error: ", err)
		}
		return
	}

	if tpl.SlotSwitch == 2 {
		if n, err := NewRtvResp("slot closed", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[interstitial] slot closed resp write: ", n, ", error: ", err)
		}
		return
	}

	// 插屏
	if !tpl.MatchSspFormat(9) && !tpl.MatchSspFormat(1) {
		if n, err := NewRtvResp("not interstitial slot", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[interstitial] not interstitial slot resp write: ", n, ", error: ", err)
		}
		return
	}
	ctx.AdType = "15"

	if rand.Float64() > tpl.ImpressionRate {
		if n, err := NewRtvResp("no match ad", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[interstitial] slot improssion rate control resp write: ", n, ", error:", err)
		}
		return
	}

	if err := ctx.GetFreq(); err != nil {
		s.l.Println("[interstitial] get frequency error: ", err)
	}

	// 单用户频控
	if !ctx.UserFreqCap(10) {
		if n, err := NewRtvResp("no ads, user cap", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[interstitial] reach user cap resp write: ", n, ", error: ", err)
		}
		return
	}

	/*
	 * 横屏大图和视频模板，图片比例 1.9:1
	 * 竖屏大图，图片比例 9:16
	 */
	ctx.ImgW, ctx.ImgH = 19, 10
	// 视频只出横屏
	ctx.VideoScreenType = 1
	horizontal := true
	if ctx.ScreenW < ctx.ScreenH { // 竖屏
		horizontal = false
	}

	tpls := make([]*ssp.TemplateObj, 0, 2)
	for i := 0; i != len(tpl.Templates); i++ {
		t := &tpl.Templates[i]
		if horizontal && (t.StyleType == 9 || t.StyleType == 11) {
			tpls = append(tpls, t)
		} else if !horizontal && (t.StyleType == 10 || t.StyleType == 12) {
			tpls = append(tpls, t)
		}
	}
	if len(tpls) == 0 {
		if n, err := NewRtvResp("no tpl selected", 6, ctx).WriteTo(w); err != nil {
			s.l.Println("[interstitial] no tpl selected resp write: ", n, ", error: ", err)
		}
		return
	}

	// 随机获取模板
	ctx.Template = tpls[rand.Intn(len(tpls))]
	ctx.H5tpl = []byte(ctx.Template.H5)
	// 竖屏大图图片 9:16
	if ctx.Template.StyleType == 10 {
		ctx.ImgW, ctx.ImgH = 9, 16
	}

	conds := s.makeRetrievalConditions(ctx)
	docs, _ := handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)

        if raw.IsT {
            return false
        }

		if _, ok := s.adSearch(raw, ctx, tpl); !ok {
			return false
		}

		// slot black cat
		if tpl.AppCategoryInBlackList(raw.AppCategory) {
			return false
		}

		// 目前使用视频
		if !raw.IsHitChannelType(raw_ad.CT_VIDEO) {
			return false
		}

		// 插屏素材
		if !raw.HasInterstitialCreative(ctx) {
			return false
		}

		return true
	})

	ndocs := len(docs)
	if ndocs == 0 {
		if n, err := NewRtvResp("no ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[interstitial] no ads, resp write: ", n, ", error: ", err)
		}
		return
	}

	docCnt := 0
	listCap := ndocs
	rawList := make([]*raw_ad.RawAdObj, 0, listCap)

	for i := 0; i != ndocs && docCnt < listCap; i++ {
		docCnt++
		rawAdInter, _ := handler.DocId2Attr(docs[i])
		rawList = append(rawList, rawAdInter.(*raw_ad.RawAdObj))
	}

	raws := rank.RandCopy(rawList, ctx, 1)
	if len(raws) == 0 {
		if n, err := NewRtvResp("No rank ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[interstitial] no rank ads, resp write: ", n, ", error: ", err)
		}
		return
	}

	s.SetCtxTks(ctx)
	resp := NewRtvResp("ok", 0, ctx)
	for _, raw := range raws {
		if adv := raw.ToInterstitialAd(ctx); adv != nil {
			resp.AdList = append(resp.AdList, adv)
			resp.offers = append(resp.offers, raw.UniqId)
		}
	}

	if len(resp.AdList) > 0 {
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[interstitial] resp write: ", n, ", error: ", err)
		}
		return
	}

	if n, err := NewRtvResp("no ad suggesteed", 6, ctx).WriteTo(w); err != nil {
		s.l.Println("[interstitial] no interstitial ad resp write: ", n, ", error: ", err)
	}
	return
}
