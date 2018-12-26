package raw_ad

import (
	"ad"
	"bytes"
	"fmt"
	"math/rand"
	"strings"

	"http_context"
)

var (
	err         error
	choiceslink = "https://en.yeahmobi.com/privacy-policy/"
	trackEvents = map[string]string{
		"creativeView":  "0",
		"start":         "1",
		"firstQuartile": "2",
		"midpoint":      "3",
		"thirdQuartile": "4",
		"complete":      "5",
		"closeLinear":   "16",
	}
	playableMap = map[string]string{
		"v1ym_12077400": "http://static.zcoup.com/playable/words/index.html",
	}
)

// pagead 广告生成
type PageAdData struct {
	Title       string      `json:"title"`
	Desc        string      `json:"description"`
	Icon        string      `json:"icon"`
	Rate        float32     `json:"rate"`
	Review      int         `json:"review"`
	Img         string      `json:"img"`
	ImgW        int         `json:"img_w"`
	ImgH        int         `json:"img_h"`
	Button      string      `json:"button"`
	Format      string      `json:"format"`
	TplType     string      `json:"tpl_type"`
	AdW         int         `json:"ad_w"`
	AdH         int         `json:"ad_h"`
	ClkUrl      string      `json:"click_url"`
	FinalUrl    string      `json:"final_url"`
	ChoicesLink string      `json:"choices_link"`
	ClkTks      []string    `json:"clk_tks"`
	ImpTks      []string    `json:"imp_tks"`
	Playable    string      `json:"playable,omitempty"` // 可玩页面，前期测试调研
	Video       *VideoObj   `json:"video,omitempty"`    // video数据
	Control     interface{} `json:"control,omitempty"`  // 控制结构
}

type VideoObj struct {
	Url         string            `json:"url"`
	TrackEvents map[string]string `json:"track_events"`
}

func (raw *RawAdObj) ToPageadBanner(ctx *http_context.Context) *ad.AdObjV2 {
	var rc ad.AdObjV2
	var pageadObj ad.Pagead
	rc.PageadObj = &pageadObj

	ctx.ImpId = ctx.ReqId + raw.Id + ctx.SlotId

	// 生成点击链接，监测链接
	clkUrlGenerator, ok := raw.GetChannelClkUrlGenerator()
	if !ok {
		ctx.L.Println("[pagead] ad unknown channel of: ", raw.Channel)
		return nil
	}

	adapters := raw.MatchedAdpaters(ctx)
	if len(adapters) == 0 {
		ctx.L.Println("[pagead] not matched adapter")
		return nil
	}

	// 随机模板
	aptChosen := adapters[rand.Intn(len(adapters))]
	// 素材选择
	raw.choseAdapterCreative(aptChosen, ctx.Lang, ctx.ImgRule)

	// creative_id拼装到serverId中
	serverIdBak := ctx.ServerId // push server id
	if raw.VideoChosen != nil {
		ctx.ServerId = "v." + raw.VideoChosen.Id
	} else if raw.ImageChosen != nil {
		ctx.ServerId = "v." + raw.ImageChosen.Id
	}
	clkUrl := clkUrlGenerator(raw, ctx)
	// 生成点击链接后pop
	ctx.ServerId = serverIdBak // pop server id

	impTks, clkTks := raw.jointTks(ctx)

	if ctx.IsDebug {
		rc.CommonObj = &ad.Common{
			Id:       raw.Id,
			ImpId:    ctx.ImpId,
			Channel:  raw.Channel,
			Slot:     ctx.SlotId,
			Country:  ctx.Country,
			ClkUrl:   clkUrl,
			FinalUrl: raw.FinalUrl,
			ClkTks:   clkTks,
			ImpTks:   impTks,
		}
	}

	adData := &PageAdData{
		Title:       raw.AppDownload.Title,
		Desc:        raw.AppDownload.Desc,
		Rate:        raw.AppDownload.Rate,
		Review:      raw.AppDownload.Review,
		Button:      ctx.ButtonText,
		Format:      ctx.Format,
		TplType:     aptChosen,
		AdW:         ctx.AdW,
		AdH:         ctx.AdH,
		ClkUrl:      clkUrl,
		FinalUrl:    raw.FinalUrl,
		ClkTks:      clkTks,
		ImpTks:      impTks,
		ChoicesLink: choiceslink,
		Control:     ctx.Control,
	}

	// NOTE 可玩广告试投放，用于产品调研，完成后下掉
	if ctx.Country == "US" && ctx.Platform == "iOS" {
		if url, ok := playableMap[raw.UniqId]; ok {
			adData.Playable = url
		}
	}

	// 必须带有icon
	if raw.IconChosen != nil {
		adData.Icon = ctx.CreativeCdnConv(raw.IconChosen.Url, raw.IconChosen.DomesticCDN)
	} else {
		ctx.L.Println("[pagead] banner no chosen icon")
		return nil
	}

	if raw.ImageChosen != nil {
		adData.Img = ctx.CreativeCdnConv(raw.ImageChosen.Url, raw.ImageChosen.DomesticCDN)
		adData.ImgW = raw.ImageChosen.Width
		adData.ImgH = raw.ImageChosen.Height
	}

	// 选中视频
	if raw.VideoChosen != nil {
		// NOTE 由于mraid中vpaid未弄清楚，目前将视频一json格式传递给js，
		// 让js进行视频的展示与播放

		var video VideoObj
		video.Url = ctx.CreativeCdnConv(raw.VideoChosen.Url, raw.VideoChosen.DomesticCDN)
		video.TrackEvents = make(map[string]string, len(trackEvents))

		if len(adData.ImpTks) > 0 {
			for k, v := range trackEvents {
				video.TrackEvents[k] = adData.ImpTks[0] + "&track_event=" + v
			}
		}
		adData.Video = &video
	}

	// 根据ctx的Output选择输出模板
	tpl := ctx.PageadTpl()

	pageadObj.Manifest = ctx.StaticBaseUrl + "manifest.appcache"

	tplData := map[string]interface{}{
		"manifestUrl": ctx.StaticBaseUrl + "manifest.appcache",
		"appCss":      ctx.StaticBaseUrl + "css/app.css",
		"mraidJs":     ctx.StaticBaseUrl + "js/mraid.js",
		"appJs":       ctx.StaticBaseUrl + "js/app.js",
		"ctAdData":    adData,
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, tplData); err != nil {
		ctx.L.Println("[pagead] banner tpl execute error: ", err)
		return nil
	}

	if ctx.Output == "html" {
		pageadObj.HtmlBody = buf.Bytes()
	}

	pageadObj.HtmlTag = buf.String()

	return &rc
}

// NOTE pagead adapter
func (raw *RawAdObj) HasMatchedAdapter(ctx *http_context.Context) bool {
	if len(raw.MatchedAdpaters(ctx)) > 0 {
		return true
	}
	return false
}

func (raw *RawAdObj) MatchedAdpaters(ctx *http_context.Context) (apts []string) {
	// icon, title, desc
	if raw.AppDownload.Title == "" || raw.AppDownload.Desc == "" ||
		raw.getMatchedIcon(ctx.Lang) == nil {
		return
	}

	apts = make([]string, 0, 3)
	for _, apt := range ctx.Adapters {
		if raw.matchedAdapter(apt, ctx.Lang, ctx.ImgRule) {
			apts = append(apts, apt)
		}
	}
	return
}

// 模板适配器
func (raw *RawAdObj) matchedAdapter(adapter, lang string, rule int) bool {
	adapters := strings.Split(adapter, "+")
	for _, apt := range adapters {
		var cat, w, h int
		if _, err := fmt.Sscanf(apt, "%c_%dx%d", &cat, &w, &h); err != nil {
			return false
		}

		if w == 0 && h == 0 {
			continue
		}

		if cat == 'i' && raw.getMatchedCreative(lang, w, h, rule) == nil {
			return false
		} else if cat == 'v' && raw.getMatchedVideo(lang, w, h) == nil {
			return false
		}
	}

	return true
}

func (raw *RawAdObj) choseAdapterCreative(adapter, lang string, rule int) {
	raw.IconChosen = raw.getMatchedIcon(lang)
	apts := strings.Split(adapter, "+")
	for _, apt := range apts {
		var cat, w, h int
		if _, err := fmt.Sscanf(apt, "%c_%dx%d", &cat, &w, &h); err != nil {
			continue
		}
		if w == 0 && h == 0 {
			continue
		}

		if cat == 'i' {
			raw.ImageChosen = raw.getMatchedCreative(lang, w, h, rule)
		} else if cat == 'v' {
			raw.VideoChosen = raw.getMatchedVideo(lang, w, h)
		}
	}
}
