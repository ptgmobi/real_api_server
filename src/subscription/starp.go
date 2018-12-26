package subscription

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"net/url"
	"strings"

	"http_context"
)

type StarpResp struct {
	Status   string   `json:"status"`
	OfferId  string   `json:"offerid"`
	OfferUrl string   `json:"offerUrl"`
	JsArr    []string `json:"jsArr"`

	clickId   string
	clickArgs string
}

func (s StarpResp) genClickUrl() string {
	if strings.Contains(s.OfferUrl, "?") {
		if strings.HasSuffix(s.OfferUrl, "?") || strings.HasSuffix(s.OfferUrl, "&") {
			return s.OfferUrl + "clickid=" + s.clickId
		}
		return s.OfferUrl + "&clickid=" + s.clickId
	}
	return s.OfferUrl + "?clickid=" + s.clickId
}

func (s StarpResp) toSubscription(ch string, ctx *http_context.Context) *Subscription {
	if len(s.OfferUrl) == 0 {
		ctx.L.Println("Starp toSubscription, offer haven't url")
		return nil
	}

	base64Js := make([]string, 0, len(s.JsArr))
	for _, js := range s.JsArr {
		base64Js = append(base64Js, base64.StdEncoding.EncodeToString([]byte(js)))
	}

	sub := &Subscription{
		UseWebView: 1,
		Report:     fmt.Sprintf(gSubControl.reportUrl, ch, s.OfferId, s.clickArgs),
		ClkUrl:     s.genClickUrl(),
		ImpUrl:     "",
		Js:         base64Js,
	}
	return sub
}

func GetMoreStarpJs(ctx *http_context.Context) ([]*Subscription, string) {
	subs := make([]*Subscription, 0, 1)
	clickId, clickArgs := getClickId("starp", ctx)
	redirectUrl := ""

	parameters := make([]string, 0, 5)
	parameters = append(parameters, "p="+ctx.PkgName)
	parameters = append(parameters, "i="+ctx.IP)
	parameters = append(parameters, "u="+url.QueryEscape(ctx.UA))
	parameters = append(parameters, "androidId="+ctx.Aid)
	parameters = append(parameters, "gaid="+ctx.Gaid)
	parameters = append(parameters, "mcc="+ctx.Mcc)
	parameters = append(parameters, "mnc="+ctx.Mnc)

	starpUrl := gSubControl.starpUrl + "?" + strings.Join(parameters, "&")
	header := make(map[string]string, 1)
	header["Accept-Encoding"] = "gzip, deflate"

	var starpResp StarpResp
	if err := HttpGet(header, 700, starpUrl, &starpResp); err != nil {
		ctx.L.Println("GetMoreStarpJs err: ", err, " api: ", starpUrl)
		return subs, redirectUrl
	}

	if starpResp.Status != "1" {
		if rand.Intn(100) == 1 {
			ctx.L.Println("GetMoreStarpJs status not 1, status: ", starpResp.Status)
		}
		return subs, redirectUrl
	}

	starpResp.clickId = clickId
	starpResp.clickArgs = clickArgs

	if sub := starpResp.toSubscription("starp", ctx); sub != nil {
		subs = append(subs, sub)
	}
	return subs, redirectUrl
}
