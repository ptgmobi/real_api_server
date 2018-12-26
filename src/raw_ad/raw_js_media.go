package raw_ad

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"ad"
	"http_context"
)

var discount []string = []string{
	"70", "80", "90",
}

// logger.cloudmobi.net/(ios|android)/v1/(impression|click)?(params)
var trackPrefixFmt string = "http://logger.cloudmobi.net/%s/v1/%s?%s"

func (raw *RawAdObj) ToJsMediaAd(ctx *http_context.Context) *ad.JsMediaAdObj {
	idx := rand.Intn(len(discount))
	rc := &ad.JsMediaAdObj{
		Title: raw.AppDownload.Title,
		Desc:  raw.AppDownload.Desc,
		// Image:
		// Icon:
		Button: discount[idx] + "% Discount",
		// ClkUrl:
		// ImpTrack:
		// ClkTrack:
	}

	ctx.ImpId = ctx.ReqId + raw.Id + strconv.Itoa(rand.Intn(100)) + ctx.SlotId

	creativeList := raw.Creatives[ctx.Country]
	if creativeList == nil {
		creativeList = raw.Creatives["ALL"]
	}
	if creativeList == nil {
		return nil
	}
	imgIdx := rand.Intn(len(creativeList))
	rc.Image = creativeList[imgIdx].Url

	iconList := raw.Icons["ALL"]
	if len(iconList) > 0 {
		rc.Icon = iconList[rand.Intn(len(iconList))].Url
	}
	if clkUrlGenerator, ok := raw.GetChannelClkUrlGenerator(); ok {
		rc.ClkUrl = clkUrlGenerator(raw, ctx)
	} else {
		return nil
	}

	trackArgs := make([]string, 0, 12)
	trackArgs = append(trackArgs, "slot="+ctx.SlotId)
	trackArgs = append(trackArgs, "imp="+ctx.ImpId)
	trackArgs = append(trackArgs, "creative_id="+creativeList[imgIdx].Id)
	trackArgs = append(trackArgs, "channel="+raw.Channel)
	trackArgs = append(trackArgs, "country="+ctx.Country)
	trackArgs = append(trackArgs, "uniq_id="+raw.UniqId)
	trackArgs = append(trackArgs, "adtype=18") // 18: jstag intersitial

	trackArgsStr := strings.Join(trackArgs, "&")
	rc.ImpTrack = fmt.Sprintf(trackPrefixFmt, strings.ToLower(ctx.Platform), "impression", trackArgsStr)
	rc.ClkTrack = fmt.Sprintf(trackPrefixFmt, strings.ToLower(ctx.Platform), "click", trackArgsStr)

	return rc
}
