package raw_ad

import (
	"ad"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"math/rand"
	"strconv"
	"strings"

	"http_context"
	"util"
)

func (raw *RawAdObj) ToInterstitialAd(ctx *http_context.Context) *ad.InterstitialAd {
	rc := &ad.InterstitialAd{
		Id:          raw.Id,
		ImpId:       ctx.ReqId + raw.Id + ctx.SlotId,
		Channel:     raw.Channel,
		Slot:        ctx.SlotId,
		Country:     ctx.Country,
		LandingType: raw.LandingType,
		AdExp:       1000,
		CountDown:   5,
		ClkTks:      raw.ClkTks,
		TaoBaoKe:    raw.TaoBaoKe,
		TaoBaoKeT:   raw.TaoBaoKeT,
		UrlSchema:   raw.UrlSchema,
	}

	ctx.ImpId = rc.ImpId

	download := &rc.AppDownload
	download.Title = raw.AppDownload.Title
	download.Desc = raw.AppDownload.Desc
	download.PkgName = raw.AppDownload.PkgName
	download.Size = raw.AppDownload.Size
	download.Rate = raw.AppDownload.Rate
	download.Download = raw.AppDownload.Download
	download.Review = raw.AppDownload.Review

	// 生成点击链接，监测链接
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
		ctx.L.Println("interstitial ad unknow channel of: ", raw.Channel)
		return nil
	}

	// 素材选择
	if err := raw.ChoseInterstitialCreative(ctx); err != nil {
		ctx.L.Println("intersitital ad chose creative false: ", raw.UniqId, ", because ", err)
		return nil
	}

	raw.AttachArgs = append(raw.AttachArgs, "method="+ctx.Method)
	raw.AttachArgs = append(raw.AttachArgs, "sv="+ctx.SdkVersion)
	raw.AttachArgs = append(raw.AttachArgs, "pck="+strconv.Itoa(rc.PreClick))
	raw.AttachArgs = append(raw.AttachArgs, "doimp=1") // tell imp monitor incr freq
	raw.AttachArgs = append(raw.AttachArgs, "city="+ctx.City)
	raw.AttachArgs = append(raw.AttachArgs, "region="+ctx.Region)
	raw.AttachArgs = append(raw.AttachArgs, "pkg="+raw.AppDownload.PkgName) // offer pkg name
	if ctx.IsVideoTpl() {
		raw.AttachArgs = append(raw.AttachArgs, "creative_id="+raw.VideoChosen.Id)
	} else {
		raw.AttachArgs = append(raw.AttachArgs, "creative_id="+raw.ImageChosen.Id)
	}

	thirdTk := raw.genThirdPartyImpTk(ctx)
	rc.SetTks(ctx, raw.AttachArgs, thirdTk)

	// BackCreativeObj
	bak := &rc.BakCreative
	bak.CreativeType = 0 // 0 图片 2 视频

	choiceslink := "https://en.yeahmobi.com/privacy-policy/"
	// 根据模板填充广告
	// 模板宏替换
	m := make(map[string]string)

	m["{$icon}"] = template.JSEscapeString(util.UnicodeReplace(raw.IconChosen.Url))
	// img
	m["{$img}"] = template.JSEscapeString(util.UnicodeReplace(raw.ImageChosen.Url))
	// title
	m["{$title}"] = template.JSEscapeString(util.UnicodeReplace(raw.AppDownload.Title))
	// btntext
	m["{$btntext}"] = template.JSEscapeString(util.UnicodeReplace(ctx.ButtonText))
	// choiceslink
	m["{$choiceslink}"] = template.JSEscapeString(util.UnicodeReplace(choiceslink))

	if ctx.IsVideoTpl() {
		uid := raw.UniqId
		vid := raw.VideoChosen.Id
		mid := raw.ImageChosen.Id
		clk := rc.ClkUrl
		impTks := strings.Join(bak.BakImpTkUrl, ";;")
		clkTks := strings.Join(bak.BakClkTkUrl, ";;")
		bak.CreativeType = 2

		vastTag := fmt.Sprintf(
			"%s&uid=%s&vid=%s&mid=%s&platform=%s&country=%s&clk=%s&imptks=%s&clktks=%s&choiceslink=%s",
			ctx.VastServerApi, uid, vid, mid, ctx.Platform, ctx.Country,
			base64.StdEncoding.EncodeToString([]byte(clk)),
			base64.StdEncoding.EncodeToString([]byte(impTks)),
			base64.StdEncoding.EncodeToString([]byte(clkTks)),
			base64.StdEncoding.EncodeToString([]byte(choiceslink)),
		)
		m["{$vasttag}"] = template.JSEscapeString(util.UnicodeReplace(vastTag))
		m["{$vastjs}"] = template.JSEscapeString(util.UnicodeReplace(ctx.VastJsUrl))
	}

	tpl := raw.ReplaceTpl(ctx.Template, ctx.H5tpl, m, ctx)
	tplBase64, _ := util.Base64Encode(tpl)
	bak.Html = string(tplBase64)
	return rc
}

func (raw *RawAdObj) ChoseInterstitialCreative(ctx *http_context.Context) error {
	// icon
	icon := raw.getMatchedIcon(ctx.Lang)
	if icon == nil {
		return errors.New("no match icon")
	}
	raw.IconChosen = icon

	// image
	image := raw.getMatchedCreative(ctx.Lang, ctx.ImgW, ctx.ImgH, ctx.ImgRule)
	if image == nil {
		return errors.New("no match image")
	}
	raw.ImageChosen = image

	// video
	video := raw.TryGetMatchedVideo(ctx)
	if video == nil {
		return errors.New("no match video")
	}
	raw.VideoChosen = video

	return nil
}

func (raw *RawAdObj) HasInterstitialCreative(ctx *http_context.Context) bool {
	// XXX 目前选择视频广告
	if !raw.HasMatchedNativeVideo(ctx) {
		return false
	}

	// 是否匹配图片
	if !raw.HasMatchedCreative(ctx) {
		return false
	}

	return true
}
