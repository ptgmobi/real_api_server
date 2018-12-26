package subscription

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	dnf "github.com/brg-liuwei/godnf"

	"http_context"
	"raw_ad"
)

type Top1Response struct {
	OfferId string
	ClkUrl  string
	Js      []string

	ClickId    string
	ReportId   string
	ReportArgs string
}

func GetTop1Js(ctx *http_context.Context, conds []dnf.Cond) *Top1Response {
	raw := dnfRetrieval(ctx, conds)
	if raw == nil {
		return nil
	}

	top1Resp := new(Top1Response)
	top1Resp.OfferId = raw.Id
	top1Resp.ClkUrl = raw.Subscription.TrackLink
	top1Resp.Js = raw.Subscription.Js
	payout := strconv.FormatFloat(float64(raw.Payout), 'f', 4, 64)
	top1Resp.ReportId, top1Resp.ReportArgs = getClickId("top1", ctx)
	top1Resp.ClickId = top1Resp.ReportId + "_" + payout
	return top1Resp
}

func (top1 *Top1Response) genClickUrl(ctx *http_context.Context) string {
	if strings.Contains(top1.ClkUrl, "?") {
		if strings.HasSuffix(top1.ClkUrl, "?") || strings.HasSuffix(top1.ClkUrl, "&") {
			return top1.ClkUrl + "cid=" + top1.ClickId + "&ac=" + ctx.SlotId
		}
		return top1.ClkUrl + "&cid=" + top1.ClickId + "&ac=" + ctx.SlotId
	}
	return top1.ClkUrl + "?cid=" + top1.ClickId + "&ac=" + ctx.SlotId
}

func (top1 *Top1Response) ToSubscription(ch string, ctx *http_context.Context) *Subscription {
	sub := &Subscription{
		UseWebView: 1,
		Report:     fmt.Sprintf(gSubControl.reportUrl, ch, top1.OfferId, top1.ReportArgs),
		ClkUrl:     top1.genClickUrl(ctx),
		ImpUrl:     "",
		Js:         top1.Js,
	}
	return sub
}

func GetMoreTop1Js(ctx *http_context.Context, conds []dnf.Cond) ([]*Subscription, string) {
	js := GetTop1Js(ctx, conds)
	if js == nil {
		return nil, ""
	}
	return []*Subscription{js.ToSubscription("top1", ctx)}, ""
}

func dnfRetrieval(ctx *http_context.Context, conds []dnf.Cond) *raw_ad.RawAdObj {
	handler := dnf.GetHandler()
	if handler == nil {
		return nil
	}

	docs, _ := handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)
		if raw.Channel != "top1" {
			return false
		}
		if raw.ProductCategory != "subscription" {
			return false
		}

		return true
	})

	ndocs := len(docs)
	if ndocs == 0 {
		return nil
	}
	i := rand.Intn(ndocs)
	rawAdInter, _ := handler.DocId2Attr(docs[i])
	raw := rawAdInter.(*raw_ad.RawAdObj)
	return raw
}
