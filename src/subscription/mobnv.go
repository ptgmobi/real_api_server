package subscription

import (
	"fmt"
	"math/rand"
	"net/url"
	"strings"

	"http_context"
)

type MobnvResp struct {
	Status   string   `json:"status"`
	OfferId  string   `json:"offerid"`
	OfferUrl string   `json:"offerUrl"`
	Js       []string `json:"js"`

	clickId   string
	clickArgs string
}

func (m MobnvResp) genClickUrl() string {
	if strings.Contains(m.OfferUrl, "?") {
		if strings.HasSuffix(m.OfferUrl, "?") || strings.HasSuffix(m.OfferUrl, "&") {
			return m.OfferUrl + "click=" + m.clickId
		}
		return m.OfferUrl + "&clickid=" + m.clickId
	}
	return m.OfferUrl + "?clickid=" + m.clickId
}

func (m MobnvResp) toSubscription(ch string, ctx *http_context.Context) *Subscription {
	if len(m.OfferUrl) == 0 {
		ctx.L.Println("Mobnv toSubscription, offer haven't url")
		return nil
	}

	sub := &Subscription{
		UseWebView: 1,
		Report:     fmt.Sprintf(gSubControl.reportUrl, ch, m.OfferId, m.clickArgs),
		ClkUrl:     m.genClickUrl(),
		ImpUrl:     "",
		Js:         m.Js,
	}

	return sub
}

func GetMoreMobnvJs(ctx *http_context.Context) ([]*Subscription, string) {
	subs := make([]*Subscription, 0, 1)
	clickId, clickArgs := getClickId("mbn", ctx)
	redirectUrl := ""

	parameters := make([]string, 0, 5)
	parameters = append(parameters, "p="+ctx.PkgName)
	parameters = append(parameters, "i="+ctx.IP)
	parameters = append(parameters, "u="+url.QueryEscape(ctx.UA))
	parameters = append(parameters, "androidId="+ctx.Aid)
	parameters = append(parameters, "gaid="+ctx.Gaid)
	parameters = append(parameters, "mcc="+ctx.Mcc)
	parameters = append(parameters, "mnc="+ctx.Mnc)

	mobnvUrl := gSubControl.mobnvUrl + "?" + strings.Join(parameters, "&")
	header := make(map[string]string, 1)
	header["Accept-Encoding"] = "gzip, deflate"

	var mobnvResp MobnvResp
	if err := HttpGet(header, 700, mobnvUrl, &mobnvResp); err != nil {
		ctx.L.Println("GetMoreMobnvJs err: ", err, " api: ", mobnvUrl)
		return subs, redirectUrl
	}

	if mobnvResp.Status != "1" {
		if rand.Intn(100) == 1 {
			ctx.L.Println("GetMoreMobnvJs status not 1, status: ", mobnvResp.Status)
		}
		return subs, redirectUrl
	}

	mobnvResp.clickId = clickId
	mobnvResp.clickArgs = clickArgs

	if sub := mobnvResp.toSubscription("mbn", ctx); sub != nil {
		subs = append(subs, sub)
	}
	return subs, redirectUrl
}
