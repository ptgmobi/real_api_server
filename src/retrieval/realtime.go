package retrieval

import (
	"math/rand"
	"net/http"
	"strconv"

	dnf "github.com/brg-liuwei/godnf"

	"ad"
	"http_context"
	"rank"
	"raw_ad"
	"util"
)

type rltAdObj struct {
	AdId   string   `json:"adid"`
	Icon   string   `json:"icon"`
	Image  string   `json:"image"`
	Title  string   `json:"title"`
	Desc   string   `json:"desc"`
	Star   float32  `json:"star"`
	ClkUrl string   `json:"clk_url"`
	ImpTks []string `json:"imp_tks"`
	ClkTks []string `json:"clk_tks"`
}

func ToRltAdObj(adv *ad.NativeAdObj) *rltAdObj {
	return &rltAdObj{
		AdId:   adv.Id,
		Icon:   adv.Core.Icon,
		Image:  adv.Core.Image,
		Title:  adv.Core.Title,
		Desc:   adv.Core.Desc,
		Star:   adv.Core.Star,
		ClkUrl: adv.ClkUrl,
		ImpTks: adv.ImpTkUrl,
		ClkTks: adv.ClkTkUrl,
	}
}

func (s *Service) realtimeHandler(w http.ResponseWriter, r *http.Request) {
	s.stat.GetRltStat().IncrTot()
	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		s.l.Println("realtimeHandler new context err: ", err)
		if n, err := NewRtvResp("url parameters error", 42, nil).WriteTo(w); err != nil {
			s.l.Println("[realtime] context err resp write: ", n, ", error:", err)
		}
		s.stat.GetRltStat().IncrCtxErr()
		return
	}

	ctx.DetailReqType = "jstag_rlt"

	ctx.Estimate("BeginRealtimeAd: " + ctx.Platform)
	defer func() {
		ctx.Estimate("End")
		ctx.LogEstimate()
	}()

	handler := dnf.GetHandler()
	if handler == nil {
		ctx.Phase = "RealtimeDnfLoading"
		if n, err := NewRtvResp("dnf_handler nil", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[realtime] dnf nil resp write: ", n, ", error:", err)
		}
		s.stat.GetRltStat().IncrDnfNil()
		return
	}

	tpl := s.getTpl(ctx.SlotId)
	ctx.Estimate("GetTpl: " + ctx.SlotId)
	if tpl == nil {
		ctx.Phase = "RealTimeTplNil"
		if n, err := NewRtvResp("no match tpl", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[realtime] no match tpl resp write: ", n, ", error:", err)
		}
		s.stat.GetRltStat().IncrTplNoMatch()
		return
	}

	if tpl.SlotSwitch == 2 {
		// slot switch 2: closed
		ctx.Phase = "RealTimeTplClosed"
		if n, err := NewRtvResp("slot closed", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[realtime] slot closed resp write: ", n, ", error:", err)
		}
		s.stat.GetRltStat().IncrTplNoMatch()
		return
	}

	// slot层级的曝光控制
	if ctx.AdType == "0" || ctx.AdType == "1" || ctx.AdType == "2" {
		if rand.Float64() > tpl.ImpressionRate {
			ctx.Phase = "RealTimeImpressionCtrl"
			if n, err := NewRtvResp("no match ad", 5, ctx).WriteTo(w); err != nil {
				s.l.Println("[realtime] slot improssion rate control resp write: ", n, ", error:", err)
			}
			s.stat.GetRltStat().IncrImpRateFilted()
			return
		}
	}

	if ctx.ImgH*ctx.ImgW == 0 {
		ctx.ImgH = 500
		ctx.ImgW = 950
		s.l.Println("[realtime] missing imgw or imgh, use 950x500")
	}

	conds := s.makeRetrievalConditions(ctx)

	ctx.Estimate("GenRealtimeConditions: " + dnf.ConditionsToString(conds))

	if err := ctx.GetFreq(); err != nil {
		s.l.Println("[realtime] GetFreq error: ", err)
	}

	docs, _ := handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)

		if raw.IsT {
			return false
		}

		if len(ctx.Channel) != 0 {
			if raw.Channel != ctx.Channel {
				return false
			}
		}

		// 渠道流量比控制
		if rand.Float64() > raw.TrafficRate {
			return false
		}

		if raw.Channel == "tym" {
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

		if tpl.PkgBlackMap != nil && tpl.PkgBlackMap[raw.AppDownload.PkgName] {
			return false
		}

		// 曝光频次
		if inCap, _ := ctx.InFreqCap(raw.AppDownload.PkgName, 10); !inCap {
			return false
		}

		if !raw.HasMatchedCreative(ctx) {
			return false
		}

		// 搜索接口
		if !s.matchKeyWords(raw, ctx) {
			return false
		}

		cat := "all"
		if len(raw.AppCategory) > 0 {
			cat = raw.AppCategory[0]
		}

		// adcat == 1代表请求游戏广告
		if ctx.AdCat == "1" && cat != "game" {
			return false
		}

		// adcat == 2代表请求工具广告
		if ctx.AdCat == "2" && cat != "tool" {
			return false
		}

		return true
	})

	ctx.Estimate("SearchNDocs: " + strconv.Itoa(len(docs)) + ", uid: " + ctx.UserId)
	ctx.Phase = "RealtimeRetrieval"
	ctx.Nrtv = len(docs)

	ndocs := len(docs)

	if ndocs == 0 {
		if n, err := NewRtvResp("No ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[realtime] no ads resp write: ", n, ", error:", err)
		}
		s.stat.GetRltStat().IncrRetrievalFilted()
		return
	}

	util.Shuffle(docs)
	docCnt := 0

	listCap := ndocs
	if listCap > ctx.AdNum {
		listCap = ctx.AdNum
	}
	rawList := make([]*raw_ad.RawAdObj, 0, listCap)

	for i := 0; i != ndocs && docCnt < ctx.AdNum; i++ {
		docCnt++
		rawAdInter, _ := handler.DocId2Attr(docs[i])
		raw := rawAdInter.(*raw_ad.RawAdObj)

		rawList = append(rawList, raw)
	}
	ctx.Estimate("RawListPointerCopy-" + strconv.Itoa(ndocs) +
		", erase dup: " + strconv.Itoa(len(rawList)))

	raws := rank.Select(rawList, ctx)

	ctx.Estimate("Rank: " + strconv.Itoa(len(raws)))

	if len(raws) == 0 {
		ctx.Phase = "RealtimeRankZero"
		if n, err := NewRtvResp("No ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[realtime] rank no ads resp write: ", n, ", error:", err)
		}
		s.stat.GetRltStat().IncrRankFilted()
		return
	}
	s.SetCtxTks(ctx)

	ctx.Phase = "RealtimeOK"
	resp := NewRtvResp("ok", 0, ctx)

	// XXX
	for i, raw := range raws {
		if adv := raw.ToNativeAd(ctx, i); adv != nil {

			ctx.AdPkgName = append(ctx.AdPkgName, raw.AppDownload.PkgName)

			rltAdv := ToRltAdObj(adv)

			resp.AdList = append(resp.AdList, rltAdv)
			resp.offers = append(resp.offers, raw.UniqId)
		}
	}
	ctx.Estimate("ToNativeAds: " + strconv.Itoa(len(resp.AdList)))

	if len(resp.AdList) > 0 {
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[realtime] resp write: ", n, ", error:", err)
		}
		ctx.Estimate("WriteTo")
		s.stat.GetRltStat().IncrImp()
		return
	}

	ctx.Phase = "RealtimeZero"
	if n, err := NewRtvResp("no realtime ad suggestted", 6, ctx).WriteTo(w); err != nil {
		s.l.Println("[realtime] no wugan ad resp write: ", n, ", error:", err)
	}
	s.stat.GetRltStat().IncrWuganFilted()
	return
}
