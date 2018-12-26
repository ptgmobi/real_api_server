package raw_ad

import (
	"math/rand"
	"strings"

	"ad"
	"http_context"
	"util"
	"vast_module"
)

func (raw *RawAdObj) ToVideoV4Ad(ctx *http_context.Context) *ad.VideoAdObj {
	rc := &ad.VideoAdObj{
		Id:          raw.Id,
		ImpId:       ctx.ReqId + util.ToMd5(raw.Id+ctx.SlotId+ctx.Now.String()+ctx.Ran),
		LandingType: raw.LandingType,
		AdExp:       1000,
		Slot:        ctx.SlotId,
		Channel:     raw.Channel,
		Country:     ctx.Country,
		FinalUrl:    raw.FinalUrl,
		UrlSchema:   raw.UrlSchema,
		Star:        rand.Float32() + 4,
		Button:      ctx.ButtonText,
		RateNum:     raw.AppDownload.Review,
		TaoBaoKe:    raw.TaoBaoKe,
		TaoBaoKeT:   raw.TaoBaoKeT,
		RewardedVideo: ad.RewardedVideoObj{
			/**
			 * https://github.com/cloudadrd/offer_server/issues/493
			 * 将激励视频的click_time和load_time强制置0, 新旧版本兼容
			 */
			LoadTime:  0,
			ClickTime: 0,
			H5Opt:     playableMap[raw.UniqId],
		},
		ImpTks: make([]string, 0),
		ClkTks: make([]string, 0),
	}

	ctx.ImpId = rc.ImpId

	if clkUrlGenerator, ok := raw.GetChannelClkUrlGenerator(); ok {
		serverIdBak := ctx.ServerId // push server id
		if raw.VideoChosen != nil {
			ctx.ServerId = "v." + raw.VideoChosen.Id
		}
		rc.ClkUrl = clkUrlGenerator(raw, ctx)
		ctx.ServerId = serverIdBak // pop server id (为了保持报表不出问题)
	} else {
		ctx.L.Println("ad unknown channel of: ", raw.Channel)
		return nil
	}

	// rewarded_video_adobj
	rc.RewardedVideo.PlayLocal = 1

	raw.VideoChoseCreatives(ctx)

	if raw.VideoChosen != nil && raw.ImageChosen != nil {

		video := &rc.RewardedVideo.Video
		video.Id = raw.VideoChosen.Id
		video.Url = ctx.CreativeCdnConv(raw.VideoChosen.Url, raw.VideoChosen.DomesticCDN)
		video.W = raw.VideoChosen.W
		video.H = raw.VideoChosen.H

		img := &rc.RewardedVideo.Img
		img.Id = raw.ImageChosen.Id
		img.Url = ctx.CreativeCdnConv(raw.ImageChosen.Url, raw.ImageChosen.DomesticCDN)
		img.W = raw.ImageChosen.Width
		img.H = raw.ImageChosen.Height
	} else {
		return nil
	}
	iconChosen := raw.getMatchedIcon(ctx.Lang)

	raw.AttachArgs = append(raw.AttachArgs, "city="+ctx.City)
	raw.AttachArgs = append(raw.AttachArgs, "region="+ctx.Region)
	raw.AttachArgs = append(raw.AttachArgs, "sv="+ctx.SdkVersion)
	raw.AttachArgs = append(raw.AttachArgs, "doimp=1")                      // tell imp monitor incr freq
	raw.AttachArgs = append(raw.AttachArgs, "pkg="+raw.AppDownload.PkgName) // offer pkg name
	raw.AttachArgs = append(raw.AttachArgs, "creative_id="+raw.VideoChosen.Id)
	raw.AttachArgs = append(raw.AttachArgs, "slot="+ctx.SlotId)
	raw.AttachArgs = append(raw.AttachArgs, "adtype="+ctx.AdType)
	raw.AttachArgs = append(raw.AttachArgs, "offer="+rc.Id)
	raw.AttachArgs = append(raw.AttachArgs, "imp="+rc.ImpId)
	raw.AttachArgs = append(raw.AttachArgs, "channel="+rc.Channel)
	raw.AttachArgs = append(raw.AttachArgs, "server_id="+ctx.ServerId)
	raw.AttachArgs = append(raw.AttachArgs, "method="+ctx.Method)
	raw.AttachArgs = append(raw.AttachArgs, "osv="+ctx.Osv)

	appArgsStr := ctx.PossiableArgs + "&" + strings.Join(raw.AttachArgs, "&")

	thirdImpTk := raw.genThirdPartyImpTk(ctx)
	imp := make([]string, len(ctx.PostImpTks))
	for i := 0; i != len(ctx.PostImpTks); i++ {
		if strings.Contains(ctx.PostImpTks[i], "?") {
			imp[i] = ctx.PostImpTks[i] + "&" + appArgsStr
		} else {
			imp[i] = ctx.PostImpTks[i] + "?" + appArgsStr
		}
	}
	if len(thirdImpTk) > 0 {
		imp = append(imp, thirdImpTk)
	}

	if len(imp) > 0 {
		rc.ImpTks = append(rc.ImpTks, imp...)
	}

	rc.ClkTks = util.AppendArgsToMonitors(ctx.PostClkTks, appArgsStr)
	rc.Title = raw.AppDownload.Title
	rc.Desc = raw.AppDownload.Desc
	rc.Icon = ctx.CreativeCdnConv(iconChosen.Url, iconChosen.DomesticCDN)

	vast_module.FillV4Vast(rc)

	return rc
}

func (raw *RawAdObj) ToNativeVideoV4Ad(ctx *http_context.Context) *ad.NativeVideoV4Ad {
	rc := &ad.NativeVideoV4Ad{
		Id:          raw.Id,
		ImpId:       ctx.ReqId + util.ToMd5(raw.Id+ctx.SlotId+ctx.Now.String()+ctx.Ran),
		LandingType: raw.LandingType,
		AdExp:       1000,
		Slot:        ctx.SlotId,
		Channel:     raw.Channel,
		Country:     ctx.Country,
		FinalUrl:    raw.FinalUrl,
		TaoBaoKe:    raw.TaoBaoKe,
		TaoBaoKeT:   raw.TaoBaoKeT,
		LoadTime:    ctx.LoadTime,
		ClickTime:   ctx.ClickTime,
		Lp:          1,
	}

	ctx.ImpId = rc.ImpId

	if clkUrlGenerator, ok := raw.GetChannelClkUrlGenerator(); ok {
		serverIdBak := ctx.ServerId // push server id
		if raw.VideoChosen != nil {
			ctx.ServerId = "v." + raw.VideoChosen.Id
		}
		rc.ClkUrl = clkUrlGenerator(raw, ctx)
		ctx.ServerId = serverIdBak // pop server id (为了保持报表不出问题)
	} else {
		ctx.L.Println("ad unknown channel of: ", raw.Channel)
		return nil
	}

	nativeAdObj := &rc.Core
	nativeAdObj.Button = ctx.ButtonText
	nativeAdObj.Star = rand.Float32() + 4
	nativeAdObj.Title = raw.AppDownload.Title
	nativeAdObj.Desc = raw.AppDownload.Desc
	nativeAdObj.ChoicesLinkUrl = "https://en.yeahmobi.com/privacy-policy"

	if video := raw.TryGetMatchedVideo(ctx); video != nil {
		rc.Video.Id = video.Id
		rc.Video.Url = ctx.CreativeCdnConv(video.Url, video.DomesticCDN)
		rc.Video.W = video.W
		rc.Video.H = video.H
	} else {
		return nil
	}

	if img := raw.getMatchedCreative(ctx.Lang, ctx.ImgW, ctx.ImgH, ctx.ImgRule); img != nil {
		rc.Image.Id = img.Id
		rc.Image.Url = ctx.CreativeCdnConv(img.Url, img.DomesticCDN)
		rc.Image.W = img.Width
		rc.Image.H = img.Height
		nativeAdObj.Image = img.Url
	} else {
		return nil
	}

	if iconChosen := raw.getMatchedIcon(ctx.Lang); iconChosen != nil {
		nativeAdObj.Icon = ctx.CreativeCdnConv(iconChosen.Url, iconChosen.DomesticCDN)
	} else {
		return nil
	}

	raw.AttachArgs = append(raw.AttachArgs, "city="+ctx.City)
	raw.AttachArgs = append(raw.AttachArgs, "region="+ctx.Region)
	raw.AttachArgs = append(raw.AttachArgs, "sv="+ctx.SdkVersion)
	raw.AttachArgs = append(raw.AttachArgs, "doimp=1")                      // tell imp monitor incr freq
	raw.AttachArgs = append(raw.AttachArgs, "pkg="+raw.AppDownload.PkgName) // offer pkg name
	raw.AttachArgs = append(raw.AttachArgs, "creative_id="+rc.Video.Id)
	raw.AttachArgs = append(raw.AttachArgs, "slot="+ctx.SlotId)
	raw.AttachArgs = append(raw.AttachArgs, "adtype="+ctx.AdType)
	raw.AttachArgs = append(raw.AttachArgs, "offer="+rc.Id)
	raw.AttachArgs = append(raw.AttachArgs, "imp="+rc.ImpId)
	raw.AttachArgs = append(raw.AttachArgs, "channel="+rc.Channel)
	raw.AttachArgs = append(raw.AttachArgs, "server_id="+ctx.ServerId)
	raw.AttachArgs = append(raw.AttachArgs, "method="+ctx.Method)
	raw.AttachArgs = append(raw.AttachArgs, "osv="+ctx.Osv)

	appArgsStr := ctx.PossiableArgs + "&" + strings.Join(raw.AttachArgs, "&")

	thirdImpTk := raw.genThirdPartyImpTk(ctx)
	imp := make([]string, len(ctx.PostImpTks))
	for i := 0; i != len(ctx.PostImpTks); i++ {
		if strings.Contains(ctx.PostImpTks[i], "?") {
			imp[i] = ctx.PostImpTks[i] + "&" + appArgsStr
		} else {
			imp[i] = ctx.PostImpTks[i] + "?" + appArgsStr
		}
	}
	if len(thirdImpTk) > 0 {
		imp = append(imp, thirdImpTk)
	}

	if len(imp) > 0 {
		rc.ImpTks = append(rc.ImpTks, imp...)
	}

	rc.ClkTks = util.AppendArgsToMonitors(ctx.PostClkTks, appArgsStr)

	vastXmlData := vast_module.NativeV4(rc)
	if len(vastXmlData) == 0 {
		ctx.L.Println("vast data empty ", raw.Channel, "_", raw.Id)
		return nil
	}
	rc.VastXmlData = vastXmlData

	return rc
}
