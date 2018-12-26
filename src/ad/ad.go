package ad

import (
	"strings"

	"http_context"
)

type ImgObj struct {
	Id  string `json:"id"`
	Url string `json:"img_url"`
	W   int    `json:"imgw"`
	H   int    `json:"imgh"`
}

type VideoObj struct {
	Id  string `json:"id"`
	Url string `json:"video_url"`
	W   int    `json:"videow"`
	H   int    `json:"videoh"`
}

type PreCreativeObj struct {
	CreativeType int      `json:"creative_type"` // -1：表示没有此创意节点 0：图片 1：H5 (默认-1)
	Img          ImgObj   `json:"img"`
	Html         string   `json:"html"`           // h5模板拼好的字符串，Base64编码
	PreImpTkUrl  []string `json:"pre_imp_tk_url"` // 前置曝光监测链接数组
	PreClkTkUrl  []string `json:"pre_clk_tk_url"` // 前置点击监测链接数组
}

type BakCreativeObj struct {
	CreativeType int      `json:"creative_type"` // 0: 图片 1: H5 2: 视频
	Img          ImgObj   `json:"img"`
	Html         string   `json:"html"` // h5模板拼好的字符串，Base64编码
	Video        VideoObj `json:"video"`
	BakImpTkUrl  []string `json:"bak_imp_tk_url"` // 后置曝光监测链接数组
	BakClkTkUrl  []string `json:"bak_clk_tk_url"` // 后置点击监测链接数组
}

type DeepLinkObj struct {
	DlSuccTkUrl string `json:"dlsucc_tk_url"` // 成功跳转
	DlFailTkUrl string `json:"dlfail_tk_url"` // 失败跳转
}

type AppDownloadObj struct {
	Title    string   `json:"title"`
	Desc     string   `json:"description"`
	PkgName  string   `json:"app_pkg_name"`
	Size     string   `json:"size"`       // format: 42KB
	Rate     float32  `json:"rate"`       // 应用评分
	Download string   `json:"download"`   // 下载数
	Review   int      `json:"review"`     // 评论数
	IconUrl  []string `json:"icon_url"`   // 图标数组，最后拼到模板中
	ItlTKUrl []string `json:"itl_tk_url"` // 安装监测数组
	ActTkUrl []string `json:"act_tk_url"` // 激活监测数组
}

// LandingType Macro
const (
	APP_DOWNLOAD   = 0
	EXTERN_LANDING = iota
	INNER_LANDING  = iota
	RSS            = iota
	DEEP_LINK      = iota
)

type VastObj struct {
	OfferId       string   `json:"id"` // offerid
	Impressions   []string `json:"impressions"`
	ClickTracks   []string `json:"click_trackings"`
	ClickThroughs []string `json:"click_throughs"`
	CustomClicks  []string `json:"custom_clicks"`
}

type AdObj struct {
	Id          string         `json:"adid"`    // offer id
	ImpId       string         `json:"impid"`   // uuid
	Channel     string         `json:"channel"` // channel
	Slot        string         `json:"slot"`
	Ck          string         `json:"ck,omitempty"` // cookie for jstag
	LandingType int            `json:"landing_type"`
	AppDownload AppDownloadObj `json:"app_download"`
	DeepLink    DeepLinkObj    `json:"deeplink"`
	AdExp       int            `json:"ad_expire_time"` // SDK缓存广告过期时间，默认为1000秒
	TaoBaoKe    string         `json:"tbk"`            // 淘宝客口令
	TaoBaoKeT   int            `json:"tbk_t"`          // 淘口令写入时间节点, 1:不写入, 2:广告被展示时, 3:广告被点击时
	PreClick    int            `json:"pre_click"`      // 1: on, 2: off
	ClkUrl      string         `json:"clk_url"`        // 点击链接
	FinalUrl    string         `json:"final_url"`      // 最终链接，如果为空字符串，则客户端不作处理
	UrlSchema   string         `json:"url_schema"`     // 已安装app可直接打开
	IconChosen  string         `json:"-"`              // 选中的icon

	VastUrl     string  `json:"vast_url"` // 视频广告的vast链接
	VastWrapObj VastObj `json:"vast_wrap_obj"`
	VastXmlData string  `json:"vast_xml_data"` // vast XML数据
	PlayNum     int     `json:"play_num"`      // 视频默认播放次数

	PreCreative PreCreativeObj `json:"pre_creative"` // 前置创意对象
	BakCreative BakCreativeObj `json:"bak_creative"` // 后置创意对象

	ClkTks []map[string]interface{} `json:"clk_tks"` // 异步跳转302链接
}

func (ad *AdObj) SetPreClick(b bool) {
	if b {
		ad.PreClick = 1
	} else {
		ad.PreClick = 2
	}
}

func (ad *AdObj) SetTrackUrl(url string) {
	ad.ClkUrl = url
}

func (ad *AdObj) SetTks(ctx *http_context.Context, attachedArgs []string, thirdImpTk string) {
	args := make([]string, 0, 4)

	if ctx.AdType == "3" {
		args = append(args, "slot=anli_"+ctx.SlotId)
	} else if ctx.AdType == "9" {
		args = append(args, "slot=anli_"+ctx.SlotId)
	} else {
		args = append(args, "slot="+ctx.SlotId)
	}
	args = append(args, "adtype="+ctx.AdType)
	args = append(args, "offer="+ad.Id)
	args = append(args, "imp="+ad.ImpId)
	args = append(args, "channel="+ad.Channel)
	args = append(args, "server_id="+ctx.ServerId)
	args = append(args, attachedArgs...)

	appendStr := ctx.PossiableArgs + "&" + strings.Join(args, "&")

	if ad.PreCreative.CreativeType != -1 {
		imp := make([]string, len(ctx.PreImpTks))
		for i := 0; i != len(ctx.PreImpTks); i++ {
			imp[i] = ctx.PreImpTks[i] + "?" + appendStr
		}
		ad.PreCreative.PreImpTkUrl = imp

		clk := make([]string, len(ctx.PreClkTks))
		for i := 0; i != len(clk); i++ {
			clk[i] = ctx.PreClkTks[i] + "?" + appendStr
		}
		ad.PreCreative.PreClkTkUrl = clk
	}

	imp := make([]string, len(ctx.PostImpTks))
	for i := 0; i != len(ctx.PostImpTks); i++ {
		if strings.Contains(ctx.PostImpTks[i], "?") {
			imp[i] = ctx.PostImpTks[i] + "&" + appendStr
		} else {
			imp[i] = ctx.PostImpTks[i] + "?" + appendStr
		}
	}
	if len(thirdImpTk) > 0 {
		imp = append(imp, thirdImpTk)
	}
	ad.BakCreative.BakImpTkUrl = imp

	clk := make([]string, len(ctx.PostClkTks))
	for i := 0; i != len(ctx.PostClkTks); i++ {
		if strings.Contains(ctx.PostClkTks[i], "?") {
			clk[i] = ctx.PostClkTks[i] + "&" + appendStr
		} else {
			clk[i] = ctx.PostClkTks[i] + "?" + appendStr
		}
	}
	ad.BakCreative.BakClkTkUrl = clk
}

type NativeAdObjCore struct {
	Icon           string  `json:"icon"`
	Title          string  `json:"title"`
	Image          string  `json:"image"`
	Desc           string  `json:"desc"`
	Button         string  `json:"button"`
	Star           float32 `json:"star"`
	ChoicesLinkUrl string  `json:"choices_link_url"`
	OfferType      int     `json:"offer_type"` // 1：表示下载类，2：表示非下载类
}

type NativeAdObj struct {
	Core         NativeAdObjCore          `json:"native_adobj"`
	Id           string                   `json:"adid"`
	Country      string                   `json:"country"`
	Channel      string                   `json:"channel"`
	ImpId        string                   `json:"impid"`
	LandingType  int                      `json:"landing_type"`
	AdExp        int                      `json:"ad_expire_time"`
	TaoBaoKe     string                   `json:"tbk"`   // 淘宝客口令
	TaoBaoKeT    int                      `json:"tbk_t"` // 淘口令写入时间节点, 1:不写入, 2:广告被展示时, 3:广告被点击时
	PreClick     int                      `json:"pre_click"`
	ClkUrl       string                   `json:"clk_url"`
	FinalUrl     string                   `json:"final_url"`
	UrlSchema    string                   `json:"url_schema"` // 已安装app可直接打开
	ClkTks       []map[string]interface{} `json:"clk_tks"`
	ImpTkUrl     []string                 `json:"imp_tk_url"`
	ClkTkUrl     []string                 `json:"clk_tk_url"`
	RequestViaUa int                      `json:"request_via_ua"`
	AppWallCat   string                   `json:"app_wall_cat,omitempty"`

	PkgName string `json:"-"`
	UniqId  string `json:"-"`
}

type LifeAdObjCore struct {
	NativeAdObjCore
	Order int `json:"order"`
}

type LifeAdObj struct {
	Core         LifeAdObjCore `json:"service_adobj"`
	Id           string        `json:"adid"`
	Country      string        `json:"country"`
	Channel      string        `json:"channel"`
	ImpId        string        `json:"impid"`
	LandingType  int           `json:"landing_type"`
	AdExp        int           `json:"ad_expire_time"`
	PreClick     int           `json:"pre_click"`
	ClkUrl       string        `json:"clk_url"`
	FinalUrl     string        `json:"final_url"`
	UrlSchema    string        `json:"url_schema"` // 已安装app可直接打开
	ImpTkUrl     []string      `json:"imp_tk_url"`
	ClkTkUrl     []string      `json:"clk_tk_url"`
	RequestViaUa int           `json:"request_via_ua"`

	PkgName string `json:"-"`
	UniqId  string `json:"-"`
}

func (natAd *NativeAdObj) SetPreClick(b bool) {
	if b {
		natAd.PreClick = 1
	} else {
		natAd.PreClick = 2
	}
}

type RealtimeAdObj struct {
	Id          string   `json:"-"`
	ImpId       string   `json:"-"`
	LandingType int      `json:"landing_type"` // 0：应用下载 1：外开落地页 2：内开落地页 3：订阅
	Title       string   `json:"title"`
	Desc        string   `json:"desc"`
	PkgName     string   `json:"pkg_name"`
	Icon        string   `json:"icon"`
	IconSize    string   `json:"icon_size"` // "fmt: 1x1"
	Button      string   `json:"button"`
	Image       string   `json:"image"`
	ImageSize   string   `json:"image_size"` // "fmt:300x250"
	ClkUrl      string   `json:"clk_url"`
	ImpTks      []string `json:"imp_tks"`
	ClkTks      []string `json:"clk_tks"`
}

type VideoAdObj struct {
	RewardedVideo RewardedVideoObj `json:"rewarded_video_adobj"`
	Id            string           `json:"adid"`
	ImpId         string           `json:"impid"`
	Slot          string           `json:"slot_id"`
	Channel       string           `json:"channel"`
	Country       string           `json:"country"`
	LandingType   int              `json:"landing_type"`
	Star          float32          `json:"star"`
	Button        string           `json:"button"`
	RateNum       int              `json:"rate_num"`
	AdExp         int              `json:"ad_expire_time"`
	ClkUrl        string           `json:"clk_url"`
	FinalUrl      string           `json:"final_url"`
	UrlSchema     string           `json:"url_schema"` // 已安装app可直接打开
	VastXmlData   string           `json:"vast_xml_data"`
	TaoBaoKe      string           `json:"tbk"`   // 淘宝客口令
	TaoBaoKeT     int              `json:"tbk_t"` // 淘口令写入时间节点, 1:不写入, 2:广告被展示时, 3:广告被点击时

	Title  string   `json:"-"`
	Desc   string   `json:"-"`
	Icon   string   `json:"-"`
	ImpTks []string `json:"-"`
	ClkTks []string `json:"-"`

	Ext interface{} `json:"-"` // 该字段用于灵活处理一些第三方广告的附加信息
}

type RewardedVideoObj struct {
	PlayLocal   int      `json:"play_local"`
	ClickTime   int      `json:"click_time,omitempty"`
	LoadTime    int      `json:"load_time,omitempty"`
	ButtonColor string   `json:"button_color,omitempty"`
	H5Opt       string   `json:"h5_opt,omitempty"`
	Img         ImgObj   `json:"img"`
	Video       VideoObj `json:"video"`
}

type NativeVideoV4Ad struct {
	Core        NativeAdObjCore `json:"native_adobj"`
	Id          string          `json:"adid"`
	ImpId       string          `json:"impid"`
	LandingType int             `json:"landing_type"`
	AdExp       int             `json:"ad_expire_time"`
	Slot        string          `json:"slot_id"`
	Channel     string          `json:"channel"`
	Country     string          `json:"country"`
	TaoBaoKe    string          `json:"tbk"`
	LoadTime    int             `json:"load_time"`
	ClickTime   int             `json:"click_time"`
	TaoBaoKeT   int             `json:"tbk_t"`
	PreClick    int             `json:"pre_click"`
	ClkUrl      string          `json:"clk_url"`
	FinalUrl    string          `json:"final_url"`
	UrlSchema   string          `json:"url_schema"` // 已安装app可直接打开
	VastXmlData string          `json:"vast_xml_data"`
	Lp          int             `json:"lp"`     //1：点击特定区域跳转原生定制落地页（默认）；2：不跳转
	Remind      string          `json:"remind"` // 判定当前为非wifi环境时进行提示“当前观看提示语”和继续播放按钮
	ImpTks      []string        `json:"imp_tks"`
	ClkTks      []string        `json:"clk_tks"`

	// 调试时方便观察
	Video VideoObj `json:"d_video"`
	Image ImgObj   `json:"d_image"`
}

// 插屏广告
type InterstitialAd struct {
	Id          string                   `json:"adid"`    // offer id
	ImpId       string                   `json:"impid"`   // uuid
	Channel     string                   `json:"channel"` // channel
	Slot        string                   `json:"slot"`
	Country     string                   `json:"country"`
	LandingType int                      `json:"landing_type"`
	AppDownload AppDownloadObj           `json:"app_download"`
	AdExp       int                      `json:"ad_expire_time"` // SDK缓存广告过期时间，默认为1000秒
	TaoBaoKe    string                   `json:"tbk"`            // 淘宝客口令
	TaoBaoKeT   int                      `json:"tbk_t"`          // 淘口令写入时间节点, 1:不写入, 2:广告被展示时, 3:广告被点击时
	CountDown   int                      `json:"count_down"`     // 倒计时秒数，倒数动画结束后显示关闭按钮 默认5
	PreClick    int                      `json:"pre_click"`      // 1: on, 2: off
	ClkUrl      string                   `json:"clk_url"`        // 点击链接
	FinalUrl    string                   `json:"final_url"`      // 最终链接，如果为空字符串，则客户端不作处理
	UrlSchema   string                   `json:"url_schema"`     // 已安装app可直接打开
	ClkTks      []map[string]interface{} `json:"clk_tks"`        // 异步跳转302链接
	PreCreative PreCreativeObj           `json:"pre_creative"`   // 前置创意对象
	BakCreative BakCreativeObj           `json:"bak_creative"`   // 后置创意对象
}

// 监测链接
func (ad *InterstitialAd) SetTks(ctx *http_context.Context, attachedArgs []string, thirdImpTk string) {
	args := make([]string, 0, 8)
	args = append(args, "slot="+ctx.SlotId)
	args = append(args, "adtype="+ctx.AdType)
	args = append(args, "offer="+ad.Id)
	args = append(args, "imp="+ad.ImpId)
	args = append(args, "channel="+ad.Channel)
	args = append(args, "server_id="+ctx.ServerId)
	args = append(args, attachedArgs...)

	appendStr := ctx.PossiableArgs + "&" + strings.Join(args, "&")

	imp := make([]string, len(ctx.PostImpTks))
	for i := 0; i != len(ctx.PostImpTks); i++ {
		if strings.Contains(ctx.PostImpTks[i], "?") {
			imp[i] = ctx.PostImpTks[i] + "&" + appendStr
		} else {
			imp[i] = ctx.PostImpTks[i] + "?" + appendStr
		}
	}
	if len(thirdImpTk) > 0 {
		imp = append(imp, thirdImpTk)
	}
	ad.BakCreative.BakImpTkUrl = imp

	clk := make([]string, len(ctx.PostClkTks))
	for i := 0; i != len(ctx.PostClkTks); i++ {
		if strings.Contains(ctx.PostClkTks[i], "?") {
			clk[i] = ctx.PostClkTks[i] + "&" + appendStr
		} else {
			clk[i] = ctx.PostClkTks[i] + "?" + appendStr
		}
	}
	ad.BakCreative.BakClkTkUrl = clk
}

// pagead 后定义新的广告对象
type AdObjV2 struct {
	CommonObj *Common `json:"common_obj"`
	PageadObj *Pagead `json:"pagead_obj"`
}

type Common struct {
	Id       string   `json:"adid"`
	ImpId    string   `json:"impid"`
	Slot     string   `json:"slot_id"`
	Channel  string   `json:"channel"`
	Country  string   `json:"country"`
	ClkUrl   string   `json:"clk_url"`
	FinalUrl string   `json:"final_url"`
	ImpTks   []string `json:"imp_tks"`
	ClkTks   []string `json:"clk_tks"`
}

type Pagead struct {
	Manifest string `json:"manifest"` // 有值表示替换html标签属性
	HtmlTag  string `json:"html_tag"`
	Interval int    `json:"interval"` // 下次请求间隔： 0表示不请求，单位s
	HtmlBody []byte `json:"-"`        // 用于output=html时直接返回html
}

type JsMediaAdObj struct {
	Title    string `json:"title"`
	Desc     string `json:"desc"`
	Image    string `json:"image"`
	Icon     string `json:"icon"`
	Button   string `json:"button"`
	ClkUrl   string `json:"clkUrl"`
	ImpTrack string `json:"impTrack"`
	ClkTrack string `json:"clkTrack"`
}
