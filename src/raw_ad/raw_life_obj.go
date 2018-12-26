package raw_ad

import (
	"math/rand"
	"strconv"
	"strings"

	"ad"
	"http_context"
	"util"
)

func (raw *RawAdObj) ToLifeAd(ctx *http_context.Context) *ad.LifeAdObj {

	rc := &ad.LifeAdObj{
		Id:          raw.Id,
		UniqId:      raw.UniqId,
		ImpId:       ctx.ReqId + raw.Id + strconv.Itoa(rand.Intn(100)) + ctx.SlotId,
		LandingType: raw.LandingType,
		AdExp:       1000,
		FinalUrl:    raw.FinalUrl,
		PkgName:     raw.AppDownload.PkgName,
		Country:     ctx.Country,
		Channel:     raw.Channel,
		UrlSchema:   raw.UrlSchema,
	}

	ctx.ImpId = rc.ImpId
	core := &rc.Core
	core.Title = raw.AppDownload.Title
	core.Desc = raw.AppDownload.Desc
	core.Star = raw.AppDownload.Rate
	core.Button = ctx.ButtonText
	core.OfferType = raw.ContentType // 1：下载类，2：非下载类
	core.Order = 1                   // TODO 暂时没有排序

	if icon := raw.getMatchedIcon(ctx.Lang); icon != nil {
		core.Icon = ctx.CreativeCdnConv(icon.Url, icon.DomesticCDN)
	} else {
		ctx.L.Println("ad haven't icon, id: ", raw.UniqId)
		return nil
	}

	if clkUrlGenerator, ok := raw.GetChannelClkUrlGenerator(); ok {
		oldSlotId := ctx.SlotId
		if raw.ReplaceSlotId {
			tmpSlots := util.GetReplacingSlotIds()
			tmpLen := len(tmpSlots)
			if tmpLen > 0 {
				ctx.SlotId = tmpSlots[rand.Intn(tmpLen)]
			}
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
	appArgs = append(appArgs, "slot=lf_"+ctx.SlotId)
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
	appArgs = append(appArgs, "pkg="+raw.AppDownload.PkgName)

	appArgsStr := strings.Join(appArgs, "&") + "&" + ctx.PossiableArgs

	rc.ImpTkUrl = util.AppendArgsToMonitors(ctx.PostImpTks, appArgsStr)
	thirdImpTk := raw.genThirdPartyImpTk(ctx)
	if len(thirdImpTk) > 0 {
		rc.ImpTkUrl = append(rc.ImpTkUrl, thirdImpTk)
	}
	rc.ClkTkUrl = util.AppendArgsToMonitors(ctx.PostClkTks, appArgsStr)
	return rc
}
