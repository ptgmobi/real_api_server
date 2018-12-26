package subscription

import (
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"sync"

	"http_context"
)

type KokResponse struct {
	Status     int      `json:"status"` // 1是成功
	Id         int      `json:"id"`
	Name       string   `json:"name"`
	ClickUrl   string   `json:"clickUrl"`
	ExpressUrl string   `json:"expressUrl"`
	ImageUrl   string   `json:"imageUrl"`
	Message    string   `json:"message"`
	Js         []string `json:"js"`

	ClickId   string `json:"-"`
	ClickArgs string `json:"-"`
}

func GetKokJs(ctx *http_context.Context, kokjs chan KokResponse, wg *sync.WaitGroup) {

	//kokUrl := "http://api.inbvur.com/co"
	clickId, clickArgs := getClickId("kok", ctx)
	parameters := make([]string, 0, 11)
	parameters = append(parameters, "s=2644")                     // subid
	parameters = append(parameters, "i="+ctx.IP)                  // ip
	parameters = append(parameters, "u="+url.QueryEscape(ctx.UA)) // ua
	parameters = append(parameters, "rt=api")                     // api
	parameters = append(parameters, "at=4")                       // 4:请求结果没有图片, 5:有一个竖图 6: 1200*627
	parameters = append(parameters, "imsi="+ctx.Aid)
	parameters = append(parameters, "imei="+ctx.Gaid)
	parameters = append(parameters, "androidId="+ctx.Aid)
	parameters = append(parameters, "gaid="+ctx.Gaid)
	parameters = append(parameters, "s1="+clickId) // 自定义参数
	parameters = append(parameters, "s2=")         // 自定义参数
	parametersStr := strings.Join(parameters, "&")

	kokUrl := gSubControl.kokUrl + "?" + parametersStr

	defer wg.Done()

	header := make(map[string]string, 1)
	header["Accept-Encoding"] = "gzip, deflate"
	var kokResponse KokResponse
	if err := HttpGet(header, 700, kokUrl, &kokResponse); err != nil {
		ctx.L.Println("GetKokJs err: ", err, " api: ", kokUrl)
		return
	}

	if kokResponse.Status != 1 { // 只有1是正常
		if rand.Intn(1000) == 1 {
			ctx.L.Println("GetKokJs kok status not 1, status:", kokResponse.Status, " message: ", kokResponse.Message)
		}
		return
	}
	kokResponse.ClickId = clickId
	kokResponse.ClickArgs = clickArgs
	kokjs <- kokResponse
}

func (kok *KokResponse) ToSubscription(ch string, ctx *http_context.Context) *Subscription {
	sub := &Subscription{
		UseWebView: 1,
		Report:     fmt.Sprintf(gSubControl.reportUrl, ch, kok.Id, kok.ClickArgs),
		ClkUrl:     kok.ClickUrl,
		ImpUrl:     "",
		ReplacePkg: GetReplacePkg(ctx, "kok"),
		Js:         kok.Js,
	}
	return sub
}

func GetMoreKokJs(ctx *http_context.Context) ([]*Subscription, string) {

	// 起几个线程去拉取offer
	var wg sync.WaitGroup
	subs := make([]*Subscription, 0, 2)
	kokjs := make(chan KokResponse, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go GetKokJs(ctx, kokjs, &wg)
	}

	wg.Wait()
	close(kokjs)

	for js := range kokjs {
		subs = append(subs, js.ToSubscription("kok", ctx))
	}

	return subs, ""
}
