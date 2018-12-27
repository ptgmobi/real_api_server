package huicheng

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"raw_ad"
	"strconv"
	"strings"
	"time"

	"http_context"
)

type Item struct {
	Action   int        `json:"action"`
	ImgList  []string   `json:"imglist"`
	Deeplink string     `json:"deeplink"`
	ClkUrl   string     `json:"clickurl"`
	Title    string     `json:"title"`
	Desc     string     `json:"desc"`
	Trackers []*Tracker `json:"trackers"`
	Html     string     `json:"html"`
}

type Tracker struct {
	Type string   `json:"type"`
	Urls []string `json:"urls"`
}

func (item *Item) ToImg() []raw_ad.Img {
	imgs := make([]raw_ad.Img, 0, 4)
	for _, url := range item.ImgList {
		if url != "" {
			imgs = append(imgs, raw_ad.Img{
				Width:  100,
				Height: 100,
				Url:    url,
				Lang:   "ALL",
			})
		}
	}
	return imgs
}

func (item *Item) ToRawAdObj() (*raw_ad.RawAdObj, error) {
	raw := raw_ad.NewRawAdObj()

	raw.Id = "360offer"
	raw.Channel = "huicheng"
	raw.FinalUrl = item.Deeplink
	if item.Action == 1 {
		raw.LandingType = raw_ad.INNER_LANDING
	} else if item.Action == 2 {
		raw.LandingType = raw_ad.APP_DOWNLOAD
	} else {
		raw.LandingType = raw_ad.EXTERN_LANDING
	}

	app := &raw.AppDownload
	app.Title = item.Title
	app.Desc = item.Desc
	app.Rate = rand.Float32() + 4
	app.TrackLink = item.ClkUrl
	if imgs := item.ToImg(); len(imgs) != 0 {
		raw.Icons["ALL"] = imgs
		raw.Creatives["ALL"] = imgs
	} else {
		return nil, fmt.Errorf("huicheng offer no imgs")
	}
	raw.ContentType = 2 // 2：下载类

	for _, track := range item.Trackers {
		if track.Type == "show" {
			raw.ThirdPartyImpTks = append(raw.ThirdPartyClkTks, track.Urls...)
		} else if track.Type == "click" {
			raw.ThirdPartyClkTks = append(raw.ThirdPartyClkTks, track.Urls...)
		}
	}

	return raw, nil
}

// TODO 考虑使用链接池节省消耗
func Request(api string, timeout int, ctx *http_context.Context) (*raw_ad.RawAdObj, error) {
	// 只要1：1
	ctx.ImgW, ctx.ImgH = 100, 100

	now := time.Now()
	params := make([]string, 0, 16)
	params = append(params, "bid="+strconv.Itoa(time.Now().Nanosecond()))
	params = append(params, "ver=1.2")
	params = append(params, "width="+strconv.Itoa(ctx.ImgW))
	params = append(params, "height="+strconv.Itoa(ctx.ImgH))
	params = append(params, "ts="+fmt.Sprintf("%d", now.UnixNano()/1000000))
	params = append(params, "reqtimes=1")
	params = append(params, "apppkg="+ctx.PkgName)
	params = append(params, "appver="+ctx.Params("msv"))
	params = append(params, "dt="+ctx.Device)
	params = append(params, "ost="+ctx.Platform)
	params = append(params, "osver="+ctx.Osv)
	params = append(params, "model=unknown")

	// XXX 没有imei
	if ctx.Platform == "iOS" {
		params = append(params, "idfa="+ctx.Idfa)
		params = append(params, "brand="+url.QueryEscape("Apple"))
	} else {
		params = append(params, "androidid="+ctx.Aid)
		params = append(params, "brand="+url.QueryEscape("Xiaomi"))
	}

	params = append(params, "ip="+ctx.Params("ip"))
	params = append(params, "ua="+url.QueryEscape(ctx.Params("ua")))
	if ctx.IsWifi() {
		params = append(params, "nt=WIFI")
	} else {
		params = append(params, "nt=4G")
	}
	params = append(params, "opt="+ctx.Params("cn"))

	uri := api + strings.Join(params, "&")
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("no huicheng offer")
	}

	var item Item
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, err
	}

	return item.ToRawAdObj()
}
