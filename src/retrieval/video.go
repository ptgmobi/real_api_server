package retrieval

import (
	"math/rand"
	"net/http"
	"strconv"

	dnf "github.com/brg-liuwei/godnf"

	"http_context"
	"rank"
	"raw_ad"
	"util"
	"vast_module"
)

func (s *Service) videoHandler(w http.ResponseWriter, r *http.Request) {
	s.stat.GetNatStat().IncrTot()
	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		s.l.Println("videoHandler new context err: ", err)
		if n, err := NewRtvResp("url parameters error", 42, ctx).WriteTo(w); err != nil {
			s.l.Println("[video] context err resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrCtxErr()
		return
	}

	ctx.Estimate("BeginVideoAd: " + ctx.Platform)
	defer func() {
		ctx.Estimate("End")
		ctx.LogEstimate()
	}()

	handler := dnf.GetHandler()
	if handler == nil {
		if n, err := NewRtvResp("dnf_handler nil", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[video] dnf nil resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrDnfNil()
		return
	}

	tpl := s.getVideoTplAndUpdateSlot(ctx)
	ctx.Estimate("GetTpl:" + ctx.SlotId)
	if tpl == nil {
		if n, err := NewRtvResp("no active video slot matched", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[video] no match tpl resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrTplNoMatch()
		return
	}

	if tpl.SlotSwitch == 2 {
		if n, err := NewRtvResp("slot closed", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[video] slot closed resp write: ", n, ", error: ", err)
		}
		return
	}

	if rand.Float64() > tpl.ImpressionRate {
		if n, err := NewRtvResp("no match ad", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[video] slot improssion rate control resp write: ", n, ", error:", err)
		}
		return
	}

	// Video Screen Type 1: 横屏 2: 竖屏
	if tpl.VideoScreenType == 1 {
		ctx.VideoScreenType = 1
	} else if tpl.VideoScreenType == 2 {
		ctx.VideoScreenType = 2
	} else {
		ctx.VideoScreenType = 0
	}

	// SlotInfo.Format: 6 video; 7 rewarded video
	if tpl.Format == 6 {
		ctx.AdType = "6"
	} else if tpl.Format == 7 {
		ctx.AdType = "7"
	} else {
		// use default adtype(7)
		s.l.Println("unexpected video format: ", tpl.Format, ", slot id: ", ctx.SlotId)
	}

	ctx.PkgName = ctx.PkgName + "(vdo" + ctx.AdType + ")"

	if ctx.ImgH*ctx.ImgW == 0 {
		if n, err := NewRtvResp("missing required parameter imgw or imgh",
			6, ctx).WriteTo(w); err != nil {
			s.l.Println("[video] missing imgw or imgh resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrImgSizeErr()
		return
	}

	if err := ctx.GetFreq(); err != nil {
		s.l.Println("[video] GetFreq error: ", err)
	}

	if ctx.IsRewardedVideo() {
		if !ctx.VideoInRequestCap() || !ctx.VideoInCompleteCap(tpl.VideoFreqCap) || !tpl.SlotImpInCap(tpl.MaxComplete) {
			if n, err := NewRtvResp("user reach freq cap of slot or slot reach complete cap", 1, ctx).WriteTo(w); err != nil {
				s.l.Println("[video] user reach freq cap of slot or slot reach complete cap resp write: ", n, ", error:", err)
			}
			return
		}
	} else {
		if !tpl.SlotImpInCap(tpl.MaxImpression) {
			if n, err := NewRtvResp("slot reach impression cap", 1, ctx).WriteTo(w); err != nil {
				s.l.Println("[video] slot reach impression cap resp write: ", n, ", error:", err)
			}
			return
		}
	}

	conds := s.makeRetrievalConditions(ctx)
	ctx.Estimate("GenRetrievalConditions:" + dnf.ConditionsToString(conds))

	isVastInOfferServer := ctx.IsVastInOfferServer()

	docs, _ := handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)

		// 通用广告搜索
		if _, ok := s.adSearch(raw, ctx, tpl); !ok {
			return false
		}

		if ctx.AdType == "7" { // 激励视频只出特定渠道
			if !raw.IsHitChannelType(raw_ad.CT_REWARDED_VIDEO) {
				return false
			}
		} else {
			// 曝光频次
			if inCap, _ := ctx.InFreqCap(raw.AppDownload.PkgName, 5); !inCap {
				ctx.Debug(raw.UniqId, "imp frequency")
				return false
			}
		}

		// 整合vast模块到offer_server
		if !isVastInOfferServer {
			if raw.VastUrl == "" {
				return false
			}
		}
		// 临时需求等sdk解决302
		if raw.Channel == "vmvt" {
			if (ctx.Platform == "Android" && ctx.SdkVersion <= "2.0.7") || (ctx.Platform == "iOS" && ctx.SdkVersion <= "2.1.2") {
				return false
			}
		}

		if !raw.HasMatchedNativeVideo(ctx) {
			return false
		}

		if ctx.IsIosInstalledPkg(raw.AppDownload.PkgName) {
			return false
		}

		return true
	})

	ctx.Estimate("SearchNDocs: " + strconv.Itoa(len(docs)))

	util.Shuffle(docs)
	ndocs := len(docs)

	if ndocs == 0 {
		if n, err := NewRtvResp("No ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[video] no ads resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrRetrievalFilted()
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
		raw := rawAdInter.(*raw_ad.RawAdObj)

		rawList = append(rawList, raw)
	}
	ctx.Estimate("RawListPointerCopy-" + strconv.Itoa(ndocs))

	raws := rank.Select(rawList, ctx)

	ctx.Estimate("Rank")

	if len(raws) == 0 {
		if n, err := NewRtvResp("No ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[video] rank no ads resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrRankFilted()
		return
	}

	s.SetVideoCtxTks(ctx)

	resp := NewRtvResp("ok", 0, ctx)

	for _, raw := range raws {
		if adv := raw.ToVideoAd(ctx); adv != nil {
			if isVastInOfferServer {
				vast_module.FillVast(adv)
			}
			ctx.AdPkgName = append(ctx.AdPkgName, raw.AppDownload.PkgName)
			resp.AdList = append(resp.AdList, adv)
			resp.offers = append(resp.offers, raw.UniqId)
		}
	}
	ctx.Estimate("ToVideoAds")

	if len(resp.AdList) > 0 {
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[video] resp write: ", n, ", error:", err)
		}
		ctx.Estimate("WriteTo")
		s.stat.GetNatStat().IncrImp()
		return
	}

	if n, err := NewRtvResp("no video ad suggestted", 6, ctx).WriteTo(w); err != nil {
		s.l.Println("[video] no wugan ad resp write: ", n, ", error:", err)
	}
	s.stat.GetNatStat().IncrWuganFilted()
	return
}

func (s *Service) SetVideoCtxTks(ctx *http_context.Context) {
	if ctx.Platform == "iOS" {
		ctx.PreImpTks = s.conf.IosVideoPreImpTks
		ctx.PreClkTks = s.conf.IosVideoPreClkTks
		ctx.PostImpTks = s.conf.IosVideoPostImpTks
		ctx.PostClkTks = s.conf.IosVideoPostClkTks
	} else if ctx.Platform == "Android" {
		ctx.PreImpTks = s.conf.AndroidVideoPreImpTks
		ctx.PreClkTks = s.conf.AndroidVideoPreClkTks
		ctx.PostImpTks = s.conf.AndroidVideoPostImpTks
		ctx.PostClkTks = s.conf.AndroidVideoPostClkTks
	} else {
		// untouch code here
		panic("unexpected platform: " + ctx.Platform)
	}
}
