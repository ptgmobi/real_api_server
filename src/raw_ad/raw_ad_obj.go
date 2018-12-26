package raw_ad

import (
	"fmt"
	"html/template"
	"math/rand"
	"strconv"

	"ad"
	"http_context"
	"util"
)

func (raw *RawAdObj) ToAd(ctx *http_context.Context) *ad.AdObj {
	rc := &ad.AdObj{
		Id:          raw.Id,
		ImpId:       ctx.ReqId + raw.Id + ctx.SlotId,
		LandingType: raw.LandingType,
		AdExp:       1000,
		ClkTks:      raw.ClkTks,
		Slot:        ctx.SlotId,
		Channel:     raw.Channel,
		FinalUrl:    raw.FinalUrl,
		TaoBaoKe:    raw.TaoBaoKe,
		TaoBaoKeT:   raw.TaoBaoKeT,
		UrlSchema:   raw.UrlSchema,

		// cookie for jstag
		Ck: ctx.Ck,
	}

	rc.SetPreClick(ctx.IsWugan())
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

	// TODO: handle deep link

	// PreCreativeObj
	pre := &rc.PreCreative
	pre.CreativeType = -1 // XXX: 暂时先默认没有preCreative

	// BakCreativeObj
	bak := &rc.BakCreative
	bak.CreativeType = 1 // XXX: use h5 by default

	/*
	   switch bak.CreativeType {
	   case 0: // img
	       bak.Img = ...
	   case 1: // h5
	       bak.Html = ...
	   case 2: // video
	       bak.Video = ...
	   }
	*/

	if !ctx.IsWugan() {
		raw.IconChosen = raw.getMatchedIcon(ctx.Lang)

		m := make(map[string]string)
		m["{$title}"] = template.JSEscapeString(util.UnicodeReplace(raw.AppDownload.Title))
		m["{$g_title}"] = template.JSEscapeString(util.UnicodeReplace(raw.AppDownload.Title))
		if raw.IconChosen != nil {
			m["{$icon}"] = template.JSEscapeString(ctx.CreativeCdnConv(raw.IconChosen.Url, raw.IconChosen.DomesticCDN))
			m["{$g_icon}"] = template.JSEscapeString(ctx.CreativeCdnConv(raw.IconChosen.Url, raw.IconChosen.DomesticCDN))
		}
		m["{$desc}"] = template.JSEscapeString(util.UnicodeReplace(raw.AppDownload.Desc))
		m["{$g_desc}"] = template.JSEscapeString(util.UnicodeReplace(raw.AppDownload.Desc))

		m["{$btntext}"] = template.JSEscapeString(ctx.ButtonText)
		m["{$g_btntext}"] = template.JSEscapeString(ctx.ButtonText)

		// m["{$subtitle}"] = "" // 不替换，让js模板使用默认值
		// m["{$g_subtitle}"] = "" // 不替换，让js模板使用默认值
		m["{$rank}"] = fmt.Sprintf("%.1f", raw.AppDownload.Rate)
		m["{$g_rank}"] = fmt.Sprintf("%.1f", raw.AppDownload.Rate)
		// m["{$acicon}"] = ""
		// m["{$g_acicon}"] = ""
		// m["{$aclink}"] = ""
		// m["{$g_aclink}"] = ""
		m["{$price}"] = fmt.Sprintf("%.2f", raw.Payout)
		m["{$g_price}"] = fmt.Sprintf("%.2f", raw.Payout)

		m["{$choiceslink}"] = template.JSEscapeString(util.UnicodeReplace("https://en.yeahmobi.com/privacy-policy/"))

		tpl := raw.ReplaceTpl(ctx.Template, ctx.H5tpl, m, ctx)

		tplBase64, _ := util.Base64Encode(tpl)
		bak.Html = string(tplBase64)
	} else {
		tplBase64, _ := util.Base64Encode([]byte("<html></html>"))
		bak.Html = string(tplBase64) // 不填空字符串，防止客户端崩溃
	}

	raw.AttachArgs = append(raw.AttachArgs, "method="+ctx.Method)
	raw.AttachArgs = append(raw.AttachArgs, "sv="+ctx.SdkVersion)
	raw.AttachArgs = append(raw.AttachArgs, "pck="+strconv.Itoa(rc.PreClick))
	raw.AttachArgs = append(raw.AttachArgs, "doimp=1") // tell imp monitor incr freq
	raw.AttachArgs = append(raw.AttachArgs, "city="+ctx.City)
	raw.AttachArgs = append(raw.AttachArgs, "region="+ctx.Region)
	raw.AttachArgs = append(raw.AttachArgs, "pkg="+raw.AppDownload.PkgName) // offer pkg name
	raw.AttachArgs = append(raw.AttachArgs, "creative_id="+raw.CreativeId())

	thirdTk := raw.genThirdPartyImpTk(ctx)
	rc.SetTks(ctx, raw.AttachArgs, thirdTk)

	// 猎豹品牌广告曝光链接
	if raw.Channel == "cht" {
		bak.BakImpTkUrl = append(bak.BakImpTkUrl, raw.ThirdPartyImpTks...)
		bak.BakClkTkUrl = append(bak.BakClkTkUrl, raw.ThirdPartyClkTks...)

		// 30% 概率多加一次
		if rand.Float64() <= 0.3 {
			bak.BakImpTkUrl = append(bak.BakImpTkUrl, raw.ThirdPartyImpTks...)
		}
	}

	return rc
}
