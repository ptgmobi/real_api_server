package subscription

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"net/url"
	"strings"

	"http_context"
	"util"
)

type WewoResponse struct {
	Status    int     `json:"status"`
	OfferList []Offer `json:"offerList"`
}

type Offer struct {
	OfferId  int      `json:"offerid"`
	OfferUrl string   `json:"offeUrl"`
	JS       []string `json:"js"`
}

func (o *Offer) getWewoClickId(ch string, ctx *http_context.Context) (string, string) {
	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	impId := util.Rand8Bytes() + util.Rand8Bytes() + ctx.SlotId

	clickId := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%s_%s_%s", fmt.Sprintf("%s%d", ch, o.OfferId),
		ctx.SlotId, ctx.Country, ctx.SdkVersion, ctx.Ran, "knn", "subs",
		impId, ctx.Gaid, base64PkgName, ctx.Platform, ch)

	parameters := make([]string, 0, 15)
	parameters = append(parameters, "offer_id="+fmt.Sprintf("%s%d", ch, o.OfferId))
	parameters = append(parameters, "slot="+ctx.SlotId)
	parameters = append(parameters, "country="+ctx.Country)
	parameters = append(parameters, "sv="+ctx.SdkVersion)
	parameters = append(parameters, "ran="+ctx.Ran)
	parameters = append(parameters, "method=knn")
	parameters = append(parameters, "server_id="+ctx.ServerId)
	parameters = append(parameters, "imp="+impId)
	parameters = append(parameters, "gaid="+ctx.Gaid)
	parameters = append(parameters, "pkg_name="+base64PkgName)
	parameters = append(parameters, "platform="+ctx.Platform)
	parameters = append(parameters, "channel="+ch)

	parameters = append(parameters, "click_id="+util.GenClickId(ch))
	parameters = append(parameters, "clk_ts="+fmt.Sprintf("%d", ctx.Now.Unix()))
	parameters = append(parameters, "aid="+ctx.Aid)

	return clickId, strings.Join(parameters, "&")
}

func (o *Offer) toSubscription(ch string, ctx *http_context.Context) *Subscription {
	if len(o.OfferUrl) == 0 {
		return nil
	}

	cid, clickArgs := o.getWewoClickId(ch, ctx)
	base64Js := make([]string, 0, len(o.JS))
	for _, js := range o.JS {
		base64Js = append(base64Js, base64.StdEncoding.EncodeToString([]byte(js)))
	}

	o.OfferUrl = strings.Replace(o.OfferUrl, "{cloudclickid}", cid, 1)

	sub := &Subscription{
		UseWebView: 1,
		Report:     fmt.Sprintf(gSubControl.reportUrl, ch, o.OfferId, clickArgs),
		ClkUrl:     o.OfferUrl,
		ImpUrl:     "",
		Js:         base64Js,
	}
	return sub
}

func GetMoreWewoJs(ctx *http_context.Context) ([]*Subscription, string) {
	// wewo 支持的clickid不希望超过230个字符
	subs := make([]*Subscription, 0, 1)
	redirectUrl := ""

	// XXX 临时需求，wewo目前支持不了太多in的量
	if ctx.Country == "IN" && rand.Intn(100) < 90 {
		return subs, redirectUrl
	}

	parameters := make([]string, 0, 5)
	parameters = append(parameters, "p="+ctx.PkgName)
	parameters = append(parameters, "i="+ctx.IP)
	parameters = append(parameters, "u="+url.QueryEscape(ctx.UA))
	parameters = append(parameters, "androidId="+ctx.Aid)
	parameters = append(parameters, "gaid="+ctx.Gaid)

	wewoUrl := gSubControl.wewoUrl + "?" + strings.Join(parameters, "&")

	header := make(map[string]string, 1)
	header["Accept-Encoding"] = "gzip, deflate"

	var wewoResp WewoResponse
	if err := HttpGet(header, 700, wewoUrl, &wewoResp); err != nil {
		ctx.L.Println("GetMoreWewoJs err: ", err, " api: ", wewoUrl)
		return subs, redirectUrl
	}

	if wewoResp.Status != 1 {
		if rand.Intn(100) == 1 {
			ctx.L.Println("GetMoreWewoJs status not 1, status: ", wewoResp.Status)
		}
		return subs, redirectUrl
	}

	for _, o := range wewoResp.OfferList {
		if len(o.JS) == 0 {
			continue
		}
		if sub := o.toSubscription("wewo", ctx); sub != nil {
			subs = append(subs, sub)
		}
	}

	return subs, redirectUrl
}
