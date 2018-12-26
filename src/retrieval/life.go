package retrieval

import (
	"math/rand"
	"net/http"
	"strconv"

	dnf "github.com/brg-liuwei/godnf"

	"http_context"
	"rank"
	"raw_ad"
)

func (s *Service) lifeHandler(w http.ResponseWriter, r *http.Request) {
	s.stat.GetLifeStat().IncrTot()
	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		s.l.Println("lifeHandler new context err: ", err)
		if n, err := NewRtvResp("url parameters error", 2, nil).WriteTo(w); err != nil {
			s.l.Println("[life] context err resp write: ", n, ", error:", err)
		}
		s.stat.GetLifeStat().IncrCtxErr()
		return
	}
	s.SetCtxTks(ctx)

	ctx.Estimate("BeginLifeAd: " + ctx.Platform)
	defer func() {
		ctx.Estimate("End")
		ctx.LogEstimate()
	}()

	handler := dnf.GetHandler()
	if handler == nil {
		ctx.Phase = "LifeDnfLoading"
		if n, err := NewRtvResp("dnf_handler nil", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[native] dnf nil resp write: ", n, ", error:", err)
		}
		s.stat.GetLifeStat().IncrDnfNil()
		return
	}

	tpl := s.getTpl(ctx.SlotId)
	ctx.Estimate("GetTpl: " + ctx.SlotId)
	if tpl == nil {
		ctx.Phase = "LifeTplNil"
		if n, err := NewRtvResp("no match tpl", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[life] no match tpl resp write: ", n, ", error:", err)
		}
		s.stat.GetLifeStat().IncrTplNoMatch()
		return
	}

	if tpl.SlotSwitch == 2 {
		// slot switch 2: closed
		ctx.Phase = "LifeTplClosed"
		if n, err := NewRtvResp("slot closed", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[life] slot closed resp write: ", n, ", error:", err)
		}
		s.stat.GetLifeStat().IncrTplNoMatch()
		return
	}

	if rand.Float64() > tpl.ImpressionRate {
		ctx.Phase = "LifeImpressionCtrl"
		if n, err := NewRtvResp("no match ad", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[life] slot improssion rate control resp write: ", n, ", error:", err)
		}
		s.stat.GetLifeStat().IncrImpRateFilted()
		return
	}

	conds := s.makeRetrievalConditions(ctx)

	ctx.Estimate("GenRetrievalConditions: " + dnf.ConditionsToString(conds))

	if err := ctx.GetFreq(); err != nil {
		s.l.Println("[life] GetFreq error: ", err)
	}

	docs, _ := handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)

		if len(ctx.Channel) != 0 {
			if raw.Channel != ctx.Channel {
				return false
			}
		}

		// black channel
		if tpl.AdNetworkBlackMap != nil && tpl.AdNetworkBlackMap[raw.Channel] {
			return false
		}

		// white channel
		if tpl.AdNetworkWhiteMap != nil && !tpl.AdNetworkWhiteMap[raw.Channel] {
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

		if len(ctx.Carrier) > 0 {
			if !raw.IsHitCarrier(ctx.Carrier) {
				return false
			}
		}

		if tpl.AppCategoryInBlackList(raw.AppCategory) {
			return false
		}

		// 渠道流量比控制
		if rand.Float64() > raw.TrafficRate {
			return false
		}

		if len(raw.AppDownload.Title) == 0 || len(raw.AppDownload.Desc) == 0 {
			return false
		}

		if tpl.PkgBlackMap != nil && tpl.PkgBlackMap[raw.AppDownload.PkgName] {
			return false
		}

		// 曝光频次
		if inCap, _ := ctx.InFreqCap(raw.AppDownload.PkgName, 1); !inCap {
			return false
		}

		// 搜索接口
		if !s.matchKeyWords(raw, ctx) {
			return false
		}

		// 安卓布隆过滤器
		if ctx.Platform == "Android" && ctx.CtBF != nil && ctx.CtBF.TestIndex(raw.CtBloomIndex) {
			// installed package
			return false
		}

		return true
	})

	ctx.Estimate("SearchNDocs: " + strconv.Itoa(len(docs)) + ", uid: " + ctx.UserId)
	ctx.Phase = "LifeRetrieval"

	ndocs := len(docs)

	if ndocs == 0 {
		if n, err := NewRtvResp("No ads", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[life] no ads resp write: ", n, ", error:", err)
		}
		s.stat.GetLifeStat().IncrRetrievalFilted()
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

	ctx.Estimate("RawListPointerCopy-" + strconv.Itoa(ndocs) +
		", erase dup: " + strconv.Itoa(len(rawList)))

	var raws []*raw_ad.RawAdObj = nil

	raws = rank.SelectWg(rawList, ctx)

	ctx.Estimate("Rank: " + strconv.Itoa(len(raws)))

	if len(raws) == 0 {
		ctx.Phase = "LifeRankZero"
		if n, err := NewRtvResp("No ads", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[life] rank no ads resp write: ", n, ", error:", err)
		}
		s.stat.GetLifeStat().IncrRankFilted()
		return
	}

	ctx.Phase = "LifeOK"
	resp := NewRtvResp("ok", 1, ctx)

	for _, raw := range raws {
		if adv := raw.ToLifeAd(ctx); adv != nil {
			ctx.AdPkgName = append(ctx.AdPkgName, raw.AppDownload.PkgName)

			resp.AdList = append(resp.AdList, adv)
			resp.offers = append(resp.offers, raw.UniqId)
		}
	}

	ctx.Estimate("ToLifeAds: " + strconv.Itoa(len(resp.AdList)))

	if err := ctx.IncrPreClickFreq(); err != nil {
		s.l.Println("[life] IncrPreClickFreq error: ", err)
	}

	if len(resp.AdList) > 0 {
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[life] resp write: ", n, ", error:", err)
		}
		ctx.Estimate("WriteTo")
		s.stat.GetLifeStat().IncrImp()
		return
	}

	ctx.Phase = "LifeZero"
	if n, err := NewRtvResp("no life ad suggestted", 2, ctx).WriteTo(w); err != nil {
		s.l.Println("[life] no life ad resp write: ", n, ", error:", err)
	}
	return
}
