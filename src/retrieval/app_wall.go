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
)

func shuffleAndCopyRaws(rawList []*raw_ad.RawAdObj, ctx *http_context.Context,
	size int, appWallCat string) []*ad.NativeAdObj {
	advs := make([]*ad.NativeAdObj, 0, size)
	pos := rand.Perm(len(rawList))
	for _, i := range pos {
		raw := *rawList[i]
		raw.AppWallCat = appWallCat
		if adv := raw.ToNativeAd(ctx, 1); adv != nil {
			advs = append(advs, adv)
		}
		if len(advs) >= size {
			break
		}
	}
	return advs
}

func (s *Service) appWallHandler(w http.ResponseWriter, r *http.Request) {
	s.stat.GetNatStat().IncrTot()
	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		s.l.Println("appWallHandler new context err: ", err)
		if n, err := NewRtvResp("url params error", 42, nil).WriteTo(w); err != nil {
			s.l.Println("[AppWall] context err resp write: ", n, ", error: ", err)
		}
		s.stat.GetNatStat().IncrCtxErr()
		return
	}

	ctx.Estimate("BeginAppWallAd: " + ctx.Platform)
	defer func() {
		ctx.Estimate("End")
		ctx.LogEstimate()
	}()

	handler := dnf.GetHandler()
	if handler == nil {
		ctx.Phase = "AppWallDnfLoading"
		if n, err := NewRtvResp("dnf handler nil", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[AppWall] dnf nil resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrDnfNil()
		return
	}

	tpl := s.getTpl(ctx.SlotId)
	ctx.Estimate("GetTpl: " + ctx.SlotId)
	if tpl == nil {
		ctx.Phase = "AppWallTplNil"
		if n, err := NewRtvResp("no match tpl", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[AppWall] no match tpl resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrTplNoMatch()
		return
	}

	if tpl.SlotSwitch == 2 {
		ctx.Phase = "AppWallTplClosed"
		if n, err := NewRtvResp("slot closed", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[AppWall] slot closed resp write: ", n, ", error: ", err)
		}
		s.stat.GetNatStat().IncrTplNoMatch()
		return
	}

	conds := s.makeRetrievalConditions(ctx)
	ctx.Estimate("GenRetrievalConditions: " + dnf.ConditionsToString(conds))

	docs, _ := handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)

        if raw.IsT {
            return false
        }

		// 通用广告搜索
		if _, ok := s.adSearch(raw, ctx, tpl); !ok {
			return false
		}

		icons := raw.Icons["ALL"]
		for _, icon := range icons {
			if s.IsDefIcon(raw.Platform, icon.Url) {
				return false
			}
		}

		// 过滤所有没有final_url的广告
		if raw.FinalUrl == "" {
			return false
		}

		if tpl.AppCategoryInBlackList(raw.AppCategory) {
			return false
		}

		if len(raw.AppDownload.Title) == 0 || len(raw.AppDownload.Desc) == 0 {
			return false
		}

		// 曝光频次
		if inCap, _ := ctx.InFreqCap(raw.AppDownload.PkgName, 1); !inCap {
			return false
		}

		return true
	})

	ctx.Estimate("SearchNDocs: " + strconv.Itoa(len(docs)) + ", uid: " + ctx.UserId)
	ctx.Phase = "AppWallRetrieval"

	ndocs := len(docs)

	if ndocs == 0 {
		if n, err := NewRtvResp("No ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[AppWall] no ads resp write: ", n, ", error: ", err)
		}
		s.stat.GetNatStat().IncrRetrievalFilted()
		return
	}

	appWallMap := make(map[string]bool, ndocs)

	topList := make([]*raw_ad.RawAdObj, 0, 32)
	recList := make([]*raw_ad.RawAdObj, 0, 32)
	gameList := make([]*raw_ad.RawAdObj, 0, 32)
	toolList := make([]*raw_ad.RawAdObj, 0, 32)

	for i := 0; i != ndocs; i++ {
		rawAdInter, _ := handler.DocId2Attr(docs[i])
		raw := rawAdInter.(*raw_ad.RawAdObj)
		if appWallMap[raw.AppDownload.PkgName] {
			continue
		}
		appWallMap[raw.AppDownload.PkgName] = true

		if raw.HasMatchedCreative(ctx) {
			topList = append(topList, raw)
			recList = append(recList, raw)
		}

		cat := "all"
		if len(raw.AppCategory) > 0 {
			cat = raw.AppCategory[0]
		}

		if cat == "game" {
			gameList = append(gameList, raw)
		}

		if cat == "tool" {
			toolList = append(toolList, raw)
		}
	}

	if len(topList) > rankUseGzipCap {
		ctx.RankUseGzip = true
	}

	// 大图进行排序
	topRaws := rank.Select(topList, ctx)

	ctx.Estimate("Rank: " + strconv.Itoa(len(topRaws)))

	if len(topRaws) == 0 {
		ctx.Phase = "AppWallRankZero"
		if n, err := NewRtvResp("No ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[AppWall] rank no ads resp write: ", n, ", error: ", err)
		}
		s.stat.GetNatStat().IncrRankFilted()
		return
	}
	s.SetCtxTks(ctx)

	ctx.Phase = "AppWallOk"
	resp := NewRtvResp("ok", 0, ctx)

	// 应用墙，1个带大图的，12个推荐，12个游戏，12个工具
	advs := make([]*ad.NativeAdObj, 0, 64)
	for i, raw := range topRaws {
		raw.AppWallCat = "top"
		if adv := raw.ToNativeAd(ctx, i); adv != nil {
			advs = append(advs, adv)
			break
		}
	}

	advs = append(advs, shuffleAndCopyRaws(recList, ctx, 12, "feature")...)
	advs = append(advs, shuffleAndCopyRaws(gameList, ctx, 12, "game")...)
	advs = append(advs, shuffleAndCopyRaws(topList, ctx, 12, "tool")...)

	for i := 0; i != len(advs); i++ {
		resp.AdList = append(resp.AdList, advs[i])
		resp.offers = append(resp.offers, advs[i].UniqId)
	}

	ctx.Estimate("ToAppWallAds: " + strconv.Itoa(len(resp.AdList)))

	if len(resp.AdList) > 0 {
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[AppWall] resp write: ", n, ", error:", err)
		}
		ctx.Estimate("WriteTo")
		s.stat.GetNatStat().IncrImp()
		return
	}

	ctx.Phase = "AppWallZero"
	if n, err := NewRtvResp("no app wall ad suggestted", 6, ctx).WriteTo(w); err != nil {
		s.l.Println("[AppWall] no wugan ad resp write: ", n, ", error:", err)
	}
	s.stat.GetNatStat().IncrWuganFilted()
	return

}
