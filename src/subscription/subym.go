package subscription

import (
	"fmt"
	"math/rand"
	"net/url"
	"strings"

	"http_context"
)

type SubymResp struct {
	Status   int      `json:"status"`
	OfferId  int      `json:"offerid"`
	OfferUrl string   `json:"offerUrl"`
	Js       []string `json:"js"`

	clickId   string
	clickArgs string
}

func (s *SubymResp) genClickUrl() string {
	if strings.Contains(s.OfferUrl, "?") {
		if strings.HasSuffix(s.OfferUrl, "?") || strings.HasSuffix(s.OfferUrl, "&") {
			return s.OfferUrl + "clickid=" + s.clickId
		}
		return s.OfferUrl + "&clickid=" + s.clickId
	}
	return s.OfferUrl + "?clickid=" + s.clickId
}

func (s *SubymResp) toSubscription(ch string, ctx *http_context.Context) *Subscription {
	if len(s.OfferUrl) == 0 {
		return nil
	}

	sub := &Subscription{
		UseWebView: 1,
		Report:     fmt.Sprintf(gSubControl.reportUrl, ch, s.OfferId, s.clickArgs),
		ClkUrl:     s.genClickUrl(),
		ImpUrl:     "",
		Js:         s.Js,
	}
	return sub
}

func GetMoreSubymJs(ctx *http_context.Context) ([]*Subscription, string) {

	subs := make([]*Subscription, 0, 1)
	clickId, clickArgs := getClickId("subym", ctx)
	redirectUrl := ""

	parameters := make([]string, 0, 7)
	parameters = append(parameters, "p="+ctx.PkgName)
	parameters = append(parameters, "i="+ctx.IP)
	parameters = append(parameters, "u="+url.QueryEscape(ctx.UA))
	parameters = append(parameters, "androidId="+ctx.Aid)
	parameters = append(parameters, "gaid="+ctx.Gaid)
	parameters = append(parameters, "mcc="+ctx.Mcc)
	parameters = append(parameters, "mnc="+ctx.Mnc)

	subymUrl := gSubControl.subymUrl + "?" + strings.Join(parameters, "&")
	header := make(map[string]string, 1)
	header["Accept-Encoding"] = "gzip, deflate"

	var subymResp SubymResp
	if err := HttpGet(header, 700, subymUrl, &subymResp); err != nil {
		ctx.L.Println("GetMoreSubymJs err: ", err, " api: ", subymUrl)
		return subs, redirectUrl
	}

	if subymResp.Status != 1 {
		if rand.Intn(100) == 1 {
			ctx.L.Println("GetMoreSubymJs status not 1, status: ", subymResp.Status)
		}
		return subs, redirectUrl
	}

	subymResp.clickId = clickId
	subymResp.clickArgs = clickArgs

	if sub := subymResp.toSubscription("subym", ctx); sub != nil {
		subs = append(subs, sub)
	}
	return subs, redirectUrl
}
