package subscription

import (
	"fmt"
	"math/rand"
	"strings"

	"http_context"
)

type WemobiResponse struct {
	Status   int      `json:"status"`
	OfferId  int      `json:"offerId"`
	OfferUrl string   `json:"offerUrl"`
	JS       []string `json:"js"`

	clickId   string
	clickArgs string
}

func (w WemobiResponse) genClickUrl() string {
	if strings.Contains(w.OfferUrl, "?") {
		if strings.HasSuffix(w.OfferUrl, "?") || strings.HasSuffix(w.OfferUrl, "&") {
			return w.OfferUrl + "clickid=" + w.clickId
		}
		return w.OfferUrl + "&clickid=" + w.clickId
	}
	return w.OfferUrl + "?clickid=" + w.clickId
}

func (w WemobiResponse) toSubscription(ch string, ctx *http_context.Context) *Subscription {
	if len(w.OfferUrl) == 0 {
		return nil
	}

	sub := &Subscription{
		UseWebView: 1,
		Report:     fmt.Sprintf(gSubControl.reportUrl, ch, w.OfferId, w.clickArgs),
		ClkUrl:     w.genClickUrl(),
		ImpUrl:     "",
		Js:         w.JS,
	}
	return sub
}

func GetMoreWemobiJs(ctx *http_context.Context) ([]*Subscription, string) {
	subs := make([]*Subscription, 0, 1)
	clickId, clickArgs := getClickId("wmd", ctx)
	redirectUrl := ""

	parameters := make([]string, 0, 7)
	parameters = append(parameters, "pkg="+ctx.PkgName)
	parameters = append(parameters, "aid="+ctx.Aid)
	parameters = append(parameters, "plmn="+ctx.Mcc+ctx.Mnc)
	parameters = append(parameters, "chid="+ctx.SlotId)

	wemobiUrl := gSubControl.wemobiUrl + "?" + strings.Join(parameters, "&")
	header := make(map[string]string, 1)
	header["Accept-Encoding"] = "gzip, deflate"

	var wemobiResponse WemobiResponse
	if err := HttpGet(header, 700, wemobiUrl, &wemobiResponse); err != nil {
		ctx.L.Println("GetMoreWemobiJs err: ", err, " api: ", wemobiUrl)
		return subs, redirectUrl
	}

	if wemobiResponse.Status != 1 {
		if rand.Intn(100) == 1 {
			ctx.L.Println("GetMoreWemobiJs status not 1, status: ", wemobiResponse.Status)
			return subs, redirectUrl
		}
	}

	wemobiResponse.clickId = clickId
	wemobiResponse.clickArgs = clickArgs

	if sub := wemobiResponse.toSubscription("wmb", ctx); sub != nil {
		subs = append(subs, sub)
	}
	return subs, redirectUrl
}
