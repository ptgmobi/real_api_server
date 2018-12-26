package retrieval

import (
	"strings"

	dnf "github.com/brg-liuwei/godnf"

	"ad"
	"http_context"
	"raw_ad"
	"util"
)

// 修改ctx中的值
func (s *Service) modifyFuyuCtx(ctx *http_context.Context) bool {
	ctx.Idfa = ""
	ctx.Gaid = ""
	ctx.Aid = ""

	if ctx.AdType != "10" { // 10 means jstag fuyu, DONOT reset adtype
		ctx.AdType = "9" // reset adtype = 9 (fuyu ad)
	}

	switch ctx.Platform {

	case "iOS":
		ctx.UserId = ctx.FuyuUserId // set fuyu user id
		ctx.Idfa = ctx.UserId

	case "Android":
		ss := strings.Split(ctx.FuyuUserId, ",") // eg: ff8bf06babcec8d1,cec40f84-35a8-438b-ab20-a00effcc0c0c
		if len(ss) != 2 {
			ctx.L.Println("split Android fuyu user id error: ", ctx.FuyuUserId)
			return false
		}

		ctx.Aid = ss[0]
		ctx.Gaid = ss[1]
		ctx.UserId = ctx.Aid

	default:
		ctx.L.Println("FATAL: unexpected platform of: ", ctx.Platform)
		return false
	}

	return true
}

func (s *Service) getFuyuDocsWithShuffle(ctx *http_context.Context,
	handler *dnf.Handler, conds []dnf.Cond) (docs []int) {

	if len(ctx.FuyuUserId) == 0 {
		ctx.FuyuPhase = "FuyuNoId"
		return
	}
	if !s.modifyFuyuCtx(ctx) {
		ctx.FuyuPhase = "FuyuModifyCtx"
		return
	}

	tpl := s.getTpl(ctx.SlotId)

	pacing := s.LoadFuyuPacing()
	docs, _ = handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)

		if len(ctx.Channel) != 0 {
			if raw.Channel != ctx.Channel {
				return false
			}
		}

		if ctx.Country == "CN" && ctx.Platform == "iOS" {
			if !util.WuganHitChannel(raw.Channel) {
				return false
			}
		}

		// 优先不投放离线offer
		if raw.Channel == "xx" || raw.Channel == "xxj" {
			return false
		}

		if tpl != nil && tpl.OfferBlackMap != nil {
			if tpl.OfferBlackMap[raw.UniqId] {
				return false
			}
		}

		if ctx.AdType == "3" || ctx.AdType == "4" {
			if !raw.IsWuganEnabled() {
				return false
			}
		}

		if ctx.AdType == "9" {
			if !raw.IsFuyuEnabled() {
				return false
			}
		}

		if ctx.AdType == "10" {
			if !raw.IsJsTagEnabled() {
				return false
			}
		}

		if pacing.OverCap(raw.UniqId, ctx.Country, raw.Pacing, ctx.Now) {
			return false
		}
		// 指定一些offer只在指定的slot上投放
		if !raw.IsHitSlot(ctx.SlotId) {
			return false
		}

		return true
	})

	if len(docs) == 0 {
		ctx.FuyuPhase = "FuyuRetrievalNoAds"
	} else {
		ctx.FuyuPhase = "FuyuRetrievalHit"
	}
	util.Shuffle(docs)
	return
}

func (s *Service) getOfflineFuyuDocs(ctx *http_context.Context, handler *dnf.Handler,
	conds []dnf.Cond, offlineChannels map[string]bool) (docs []int) {

	tpl := s.getTpl(ctx.SlotId)
	pacing := s.LoadFuyuPacing()

	docs, _ = handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)

		if !offlineChannels[raw.Channel] {
			return false
		}

		if ctx.AdType == "3" || ctx.AdType == "4" {
			if !raw.IsWuganEnabled() {
				return false
			}
		}

		if ctx.AdType == "9" {
			if !raw.IsFuyuEnabled() {
				return false
			}
		}

		if ctx.AdType == "10" {
			if !raw.IsJsTagEnabled() {
				return false
			}
		}

		if tpl != nil && tpl.OfferBlackMap != nil {
			if tpl.OfferBlackMap[raw.UniqId] {
				return false
			}
		}
		if pacing.OverCap(raw.UniqId, ctx.Country, raw.Pacing, ctx.Now) {
			return false
		}

		return true
	})
	return
}

func (s *Service) getFuyuAds(ctx *http_context.Context,
	handler *dnf.Handler, conds []dnf.Cond, adNum int) []*ad.NativeAdObj {

	var docs []int

	if adNum < 0 {
		adNum = 0
	}

	if ctx.Country == "CN" && ctx.Platform == "iOS" {
		if ctx.AdType == "10" {
			adNum = 20 // jstag只有70%的流量能够点20个以上
		} else {
			adNum = 100 // SDK流量
		}
	}

	if adNum > 0 {
		docs = s.getFuyuDocsWithShuffle(ctx, handler, conds)
	}

	ndocs := len(docs)

	// 对fuyu package去重
	uniqPkg := make(map[string]bool, ndocs)

	rawList := make([]*raw_ad.RawAdObj, 0, ndocs)
	for i := 0; i != ndocs; i++ {
		rawAdInter, _ := handler.DocId2Attr(docs[i])
		raw := rawAdInter.(*raw_ad.RawAdObj)
		if !uniqPkg[raw.AppDownload.PkgName] {
			rawList = append(rawList, raw)
			uniqPkg[raw.AppDownload.PkgName] = true
		}
	}

	if len(rawList) > adNum {
		// 去重后随机选adnum个fuyu offer来填充
		rawList = rawList[:adNum]
	}

	offlineChannel := make(map[string]bool, 2)
	offlineChannel["xxj"] = true // xxj代表可用js投放的线下offer
	if ctx.AdType != "10" {
		offlineChannel["xx"] = true
	}

	// 线下(xx)offer放到服预前面
	xxDocs := s.getOfflineFuyuDocs(ctx, handler, conds, offlineChannel)
	nXXDocs := len(xxDocs)

	// 对offline package去重
	uniqPkg = make(map[string]bool, nXXDocs)

	xxRawList := make([]*raw_ad.RawAdObj, 0, nXXDocs)
	for i := 0; i != nXXDocs; i++ {
		rawAdInter, _ := handler.DocId2Attr(xxDocs[i])
		raw := rawAdInter.(*raw_ad.RawAdObj)
		if !uniqPkg[raw.AppDownload.PkgName] {
			xxRawList = append(xxRawList, raw)
			uniqPkg[raw.AppDownload.PkgName] = true
		}
	}

	raws := make([]*raw_ad.RawAdObj, 0, len(rawList)+len(xxRawList))

	for i := 0; i != len(xxRawList); i++ {
		rawCopy := *xxRawList[i]
		raws = append(raws, &rawCopy)
	}

	for i := 0; i < len(rawList); i++ {
		rawCopy := *rawList[i]
		raws = append(raws, &rawCopy)
	}

	ctx.Nfy = len(raws)

	if len(raws) == 0 {
		ctx.FuyuPhase = "FuyuRankZero"
		return nil
	}

	ads := make([]*ad.NativeAdObj, 0, len(raws))
	method := ctx.Method // push method
	ctx.Method = "svm"   // flag fuyu
	pacing := s.LoadFuyuPacing()
	for _, raw := range raws {
		if adv := raw.ToNativeAd(ctx, 0); adv != nil {
			adv.Core = ad.NativeAdObjCore{}
			// 增加速度计数
			pacing.Add(raw.UniqId, ctx.Now, 1)
			ads = append(ads, adv)
		}
	}

	ctx.Method = method // pop method

	return ads
}
