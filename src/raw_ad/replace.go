package raw_ad

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"http_context"
	"ssp"
)

var marcos []string = []string{
	"\\{\\$icon\\}",
	"\\{\\$title\\}",
	"\\{\\$img_\\d+x\\d+\\}",
	"\\{\\$desc\\}",
	"\\{\\$btntext\\}",
	"\\{\\$subtitle\\}",
	"\\{\\$rank\\}",
	"\\{\\$acicon\\}",
	"\\{\\$aclink\\}",
	"\\{\\$price\\}",
	"\\{\\$vasttag\\}", // vast服务器请求地址
	"\\{\\$vastjs\\}",  // vast解析js脚本地址
	"\\{\\$img\\}",
	"\\{\\$choiceslink\\}",

	"\\{\\$g_icon\\}",
	"\\{\\$g_title\\}",
	"\\{\\$g_img_\\d+x\\d+\\}",
	"\\{\\$g_desc\\}",
	"\\{\\$g_btntext\\}",
	"\\{\\$g_subtitle\\}",
	"\\{\\$g_rank\\}",
	"\\{\\$g_acicon\\}",
	"\\{\\$g_aclink\\}",
	"\\{\\$g_price\\}",
}

var reg *regexp.Regexp

func init() {
	capture := make([]string, len(marcos))
	for i := 0; i != len(marcos); i++ {
		capture[i] = "(" + marcos[i] + ")"
	}
	pattern := strings.Join(capture, "|")
	reg = regexp.MustCompile(pattern)
}

func (raw *RawAdObj) contact(t *ssp.TemplateObj, m map[string]string, ctx *http_context.Context) string {
	res, creativeReplaced, replaced := make([]string, 0, len(m)), false, make(map[string]bool, len(m))
	for _, pos := range t.PosSlice {
		if strings.Contains(pos.Key, "{$g_img_") {
			var w, h int = 1, 1
			if _, err := fmt.Sscanf(pos.Key, "{$g_img_%dx%d}", &w, &h); err == nil {
				if creative := raw.getMatchedCreative(ctx.Lang, w, h, ctx.ImgRule); creative != nil {
					raw.CreativeChosen = creative
					res = append(res, pos.Prefix+ctx.CreativeCdnConv(creative.Url, creative.DomesticCDN))
					continue
				}
			}
		}

		if v, ok := m[pos.Key]; ok {
			if _, hasReplaced := replaced[pos.Key]; hasReplaced && !strings.HasPrefix(pos.Key, "{$g_") {
				res = append(res, pos.Prefix+pos.Key)
				continue
			}
			replaced[pos.Key] = true
			res = append(res, pos.Prefix+v)
			continue
		}

		if !creativeReplaced && strings.Contains(pos.Key, "{$img_") {
			creativeReplaced = true
			var w, h int = 1, 1
			if _, err := fmt.Sscanf(pos.Key, "{$img_%dx%d}", &w, &h); err == nil {
				if creative := raw.getMatchedCreative(ctx.Lang, w, h, ctx.ImgRule); creative != nil {
					raw.CreativeChosen = creative
					res = append(res, pos.Prefix+ctx.CreativeCdnConv(creative.Url, creative.DomesticCDN))
					continue
				}
			}
		}

		res = append(res, pos.Prefix+pos.Key)
	}
	return strings.Join(res, "")
}

func (raw *RawAdObj) ReplaceTpl(t *ssp.TemplateObj, tpl []byte, m map[string]string, ctx *http_context.Context) []byte {
	if t != nil && t.InitFlag {
		if res := raw.contact(t, m, ctx); res != "" {
			return []byte(res)
		}
	}

	replaced := make(map[string]bool)
	creativeReplaced := false

	return reg.ReplaceAllFunc(tpl, func(b []byte) []byte {
		capture := string(b)
		if v, ok := m[capture]; ok {
			// just replace the first one for non-global marco
			if _, hasReplaced := replaced[capture]; hasReplaced && !strings.HasPrefix(capture, "{$g_") {
				return b
			}
			replaced[capture] = true
			return []byte(v)
		}

		if !creativeReplaced && bytes.Contains(b, []byte("{$img_")) {
			creativeReplaced = true
			var w, h int = 1, 1
			if _, err := fmt.Sscanf(string(b), "{$img_%dx%d}", &w, &h); err == nil {
				creative := raw.getMatchedCreative(ctx.Lang, w, h, ctx.ImgRule)
				if creative != nil {
					raw.CreativeChosen = creative
					return []byte(ctx.CreativeCdnConv(creative.Url, creative.DomesticCDN))
				}
			}
		}

		if bytes.Contains(b, []byte("{$g_img_")) {
			var w, h int = 1, 1
			if _, err := fmt.Sscanf(string(b), "{$g_img_%dx%d}", &w, &h); err == nil {
				creative := raw.getMatchedCreative(ctx.Lang, w, h, ctx.ImgRule)
				if creative != nil {
					raw.CreativeChosen = creative
					return []byte(ctx.CreativeCdnConv(creative.Url, creative.DomesticCDN))
				}
			}
		}

		return b
	})
}
