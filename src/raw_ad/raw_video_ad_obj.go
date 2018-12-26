package raw_ad

import (
	"strconv"

	"ad"
	"http_context"
	"util"
)

func (raw *RawAdObj) ToVideoAd(ctx *http_context.Context) *ad.AdObj {
	rc := &ad.AdObj{
		Id:          raw.Id,
		ImpId:       ctx.ReqId + raw.Id + ctx.SlotId,
		LandingType: raw.LandingType,
		AdExp:       259200, // 默认72小时
		ClkTks:      raw.ClkTks,
		Slot:        ctx.SlotId,
		Channel:     raw.Channel,
		FinalUrl:    raw.FinalUrl,
		PlayNum:     3, // 默认播放次数
		UrlSchema:   raw.UrlSchema,

		// cookie for jstag
		Ck: ctx.Ck,
	}

	// XXX
	rc.SetPreClick(false)

	ctx.ImpId = rc.ImpId

	download := &rc.AppDownload
	download.Title = raw.AppDownload.Title
	download.Desc = raw.AppDownload.Desc
	download.PkgName = raw.AppDownload.PkgName
	download.Size = raw.AppDownload.Size
	download.Rate = raw.AppDownload.Rate
	download.Download = raw.AppDownload.Download
	download.Review = raw.AppDownload.Review

	download.IconUrl = make([]string, 0)
	download.ItlTKUrl = make([]string, 0)
	download.ActTkUrl = make([]string, 0)

	if clkUrlGenerator, ok := raw.GetChannelClkUrlGenerator(); ok {
		rc.ClkUrl = clkUrlGenerator(raw, ctx)
	} else {
		ctx.L.Println("ad unknown channel of: ", raw.Channel)
		return nil
	}

	// PreCreativeObj
	pre := &rc.PreCreative
	pre.CreativeType = -1 // XXX: 暂时先默认没有preCreative
	// TODO 前置创意

	// BakCreativeObj
	bak := &rc.BakCreative
	if len(raw.Videos) > 0 {
		bak.CreativeType = 2                       // 0:图片， 1:H5,  2:视频
		chosenVideo := raw.TryGetMatchedVideo(ctx) // 1: 横屏 2: 竖屏
		if chosenVideo == nil {
			ctx.L.Println("can't match video, id: ", raw.UniqId)
			return nil
		}
		bak.Video.Id = chosenVideo.Id
		bak.Video.Url = ctx.CreativeCdnConv(chosenVideo.Url, chosenVideo.DomesticCDN)
		bak.Video.W = chosenVideo.W
		bak.Video.H = chosenVideo.H

		if img := raw.getMatchedCreative(ctx.Lang, ctx.ImgW, ctx.ImgH, ctx.ImgRule); img != nil {
			raw.CreativeChosen = img
			bak.Img = ad.ImgObj{
				Id:  img.Id,
				Url: ctx.CreativeCdnConv(img.Url, img.DomesticCDN),
				W:   img.Width,
				H:   img.Height,
			}
		} else {
			ctx.L.Println("can't match video picture, id: ", raw.UniqId)
		}
	} else {
		bak.CreativeType = 1 // XXX: use h5 by default
	}

	if !ctx.IsWugan() { // adtype == 3 || adtype == 4
		icon := raw.getMatchedIcon(ctx.Lang)
		if icon == nil {
			ctx.L.Println("can't match icon, UniqId: ", raw.UniqId)
			return nil
		}
		raw.IconChosen = icon
		rc.IconChosen = ctx.CreativeCdnConv(icon.Url, icon.DomesticCDN)
	}

	tplBase64, _ := util.Base64Encode([]byte("<html></html>"))
	bak.Html = string(tplBase64) // 不填空字符串，防止客户端崩溃

	raw.AttachArgs = append(raw.AttachArgs, "city="+ctx.City)
	raw.AttachArgs = append(raw.AttachArgs, "region="+ctx.Region)
	raw.AttachArgs = append(raw.AttachArgs, "sv="+ctx.SdkVersion)
	raw.AttachArgs = append(raw.AttachArgs, "pck="+strconv.Itoa(rc.PreClick))
	raw.AttachArgs = append(raw.AttachArgs, "doimp=1")                      // tell imp monitor incr freq
	raw.AttachArgs = append(raw.AttachArgs, "pkg="+raw.AppDownload.PkgName) // offer pkg name
	raw.AttachArgs = append(raw.AttachArgs, "creative_id="+bak.Video.Id)

	thirdTk := raw.genThirdPartyImpTk(ctx)
	rc.SetTks(ctx, raw.AttachArgs, thirdTk)

	// 拼装VastWrapObj, Android < 2.1.0 iOS < 2.2.0
	if !ctx.IsVastInOfferServer() {
		if len(raw.Videos) > 0 {
			bak := &rc.BakCreative
			rc.VastUrl = raw.VastUrl
			rc.VastWrapObj.OfferId = raw.UniqId
			rc.VastWrapObj.Impressions = append(rc.VastWrapObj.Impressions, bak.BakImpTkUrl...)

			if ctx.SdkVersion == "i-1.5.3" || ctx.SdkVersion == "i-1.5.4" {
				// hot fix
				rc.VastWrapObj.CustomClicks = append(rc.VastWrapObj.CustomClicks, bak.BakClkTkUrl...)
				rc.VastWrapObj.ClickTracks = append(rc.VastWrapObj.ClickTracks, rc.ClkUrl)
			} else if (ctx.Platform == "Android" && ctx.SdkVersion <= "a-2.0.3") ||
				(ctx.Platform == "iOS" && ctx.SdkVersion <= "i-2.0.0") || (raw.FinalUrl == "") {
				// android <= 2.0.3 或 ios <= 2.0.0 用click_url填充click_through
				rc.VastWrapObj.ClickThroughs = append(rc.VastWrapObj.ClickThroughs, rc.ClkUrl)
				rc.VastWrapObj.ClickTracks = append(rc.VastWrapObj.ClickTracks, bak.BakClkTkUrl...)
			} else {
				rc.VastWrapObj.ClickThroughs = append(rc.VastWrapObj.ClickThroughs, raw.FinalUrl)
				rc.VastWrapObj.ClickTracks = append(rc.VastWrapObj.ClickTracks, bak.BakClkTkUrl...)
			}
		}
	}

	return rc
}
