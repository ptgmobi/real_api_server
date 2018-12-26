package retrieval

import (
	"math/rand"
	"net/http"
	"strconv"

	dnf "github.com/brg-liuwei/godnf"

	"cpt"
	"http_context"
	"rank"
	"raw_ad"
	"real_api"
	"util"
)

/*
See (https://git.oschina.net/CloudTech/Document/blob/master/adserver_native.md) for detail
*/
func (s *Service) nativeHandler(w http.ResponseWriter, r *http.Request) {
	s.stat.GetNatStat().IncrTot()
	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		s.l.Println("nativeHandler new context err: ", err)
		if n, err := NewRtvResp("url parameters error", 42, nil).WriteTo(w); err != nil {
			s.l.Println("[native] context err resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrCtxErr()
		return
	}
	s.SetCtxTks(ctx)

	ctx.Estimate("BeginNavieAd: " + ctx.Platform)
	defer func() {
		ctx.Estimate("End")
		ctx.LogEstimate()
	}()

	handler := dnf.GetHandler()
	if handler == nil {
		ctx.Phase = "NativeDnfLoading"
		if n, err := NewRtvResp("dnf_handler nil", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[native] dnf nil resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrDnfNil()
		return
	}

	tpl := s.getTpl(ctx.SlotId)
	ctx.Estimate("GetTpl: " + ctx.SlotId)
	if tpl == nil {
		ctx.Phase = "NativeTplNil"
		if n, err := NewRtvResp("no match tpl", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[native] no match tpl resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrTplNoMatch()
		return
	}

	if tpl.SlotSwitch == 2 {
		// slot switch 2: closed
		ctx.Phase = "NativeTplClosed"
		if n, err := NewRtvResp("slot closed", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[native] slot closed resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrTplNoMatch()
		return
	}

	// slot层级的曝光控制
	if ctx.AdType == "0" || ctx.AdType == "1" || ctx.AdType == "2" {
		if rand.Float64() > tpl.ImpressionRate {
			ctx.Phase = "NativeImpressionCtrl"
			if n, err := NewRtvResp("no match ad", 5, ctx).WriteTo(w); err != nil {
				s.l.Println("[native] slot improssion rate control resp write: ", n, ", error:", err)
			}
			s.stat.GetNatStat().IncrImpRateFilted()
			return
		}
	}

	// 美柚(3162,3181)和柚宝宝(f847f2c3)的特殊需求: 出指定大小的图片
	if ctx.SlotId == "3162" || ctx.SlotId == "3181" || ctx.SlotId == "f847f2c3" {
		ctx.ImgRule = 1
		var device string

		if ctx.ScreenH <= 480 { // iphone4s以下
			device = "i4"
		} else if ctx.ScreenH == 568 { // iPhone5,5c,5s,SE: 320*568
			device = "i5"
		} else if ctx.ScreenH == 667 { // iPhone6,6s,7,8: 375*667
			device = "i6"
		} else if ctx.ScreenH == 736 { // iPhone6p,6sp,7p,8p: 414*736
			device = "i6p"
		} else if ctx.ScreenH == 812 { // iphoneX: 375*812
			device = "ix"
		}

		switch device {
		case "i4":
			ctx.ImgW, ctx.ImgH = 640, 760
		// case "i5": // i5 use default
		// case "i6": // i6 use default
		case "i6p":
			ctx.ImgW, ctx.ImgH = 1080, 1555
		case "ix":
			ctx.ImgW, ctx.ImgH = 1080, 1800
		default:
			ctx.ImgW, ctx.ImgH = 750, 1080
		}
	}

	// XXX 临时需求24935627广告位尺寸
	if ctx.SlotId == "24935627" {
		ctx.ImgW = 600
		ctx.ImgH = 316
	}

	if ctx.ImgH*ctx.ImgW == 0 {
		ctx.Phase = "NativeMissingWH"
		if n, err := NewRtvResp("missing required parameter imgw or imgh",
			6, ctx).WriteTo(w); err != nil {
			s.l.Println("[native] missing imgw or imgh resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrImgSizeErr()
		return
	}

	if !tpl.SlotImpInCap(tpl.MaxImpression) {
		if n, err := NewRtvResp("slot reach impression cap", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[native] slot reach impression cap resp write: ", n, ", error:", err)
		}
		return
	}

	conds := s.makeRetrievalConditions(ctx)

	ctx.Estimate("GenRetrievalConditions: " + dnf.ConditionsToString(conds))

	pacing := s.LoadPacing()

	if err := ctx.GetFreq(); err != nil {
		s.l.Println("[native] GetFreq error: ", err)
	}

	docs, _ := handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)

		if raw.IsT && (ctx.AdType == "0" || ctx.AdType == "1" || ctx.AdType == "2") {
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

			if ctx.AdCat == "0" { // 默认的必须有大图，1:游戏，2:工具,不用保证有大图
				if !raw.HasMatchedCreative(ctx) {
					ctx.Debug(raw.UniqId, "integra_wall no big img")
					return false
				}
			}
		}

		if len(ctx.Carrier) > 0 {
			if !raw.IsHitCarrier(ctx.Carrier) {
				ctx.Debug(raw.UniqId, "not hit carrier")
				return false
			}
		}

		if ctx.IsWugan() {
			if ctx.Country == "CN" && ctx.Platform == "iOS" {
				// CN iOS 只点ym、n1ym、n2ym、n3ym、mvt、xx和xxj这些渠道
				if !util.WuganHitChannel(raw.Channel) {
					return false
				}
			}

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
			if len(raw.AppDownload.Title) == 0 || len(raw.AppDownload.Desc) == 0 {
				ctx.Debug(raw.UniqId, "has no title or desc")
				return false
			}

			// 曝光频次
			if inCap, _ := ctx.InFreqCap(raw.AppDownload.PkgName, 1); !inCap {
				ctx.Debug(raw.UniqId, "pkg frequency")
				return false
			}

			// 非应用墙必须要有素材
			if !ctx.IntegralWall && !raw.HasMatchedCreative(ctx) {
				ctx.Debug(raw.UniqId, "has no matched creative")
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
			docs = cptDocs
			useCpt = true
		}
	}

	if !useCpt {
		util.Shuffle(docs)
	}

	ctx.Estimate("SearchNDocs: " + strconv.Itoa(len(docs)) + ", uid: " + ctx.UserId)
	ctx.Phase = "NativeRetrieval"
	ctx.Nrtv = len(docs)

	if len(docs) == 0 && ctx.IsWugan() {
		pacing = s.LoadFuyuPacing() // 更新pacing计数器，免得把服预的点击记到正常投放的pacing中了
		docs = s.getFuyuDocsWithShuffle(ctx, handler, conds)
	}

	ndocs := len(docs)

	if ndocs == 0 {
		if n, err := NewRtvResp("No ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[native] no ads resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrRetrievalFilted()
		if ctx.IsWugan() && ctx.Platform == "iOS" && ctx.Country == "CN" {
			ctx.Phase = "CNiOSWuganEmpty"
		}
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

	var appWallMap map[string]bool
	if ctx.IntegralWall {
		appWallMap = make(map[string]bool, ndocs)
	}

	for i := 0; i != ndocs && docCnt < listCap; i++ {
		docCnt++
		rawAdInter, _ := handler.DocId2Attr(docs[i])
		raw := rawAdInter.(*raw_ad.RawAdObj)

		// app wall: erase duplicate offer
		if ctx.IntegralWall {
			if appWallMap[raw.AppDownload.PkgName] {
				continue
			}
			appWallMap[raw.AppDownload.PkgName] = true
		}

		rawList = append(rawList, raw)
	}
	ctx.Estimate("RawListPointerCopy-" + strconv.Itoa(ndocs) +
		", erase dup: " + strconv.Itoa(len(rawList)))

	if !useCpt {
		if ctx.IntegralWall {
			// 应用墙随机出一个结果集，再排序
			n := ctx.AdNum * 3
			if n > len(rawList) {
				n = len(rawList)
			}
			rawList = rawList[:n]
		}
	}

	var raws []*raw_ad.RawAdObj = nil

	if !useCpt {
		raws = rank.Select(rawList, ctx)
	} else {
		// just deep copy
		raws = make([]*raw_ad.RawAdObj, 0, len(rawList))
		for i := 0; i != len(rawList); i++ {
			raw := *rawList[i]
			raws = append(raws, &raw)
		}
	}

	ctx.Estimate("Rank: " + strconv.Itoa(len(raws)))

	if len(raws) == 0 {
		if raw, err := real_api.Request(ctx); err == nil {
			raws = append(raws, raw)
		} else {
			s.l.Println("[real_api] ", err)
		}
	}

	if len(raws) == 0 {
		ctx.Phase = "NativeRankZero"
		if n, err := NewRtvResp("No ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[native] rank no ads resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrRankFilted()
		return
	}

	ctx.Phase = "NativeOK"
	resp := NewRtvResp("ok", 0, ctx)

	for i, raw := range raws {
		if adv := raw.ToNativeAd(ctx, i); adv != nil {
			if ctx.IsWugan() {
				ctx.FreqInfo.PFreqFields = append(ctx.FreqInfo.PFreqFields, raw.AppDownload.PkgName)
				pacing.Add(raw.UniqId, ctx.Now, 1)
			}

			ctx.AdPkgName = append(ctx.AdPkgName, raw.AppDownload.PkgName)

			resp.AdList = append(resp.AdList, adv)
			resp.offers = append(resp.offers, raw.UniqId)
		}
	}

	ctx.Estimate("ToNativeAds: " + strconv.Itoa(len(resp.AdList)))

	if err := ctx.IncrPreClickFreq(); err != nil {
		s.l.Println("[native] IncrPreClickFreq error: ", err)
	}

	if len(resp.AdList) > 0 {
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[native] resp write: ", n, ", error:", err)
		}
		ctx.Estimate("WriteTo")
		s.stat.GetNatStat().IncrImp()
		return
	}

	ctx.Phase = "NativeZero"
	if n, err := NewRtvResp("no native ad suggestted", 6, ctx).WriteTo(w); err != nil {
		s.l.Println("[native] no wugan ad resp write: ", n, ", error:", err)
	}
	s.stat.GetNatStat().IncrWuganFilted()
	return
}
