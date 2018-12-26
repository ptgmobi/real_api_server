package raw_ad

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"http_context"
)

func (raw *RawAdObj) GetChannelClkUrlGenerator() (genFunc, bool) {
	if strings.HasPrefix(raw.Channel, "dsp") {
		return cloudmobiGen, true
	}

	if genF, ok := channelClkUrlGenerator[raw.Channel]; ok {
		return genF, true
	}

	return nil, false
}

func (raw *RawAdObj) appendArg(arg string) string {
	if strings.Contains(raw.AppDownload.TrackLink, "?") {
		return raw.AppDownload.TrackLink + "&" + arg
	} else {
		return raw.AppDownload.TrackLink + "?" + arg
	}
}

type genFunc func(raw *RawAdObj, ctx *http_context.Context) string

func ymGen(raw *RawAdObj, ctx *http_context.Context) string {
	return raw.appendArg(raw.genYeahmobiClkUrlArg(ctx))
}

func glpGen(raw *RawAdObj, ctx *http_context.Context) string {
	return raw.appendArg(raw.genGlispaClkUrlArg(ctx))
}

func offlineReplaceMacro(url string, raw *RawAdObj, ctx *http_context.Context) string {
	// 对宏进行支持
	if ctx.Platform == "iOS" {
		url = strings.Replace(url, "{idfa}", ctx.Idfa, 1)
	} else {
		url = strings.Replace(url, "{gaid}", ctx.Gaid, 1)
		url = strings.Replace(url, "{aid}", ctx.Aid, 1)
	}
	t := time.Now()
	ts10 := fmt.Sprintf("%d", t.Unix())
	ts13 := fmt.Sprintf("%d", t.UnixNano()/1000000)
	url = strings.Replace(url, "{ts10}", ts10, 1)
	url = strings.Replace(url, "{ts13}", ts13, 1)
	url = strings.Replace(url, "{imp}", ctx.ImpId, 1)
	url = strings.Replace(url, "{slotid}", ctx.SlotId, 1)
	url = strings.Replace(url, "{ip}", ctx.IP, 1)

	return url
}

func offlineGen(raw *RawAdObj, ctx *http_context.Context) string {
	return offlineReplaceMacro(raw.AppDownload.TrackLink, raw, ctx)
}

func wbyGen(raw *RawAdObj, ctx *http_context.Context) string {
	return raw.genWebeye2ClkUrl(ctx)
}

func wbytGen(raw *RawAdObj, ctx *http_context.Context) string {
	return raw.genWebeyetClkUrl(ctx)
}

func ismtGen(raw *RawAdObj, ctx *http_context.Context) string {
	return raw.genInterestMobClkUrl(ctx)
}

// cloudmobi gen clk
func cloudmobiGen(raw *RawAdObj, ctx *http_context.Context) string {
	switch raw.AttPro {
	case 1: // adjust
		return raw.genAdjustClkUrl(ctx)
	case 2: // appsflyer
		return raw.genAppsFlyerClkUrl(ctx)
	case 3: // talking data
		return raw.genTalkingDataClkUrl(ctx)
	case 4: // 热云
		return raw.genReyunClkUrl(ctx)
	case 5: // Apptrack
		return raw.genApptrackClkUrl(ctx)
	default:
		return offlineGen(raw, ctx)
	}
}

// map: channel => click_url_generator
var channelClkUrlGenerator map[string]genFunc = map[string]genFunc{
	"ym":     ymGen,
	"rssym":  ymGen,
	"nym":    ymGen,
	"tym":    ymGen,
	"iym":    ymGen,
	"vym":    ymGen,
	"v1ym":   ymGen,
	"jpym":   ymGen,
	"jpym2":  ymGen,
	"stpym":  ymGen,
	"inym":   ymGen,
	"n1ym":   ymGen,
	"n2ym":   ymGen,
	"n3ym":   ymGen,
	"sym":    ymGen,
	"glp":    glpGen,
	"nglp":   glpGen,
	"gdt":    offlineGen,
	"xx":     offlineGen,
	"xxj":    offlineGen,
	"cm":     offlineGen,
	"im":     offlineGen,
	"qczj":   offlineGen,
	"wby2":   wbyGen,
	"wby3":   wbyGen,
	"wby4":   wbyGen,
	"wby5":   wbyGen,
	"wbyt":   wbytGen,
	"wbyti":  wbytGen,
	"wbyti2": wbytGen,
	"ismt":   ismtGen,
	"vcm":    cloudmobiGen,
	"mn":     cloudmobiGen,
	"um":     cloudmobiGen,
	"adcm":   adcamieGenClkUrl,
	"apl":    genAppleadsClkUrl,
	"irs": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.appendArg(raw.genIronSourceClkUrlArg(ctx))
	},
	"ppy": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.appendArg(raw.genPapayaClkUrlArg(ctx))
	},
	"apn": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.AppDownload.TrackLink + raw.genAppNextClkUrlArg(ctx)
	},
	"apn2": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.AppDownload.TrackLink + raw.genAppNextClkUrlArg(ctx)
	},
	"yai": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genYouAppiClkUrl(ctx)
	},
	"apa": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genAppiaClkUrl(ctx)
	},
	"zom": func(raw *RawAdObj, ctx *http_context.Context) string {
		arg := raw.genZoomyClkUrlArg(ctx)
		if strings.HasSuffix(raw.AppDownload.TrackLink, "&") {
			return raw.AppDownload.TrackLink + arg
		}
		return raw.AppDownload.TrackLink + "&" + arg
	},
	"pst": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genPingStartClkUrl(ctx)
	},
	"pst2": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genPingStartClkUrl(ctx)
	},
	"pst3": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genPingStart3ClkUrl(ctx)
	},
	"pstt": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genPingStartTClkUrl(ctx)
	},
	"mnd": func(raw *RawAdObj, ctx *http_context.Context) string {
		arg := raw.genMundoClkUrlArg(ctx)
		if strings.HasSuffix(raw.AppDownload.TrackLink, "/") {
			return raw.AppDownload.TrackLink + arg
		} else {
			return raw.AppDownload.TrackLink + "/" + arg
		}
	},
	"mnd2": func(raw *RawAdObj, ctx *http_context.Context) string {
		arg := raw.genMundoClkUrlArg(ctx)
		if strings.HasSuffix(raw.AppDownload.TrackLink, "/") {
			return raw.AppDownload.TrackLink + arg
		} else {
			return raw.AppDownload.TrackLink + "/" + arg
		}
	},
	"mas": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genMaasClkUrl(ctx)
	},
	"mbx": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genMobiMaxClkUrl(ctx)
	},
	"tpa": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.appendArg(raw.genTapticaClkUrlArg(ctx))
	},
	"tpa2": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.appendArg(raw.genTapticaClkUrlArg(ctx))
	},
	"vtpa": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.appendArg(raw.genTapticaClkUrlArg(ctx))
	},
	"wby": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genWebeyeClkUrl(ctx)
	},
	"nwby": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genWebeyeClkUrl(ctx)
	},
	"apsnt": func(raw *RawAdObj, ctx *http_context.Context) string {
		arg := raw.genAppSntClkUrlArg(ctx)
		if strings.HasSuffix(raw.AppDownload.TrackLink, "&") {
			return raw.AppDownload.TrackLink + arg
		} else {
			return raw.AppDownload.TrackLink + "&" + arg
		}
	},
	"apsnt2": func(raw *RawAdObj, ctx *http_context.Context) string {
		arg := raw.genAppSntClkUrlArg(ctx)
		if strings.HasSuffix(raw.AppDownload.TrackLink, "&") {
			return raw.AppDownload.TrackLink + arg
		} else {
			return raw.AppDownload.TrackLink + "&" + arg
		}
	},
	"wdg": func(raw *RawAdObj, ctx *http_context.Context) string {
		arg := raw.genWadogoClkUrlArg(ctx)
		if strings.HasSuffix(raw.AppDownload.TrackLink, "&") {
			return raw.AppDownload.TrackLink + arg
		} else {
			return raw.AppDownload.TrackLink + "&" + arg
		}
	},
	"wdg2": func(raw *RawAdObj, ctx *http_context.Context) string {
		arg := raw.genWadogoClkUrlArg(ctx)
		if strings.HasSuffix(raw.AppDownload.TrackLink, "&") {
			return raw.AppDownload.TrackLink + arg
		} else {
			return raw.AppDownload.TrackLink + "&" + arg
		}
	},
	"uci": func(raw *RawAdObj, ctx *http_context.Context) string {
		arg := raw.genUcUnionClkUrlArg(ctx)
		if strings.HasSuffix(raw.AppDownload.TrackLink, "&") {
			return raw.AppDownload.TrackLink + arg
		} else {
			return raw.AppDownload.TrackLink + "&" + arg
		}
	},
	"pbntv": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genPubNativeClkUrl(ctx)
	},
	"pbntv2": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genPubNativeClkUrl(ctx)
	},
	"fkt": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.AppDownload.TrackLink
	},
	"wmt": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.AppDownload.TrackLink
	},
	"adst": func(raw *RawAdObj, ctx *http_context.Context) string {
		arg := raw.genAdstrackClkUrlArg(ctx)
		if strings.HasSuffix(raw.AppDownload.TrackLink, "&") {
			return raw.AppDownload.TrackLink + arg
		}
		return raw.AppDownload.TrackLink + "&" + arg
	},
	"lb": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.appendArg(raw.genLiboClkUrlArg(ctx))
	},
	"stp": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genStartAppClkUrl(ctx)
	},
	"yoda": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.appendArg(raw.genYodaClkUrl(ctx))
	},
	"yoda2": func(raw *RawAdObj, ctx *http_context.Context) string {
		// track_link: https://s2s.adjust.com/2z1f8o?install_callback=http%3A%2F%2Flog.yodamob.com%2Fpostback%3Fydosid%3D58d4f139f2553d6e6df3016c%26ydofid%3Dyoda00000001
		// 替换后：https://s2s.adjust.com/2z1f8o?install_callback=http%3A%2F%2Flog.yodamob.com%2Fpostback%3Fydosid%3D58d4f139f2553d6e6df3016c%26ydofid%3Dyoda00000004%26clid=1010_libodeidfa__yoda00000001_iOS_com.test.libo_US_test_123-234-345_1.2_1_imp12345678_5.00_1490351063%26idfa=libodeidfa%26country=US%26price=5.00%26conv_ip=0.0.0.0&idfa=libodeidfa&s2s=1
		arg := raw.genYoda2ClkUrl(ctx)
		return raw.AppDownload.TrackLink +
			"%26" + url.QueryEscape(arg) + // %26: &
			"&idfa=" + ctx.Idfa + "&aid=" + ctx.Aid + "&s2s=1"
	},
	"mtm": func(raw *RawAdObj, ctx *http_context.Context) string {
		arg := raw.genMatomyClkUrlArg(ctx)
		if strings.HasSuffix(raw.AppDownload.TrackLink, "?") {
			return raw.AppDownload.TrackLink + arg
		}
		return raw.AppDownload.TrackLink + "?" + arg
	},
	"apc": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.appendArg(raw.genAppcoachClkUrl(ctx))
	},
	"rltm": func(raw *RawAdObj, ctx *http_context.Context) string { // inmobi的实时offer
		return raw.ClkUrl
	},
	"aft": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genAppliftClkUrl(ctx)
	},
	"mbs": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genMobusiClkUrl(ctx)
	},
	"mbp": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genMobuppsClkUrl(ctx)
	},
	"adt": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genAdtimingClkUrl(ctx)
	},
	"smt": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genSmarterClkUrl(ctx)
	},
	"mbm": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genMobiSummerClkUrl(ctx)
	},
	"mvt": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genMobvistaClkUrl(ctx)
	},
	"vmvt": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genMobvistaClkUrl(ctx)
	},
	"mv": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genMobvistaTClkUrl(ctx)
	},
	"wcl": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genWecloudClkUrl(ctx)
	},
	"ctl": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genCentrixlinkClkUrl(ctx)
	},
	"afl": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genAffleClkUrl(ctx)
	},
	"stmb": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genStarmobsClkUrl(ctx)
	},
	"stmb2": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genStarmobsClkUrl(ctx)
	},
	"owy": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genOnewayClkUrl(ctx)
	},
	"avz": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genAvazuClkUrl(ctx)
	},
	"avzt": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genAvazutClkUrl(ctx)
	},
	"smt2": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genSmarter2ClkUrl(ctx)
	},
	"dun": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genDuUnionClkUrl(ctx)
	},
	"dun2": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genDuUnionClkUrl(ctx)
	},
	"dunt": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genDuUnionTClkUrl(ctx)
	},
	"inmobi": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genInmobiClkUrl(ctx)
	},
	"cht": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.AppDownload.TrackLink
	},
	"smd": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genShootMediaClkUrl(ctx)
	},
	"smd2": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genShootMediaClkUrl(ctx)
	},
	"mbc": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genMobcastleClkUrl(ctx)
	},
	"seaec": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genSeaecClkUrl(ctx)
	},
	"med": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genMediuminClkUrl(ctx)
	},
	"ipb": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genInplayableClkUrl(ctx)
	},
	"bmb": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genBingmobClkUrl(ctx)
	},
	"sgd": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genSingleDogClkUrl(ctx)
	},
	"ldb": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genLeadboltClkUrl(ctx)
	},
	"mbd": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genMobduosClkUrl(ctx)
	},
	"nbd": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.genOffersLookClkUrl(ctx)
	},
	"huicheng": func(raw *RawAdObj, ctx *http_context.Context) string {
		return raw.AppDownload.TrackLink
	},
}
