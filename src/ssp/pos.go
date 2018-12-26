package ssp

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
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

type Pos struct {
	Prefix string
	Key    string
}

type PosSlice []*Pos

var h5tpl map[string]PosSlice

func init() {
	capture := make([]string, len(marcos))
	for i := 0; i != len(marcos); i++ {
		capture[i] = "(" + marcos[i] + ")"
	}
	pattern := strings.Join(capture, "|")
	reg = regexp.MustCompile(pattern)

	h5tpl = make(map[string]PosSlice, 64)
}

func scanTpl(h5 string) (PosSlice, bool) {
	sign := fmt.Sprintf("%x", md5.Sum([]byte(h5)))
	if ps, ok := h5tpl[sign]; ok {
		return ps, true
	}
	data, err := base64.StdEncoding.DecodeString(h5)
	if err != nil {
		fmt.Println("decode tpl err")
		return nil, false
	}
	tpl := string(data)

	subs := reg.Split(tpl, -1)
	if len(subs) == 0 {
		return nil, false
	}
	subKeys := reg.FindAllString(tpl, -1)
	posSlice := make(PosSlice, 0, len(subs))
	for i, subKey := range subKeys {
		posSlice = append(posSlice, &Pos{
			Prefix: subs[i],
			Key:    subKey,
		})
	}
	posSlice = append(posSlice, &Pos{Prefix: subs[len(subs)-1]})

	h5tpl[sign] = posSlice
	return posSlice, true
}
