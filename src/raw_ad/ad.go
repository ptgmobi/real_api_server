package raw_ad

import (
	"http_context"
	"strings"
)

// 拼接监测链接
func (raw *RawAdObj) jointTks(ctx *http_context.Context) (imp, clk []string) {
	args := make([]string, 0, 16)
	args = append(args, "slot="+ctx.SlotId)
	args = append(args, "adtype="+ctx.AdType)
	args = append(args, "offer="+raw.Id)
	args = append(args, "imp="+ctx.ImpId)
	args = append(args, "channel="+raw.Channel)
	args = append(args, "server_id="+ctx.ServerId)
	args = append(args, "method="+ctx.Method)
	args = append(args, "sv="+ctx.SdkVersion)
	args = append(args, "doimp=1")
	args = append(args, "city="+ctx.City)
	args = append(args, "region="+ctx.Region)
	args = append(args, "pkg="+raw.AppDownload.PkgName)
	if raw.ImageChosen != nil {
		args = append(args, "image_id="+raw.ImageChosen.Id)
	}
	if raw.VideoChosen != nil {
		args = append(args, "video_id="+raw.VideoChosen.Id)
	}

	appendStr := ctx.PossiableArgs + "&" + strings.Join(args, "&")
	thirdImpTk := raw.genThirdPartyImpTk(ctx)

	imp = make([]string, len(ctx.PostImpTks))
	for i := 0; i != len(ctx.PostImpTks); i++ {
		if strings.Contains(ctx.PostImpTks[i], "?") {
			imp[i] = ctx.PostImpTks[i] + "&" + appendStr
		} else {
			imp[i] = ctx.PostImpTks[i] + "?" + appendStr
		}
	}
	if len(thirdImpTk) > 0 {
		imp = append(imp, thirdImpTk)
	}

	clk = make([]string, len(ctx.PostClkTks))
	for i := 0; i != len(ctx.PostClkTks); i++ {
		if strings.Contains(ctx.PostClkTks[i], "?") {
			clk[i] = ctx.PostClkTks[i] + "&" + appendStr
		} else {
			clk[i] = ctx.PostClkTks[i] + "?" + appendStr
		}
	}
	return
}
