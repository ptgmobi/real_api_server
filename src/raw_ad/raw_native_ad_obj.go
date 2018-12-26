package raw_ad

import (
	"math/rand"
	"strconv"
	"strings"

	"ad"
	"http_context"
	"util"
)

// 对美图的native广告，使用外开
var extern_landing_slots map[string]bool = map[string]bool{
	"1361":     true,
	"1794":     true,
	"2507":     true,
	"2508":     true,
	"2738":     true,
	"3447":     true,
	"21890988": true,
	"30133956": true,
	"62724276": true,
	"82270342": true,
	"70555291": true,
	"23696844": true,
}

func (raw *RawAdObj) ToNativeAd(ctx *http_context.Context, pos int) *ad.NativeAdObj {
	rc := &ad.NativeAdObj{
		Id:     raw.Id,
		UniqId: raw.UniqId,
		// 加随机数为了解决同一个raw对象多次生成native对象
		ImpId:       ctx.ReqId + raw.Id + strconv.Itoa(rand.Intn(100)) + ctx.SlotId,
		LandingType: raw.LandingType,
		AdExp:       1000,
		ClkTks:      raw.ClkTks,
		FinalUrl:    raw.FinalUrl,
		PkgName:     raw.AppDownload.PkgName,
		Country:     ctx.Country,
		Channel:     raw.Channel,
		AppWallCat:  raw.AppWallCat,
		TaoBaoKe:    raw.TaoBaoKe,
		TaoBaoKeT:   raw.TaoBaoKeT,
		UrlSchema:   raw.UrlSchema,
	}

	if extern_landing_slots[ctx.SlotId] {
		rc.LandingType = ad.EXTERN_LANDING
	}

	rc.SetPreClick(ctx.IsWugan())
	ctx.ImpId = rc.ImpId

	core := &rc.Core
	core.Title = raw.AppDownload.Title
	core.Desc = raw.AppDownload.Desc
	core.Star = raw.AppDownload.Rate
	core.Button = ctx.ButtonText
	core.ChoicesLinkUrl = "https://en.yeahmobi.com/privacy-policy"
	core.OfferType = raw.ContentType // 1: 下载类app，2：非下载类app

	if !ctx.IsWugan() && !ctx.IsPromote() {
		if icon := raw.getMatchedIcon(ctx.Lang); icon != nil {
			core.Icon = ctx.CreativeCdnConv(icon.Url, icon.DomesticCDN)
		} else {
			core.Icon = ""
		}
		if img := raw.getMatchedCreative(ctx.Lang, ctx.ImgW, ctx.ImgH, ctx.ImgRule); img != nil {
			core.Image = ctx.CreativeCdnConv(img.Url, img.DomesticCDN)
			raw.CreativeChosen = img
		}
	}

	if clkUrlGenerator, ok := raw.GetChannelClkUrlGenerator(); ok {
		oldSlotId := ctx.SlotId
		if raw.ReplaceSlotId {
			tmpSlots := util.GetReplacingSlotIds()
			tmpLen := len(tmpSlots)
			ctx.SlotId = tmpSlots[rand.Intn(tmpLen)]
		}
		rc.ClkUrl = clkUrlGenerator(raw, ctx)
		if ctx.SlotId != oldSlotId {
			ctx.SlotId = oldSlotId
		}
	} else {
		ctx.L.Println("ad unknown channel of: ", raw.Channel)
		return nil
	}

	appArgs := make([]string, 0, 12)
	if ctx.AdType == "3" {
		// AdType == "3": 集成无感，区分imp
		appArgs = append(appArgs, "slot=anli_"+ctx.SlotId)
	} else if ctx.AdType == "9" {
		// AdType == "9": fuyu无感
		appArgs = append(appArgs, "slot=anli_"+ctx.SlotId)
	} else if ctx.IsPromote() {
		// promote曝光监测
		appArgs = append(appArgs, "slot=pt_"+ctx.SlotId)
	} else {
		appArgs = append(appArgs, "slot="+ctx.SlotId)
	}

	// XXX: AttachArgs这个变量的使用比较混乱，
	// 以后会考虑对这个变量进行修改
	// 如果你要改这段代码，请确认你知道你是在干什么
	appArgs = append(appArgs, raw.AttachArgs...)

	appArgs = append(appArgs, "city="+ctx.City)
	appArgs = append(appArgs, "region="+ctx.Region)
	appArgs = append(appArgs, "adtype="+ctx.AdType)
	appArgs = append(appArgs, "offer="+rc.Id)
	appArgs = append(appArgs, "imp="+rc.ImpId)
	appArgs = append(appArgs, "channel="+raw.Channel)
	appArgs = append(appArgs, "server_id="+ctx.ServerId)
	appArgs = append(appArgs, "sv="+ctx.SdkVersion)
	appArgs = append(appArgs, "method="+ctx.Method)
	appArgs = append(appArgs, "pck="+strconv.Itoa(rc.PreClick))
	appArgs = append(appArgs, "creative_id="+raw.CreativeId())

	// 应用墙不做频控
	if !ctx.IntegralWall {
		// 增强广告不做频控
		if !ctx.IsPromote() {
			appArgs = append(appArgs, "doimp=1")
		}
		appArgs = append(appArgs, "pkg="+raw.AppDownload.PkgName)
	} else {
		appArgs = append(appArgs, "appwall=1")
	}

	if ctx.AdType == "9" {
		appArgs = append(appArgs, "fyid="+ctx.FuyuUserId)
	}

	appArgsStr := strings.Join(appArgs, "&") + "&" + ctx.PossiableArgs
	if ctx.Fake {
		appArgsStr = appArgsStr + "&f=1"
	}

	// 应用墙除第一个以外都不做曝光监测，否则客户端调用曝光监测负担太大
	// 应用墙游戏和工具类也不做曝光(每个应用墙会有all, game, tool三个请求)
	if !ctx.IntegralWall || (pos == 0 && ctx.AdCat == "0") {
		if raw.IsT {
			rc.ImpTkUrl = util.AppendArgsToMonitors(ctx.PostImpTksDebug, appArgsStr)
		} else {
			rc.ImpTkUrl = util.AppendArgsToMonitors(ctx.PostImpTks, appArgsStr)
		}

		if ctx.DetailReqType == "jstag_rlt" { // 给jstag的实时api添加jstag请求
			url := "http://resource.catch.gift/impression/v1/" + ctx.SlotId + "/tracking?idfa=" + ctx.Idfa
			rc.ImpTkUrl = append(rc.ImpTkUrl, url)
		}

		thirdImpTk := raw.genThirdPartyImpTk(ctx)
		if len(thirdImpTk) > 0 {
			rc.ImpTkUrl = append(rc.ImpTkUrl, thirdImpTk)
		}
	}

	// 无感不发click monitor
	if ctx.AdType != "3" && ctx.AdType != "4" && ctx.AdType != "9" {
		if raw.IsT {
			rc.ClkTkUrl = util.AppendArgsToMonitors(ctx.PostClkTksDebug, appArgsStr)
		} else {
			rc.ClkTkUrl = util.AppendArgsToMonitors(ctx.PostClkTks, appArgsStr)
		}
	}

	// 除去这两个渠道，第三方曝光监测已经在上面的thirdImpTk生成了
	if raw.Channel == "xx" || raw.Channel == "xxj" {
		for _, url := range raw.ThirdPartyImpTks {
			rc.ImpTkUrl = append(rc.ImpTkUrl, offlineReplaceMacro(url, raw, ctx))
		}
		for _, url := range raw.ThirdPartyClkTks {
			rc.ClkTkUrl = append(rc.ClkTkUrl, offlineReplaceMacro(url, raw, ctx))
		}
	}

	// XXX 实时API，监测
	if raw.Channel == "huicheng" {
		rc.ImpTkUrl = append(rc.ImpTkUrl, raw.ThirdPartyImpTks...)
		rc.ClkTkUrl = append(rc.ClkTkUrl, raw.ThirdPartyClkTks...)
	}

	return rc
}
