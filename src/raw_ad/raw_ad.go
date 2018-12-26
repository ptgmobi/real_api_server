package raw_ad

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	dnf "github.com/brg-liuwei/godnf"

	"ct_bloom"
	"http_context"
	"util"
)

var getPkgReg *regexp.Regexp
var pkgNameReg *regexp.Regexp
var delPkgNamePostfix *regexp.Regexp
var chReg *regexp.Regexp

const (
	APP_DOWNLOAD   = 0
	EXTERN_LANDING = iota
	INNER_LANDING  = iota
	RSS            = iota
	DEEP_LINK      = iota
)

func init() {
	getPkgReg = regexp.MustCompile("(.+)\\(.+\\)")
	pkgNameReg = regexp.MustCompile(`^[\w\d\.]+`)
	delPkgNamePostfix = regexp.MustCompile(`\(\w*\)`) // ab.com(wg3) ==> ab.com
	chReg = regexp.MustCompile("channel=([^&]+)")
}

func getPkgName(pkgName string) string {
	rc := pkgNameReg.FindStringSubmatch(pkgName)
	if len(rc) == 1 {
		return rc[0]
	}
	return ""
}

func getPkg(pkg string) string {
	rc := getPkgReg.FindStringSubmatch(pkg)
	if len(rc) >= 1 {
		return rc[1]
	}
	return ""
}

const (
	// 具体解释见: https://github.com/cloudadrd/Document/blob/master/ssp_fe_2_adserver_interface_definition.md#ad_network%E6%8B%89%E5%8F%96%E6%8E%A5%E5%8F%A3
	CT_NOCREATIVE     = "1" // 无素材广告
	CT_CREATIVE       = "2" // 图片广告
	CT_VIDEO          = "3" // 视频广告
	CT_REWARDED_VIDEO = "4" // 激励视频
	CT_SENSITIVITY    = "5" // 敏感渠道
	CT_CPS            = "6" // cps
	CT_BANNER         = "7" // pagead Banner广告
	CT_INTERSTITIAL   = "8" // pagead 插屏广告
)

type Img struct {
	Id          string `json:"creative_id"`
	OldId       string `json:"old_id"` // 兼容旧系统的cid
	Size        int64  `json:"size"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Url         string `json:"url"`
	DomesticCDN string `json:"domestic_cdn"`
	OverseasCDN string `json:"overseas_cdn"`
	Lang        string `json:"language"`
}

func (img *Img) ToString() string {
	return fmt.Sprintf("[(%d x %d) %s (%s)]", img.Width, img.Height, img.Url, img.Lang)
}

func (img *Img) Match(w, h int) bool {
	return img.Width == w && img.Height == h
}

func (img *Img) RatioFuzzyMatch(w, h int, ratioBias float64) bool {
	if h == 0 {
		w, h = w+1, h+1
	}
	argRatio, imgRatio := float64(w)/float64(h), float64(img.Width)/float64(img.Height)
	return math.Abs(argRatio-imgRatio) <= ratioBias
}

func (img *Img) RatioMatch(w, h int) bool {
	return img.RatioFuzzyMatch(w, h, 0.001)
}

func (img *Img) AbsFuzzyMatch(w, h int, bias float64) bool {
	argRatio := util.LaplaceSmooth(w, h)
	imgRatio := util.LaplaceSmooth(img.Width, img.Height)
	if math.Abs(argRatio-imgRatio) > 0.001 {
		// if argRatio != imgRatio then return false end
		return false
	}
	upperBias, lowerBias := 1+bias, 1-bias
	wUpper, wLower := int(float64(img.Width)*upperBias), int(float64(img.Width)*lowerBias)
	hUpper, hLower := int(float64(img.Height)*upperBias), int(float64(img.Height)*lowerBias)
	return w >= wLower && w <= wUpper && h >= hLower && h <= hUpper
}

func (img *Img) FuzzyMatch(w, h int, ratioBias, absBias float64) bool {
	if h == 0 {
		w, h = w+1, h+1
	}
	argRatio, imgRatio := float64(w)/float64(h), float64(img.Width)/float64(img.Height)
	if math.Abs(argRatio-imgRatio) > ratioBias {
		return false
	}
	upperBias, lowerBias := 1+absBias, 1-absBias
	wUpper, wLower := int(float64(img.Width)*upperBias), int(float64(img.Width)*lowerBias)
	hUpper, hLower := int(float64(img.Height)*upperBias), int(float64(img.Height)*lowerBias)
	return w >= wLower && w <= wUpper && h >= hLower && h <= hUpper
}

type AppDownloadObj struct {
	Title     string  `json:"title"`
	TitleLC   string  `json:"title_lower"` // TitleLowerCase
	Desc      string  `json:"description"`
	DescLC    string  `json:"description_lower"` // DescLowerCase
	PkgName   string  `json:"app_pkg_name"`
	Size      string  `json:"size"`     // format: "42 KB"
	Rate      float32 `json:"rate"`     // 应用评分
	Download  string  `json:"download"` // 下载数
	Review    int     `json:"review"`   // 评论数
	TrackLink string  `json:"tracking_link"`
	BundleId  string  `json:"bundle_id"` // iOS
	FileSize  int64   `json:"file_size"` // App字节数
}

type Subscription struct {
	TrackLink string   `json:"tracking_link"`
	Js        []string `json:"js"`
	Carrier   string   `json:"carrier"`
}

type Target struct {
	key    string
	belong bool
	vals   []string
}

type Video struct {
	Id          string `json:"id"`
	OldId       string `json:"old_id"` // 兼容旧系统的cid
	Size        int64  `json:"size"`
	Ratio       string `json:"ratio"`
	W           int    `json:"width"`
	H           int    `json:"height"`
	Url         string `json:"url"`
	DomesticCDN string `json:"domestic_cdn"`
	OverseasCDN string `json:"overseas_cdn"`
	Type        string `json:"type"` // mp4
	Lang        string `json:"language"`
}

type RawAdObj struct {
	Id         string  `json:"adid"` // offer id
	Channel    string  `json:"channel"`
	UniqId     string  `json:"uniq_id"` // uniq_id
	HashSign   string  `json:"hash_sign"`
	Payout     float32 `json:"payout"`      // currency: USD
	PayoutType string  `json:"payout_type"` // CPI/CPS/CPL/CPA
	CapMonthly int     `json:"cap_monthly"`
	CapDaily   int     `json:"cap_daily"`
	AdExp      int     `json:"ad_expire_time"` // SDK cached time, default: 1Ks

	Spon   string `json:"spon"`    // 广告主
	AttPro int    `json:"att_pro"` // 监测商

	ClkUrl   string `json:"clk_url"`        // 点击链接
	FinalUrl string `json:"final_url"`      // 最终链接
	ClkCb    string `json:"click_callback"` // 异步点击上报链接

	Platform  string   `json:"platform"`
	Countries []string `json:"countries"`
	Regions   []string `json:"regions"`
	Cities    []string `json:"cities"`

	SuppDevices []string `json:"support_devices"` // 支持设备类型, iOS: phone, ipad; Android: phone, tablet

	Versions []string `json:"versions"` // 支持版本

	ClkTks []map[string]interface{} `json:"clk_tks"` // 异步跳转监测链接

	ThirdPartyClkTks []string `json:"third_party_clk_tks"`
	ThirdPartyImpTks []string `json:"third_party_imp_tks"`

	LandingType     int      `json:"landing_type"`
	ProductCategory string   `json:"product_category"` // "AppDownload: [GP,Itune,DDL],  Content: [Other]"
	AppCategory     []string `json:"app_category"`

	AppWallCat string `json:"-"` // top, feature, tool, game

	AppDownload AppDownloadObj `json:"app_download"`

	Subscription Subscription `json:"subscription"`

	Creatives map[string][]Img `json:"creatives"` // 所有创意，Lang为索引，最合适的将拼到模板中

	CreativeChosen *Img `json:"-"` // 被选中用于替换的创意

	ImageChosen *Img   `json:"-"` // 视频广告选中图片
	VideoChosen *Video `json:"-"` // 视频广告选中视频

	Videos  map[string][]Video `json:"videos"`
	VastUrl string             `json:"vast_url"`

	Icons      map[string][]Img `json:"icons"` // 所有图标，Lang为索引，最合适的将拼到模板中
	IconChosen *Img             `json:"-"`     // 被选中用于替换的图标

	RenderImgs map[string]string `json:"render_imgs"` // 打底图片

	Network int `json:"network"` // offer要求的网络环境, 1: wifi, 2: unwifi, 3: don't care (default)

	ScreenShots []Img `json:"-"`

	Targets    []Target `json:"-"` // dnf helper
	Dnf        string   `json:"-"` // dnf description
	AttachArgs []string `json:"-"` // args attached in monitor urls

	PreRate     float64 `json:"pre_rate"`     // preclick percent[0,1]
	TrafficRate float64 `json:"traffic_rate"` // traffic percent[0,1]

	TempBlack int  `json:"temp_black"` // 1:temp black, 2: permanent black
	BlackPkg  bool `json:"black_pkg"`

	ChannelType     map[string]struct{} `json:"channel_type"`
	OfferTargetSlot map[string]struct{} `json:"offer_target_slot"` // offer只投某些slot
	Carrier         map[string]struct{} `json:"carrier"`           // 定向的运营商
	ReplaceSlotId   bool                `json:"replace_slot_id"`
	DeviceCode      map[string]struct{} `json:"device_code"` // 设备的标识：iPhone6,1

	ChannelPmtEnable     int `json:"channel_pmt_enable"`      // pmt enable 1:enable,2:disEnable
	ChannelSemiPmtEnable int `json:"channel_semi_pmt_enable"` // semi pmt enable 1: enable, 2: disEnable
	PkgPmtEnable         int `json:"pkg_pmt_enable"`          // pkg的pmt后劫控制 1:enable, 2: disenable
	PkgSemiPmtEnable     int `json:"pkg_semi_pmt_enable"`     // pkg的semi后劫控制 1:enable, 2:disenable
	PkgPreClickEnable    int `json:"pkg_pre_click_enable"`    // pkg pre click 1:enable, 2: disenable
	OfferPmtEnable       int `json:"offer_pmt_enable"`        // offer的后劫控制1:enable, 2:disenable
	OfferSemiPmtEnable   int `json:"offer_semi_pmt_enable"`   // offer的semi后劫控制1:enable, 2:disenable

	CtBloomIndex []uint `json:"-"`

	FuyuEnabled  bool `json:"fuyu_enabled"` // server side wugan click control
	JsTagEnabled bool `json:"jstag_enabled"`
	WuganEnabled bool `json:"wugan_enabled"`

	Pacing int `json:"cp"` // cp means click_pacing

	UpdateTime string `json:"update_time"` // offer更新时间

	PmtClickCount int `json:"pmt_click_count"` // pmt 额外点击数

	lastUrl string

	TaoBaoKe  string `json:"tbk"`   // 淘宝客口令
	TaoBaoKeT int    `json:"tbk_t"` // 淘口令写入时间节点, 1:不写入, 2:广告被展示时, 3:广告被点击时

	UrlSchema string `json:"url_schema"`

	ContentType int  `json:"content_type"` // 1：下载类app， 2：非下载类app
	IsT         bool `json:"t"`
}

func NewRawAdObj() *RawAdObj {
	return &RawAdObj{
		PayoutType:       "CPI", // default payout type
		Icons:            make(map[string][]Img),
		Creatives:        make(map[string][]Img),
		Videos:           make(map[string][]Video),
		Countries:        make([]string, 0, 1),
		Targets:          make([]Target, 0, 4),
		AppCategory:      make([]string, 0, 1),
		ClkTks:           []map[string]interface{}{},
		ThirdPartyClkTks: make([]string, 0, 1),
		ThirdPartyImpTks: make([]string, 0, 1),
		AttachArgs:       make([]string, 0, 1),
	}
}

func (raw *RawAdObj) SetLastUrl(url string) {
	raw.lastUrl = url
}

func (raw *RawAdObj) GetChannelType() []int {
	if len(raw.ChannelType) == 0 {
		return nil
	}

	var chType []int
	for t, _ := range raw.ChannelType {
		tInt, err := strconv.Atoi(t)
		if err != nil {
			log.Println("GetChannelType channel type atoi err: ", err, " type: ", t)
			continue
		}
		chType = append(chType, tInt)
	}
	return chType
}

// XXX 设备类型定向已经开发，但是可能影响收入暂时没有调用，
// 如果产品真的确定有这个需求再测试调用，offer update已经拉取了信息
func (raw *RawAdObj) IsHitDevice(d string) bool {
	if len(raw.DeviceCode) > 0 {
		_, ok := raw.DeviceCode[d]
		return ok
	}
	return true

}

func (raw *RawAdObj) IsHitCarrier(c string) bool {
	if len(raw.Carrier) > 0 {
		_, ok := raw.Carrier[c]
		return ok
	}
	return true
}

func (raw *RawAdObj) IsHitChannelType(t string) bool {
	if len(raw.ChannelType) > 0 {
		_, ok := raw.ChannelType[t]
		return ok
	}
	return false
}

func (raw *RawAdObj) IsHitSlot(slotid string) bool {
	if len(raw.OfferTargetSlot) > 0 {
		if _, ok := raw.OfferTargetSlot[slotid]; !ok {
			// 有slot定向，但是没有命中
			return false
		}
	}
	return true
}

func (raw *RawAdObj) ToMap() (m map[string]interface{}) {
	b, _ := json.Marshal(raw)
	json.Unmarshal(b, &m)
	m["dnf"] = raw.Dnf
	m["fuyu"] = raw.FuyuEnabled
	m["jstag"] = raw.JsTagEnabled
	m["wugan"] = raw.WuganEnabled
	m["pre_rate"] = raw.PreRate
	m["traffic_rate"] = raw.TrafficRate
	return
}

func (raw *RawAdObj) ToString() string {
	return fmt.Sprintf("<ad_id:%s>", raw.Id)
}

func getRandImg(m map[string][]Img) *Img {
	if len(m) == 0 {
		return nil
	}
	n := rand.Int() % len(m)
	i := 0
	for _, imgs := range m {
		if i == n {
			if len(imgs) == 0 {
				return nil
			}
			sel := rand.Int() % len(imgs)
			return &imgs[sel]
		}
		i++
	}
	panic("un-accessable code")
}

func GetRandMatchedImg(m map[string][]Img, lang string, w, h, rule int) *Img {
	imgs, ok := m[lang]
	if !ok {
		return nil
	}
	n := len(imgs)
	if n <= 0 {
		return nil
	}
	r := rand.Int() % n
	for i := 0; i != n; i++ {
		idx := (i + r) % n
		img := &imgs[idx]
		if w == 0 || h == 0 {
			return img
		}
		switch rule {
		case -2: // 比值浮动不超过20%即可
			if img.RatioFuzzyMatch(w, h, 0.2) {
				return img
			}
		case -1: // 比值完全匹配即可
			if img.RatioMatch(w, h) {
				return img
			}
		case 1: // 绝对值完全匹配
			if img.Match(w, h) {
				return img
			}
		case 2: // 比值完全匹配，宽高浮动不超过20%
			if img.AbsFuzzyMatch(w, h, 0.2) {
				return img
			}
		case 3: // 比值浮动不超过20%，宽高浮动不超过20%
			if img.FuzzyMatch(w, h, 0.2, 0.2) {
				return img
			}
		}
	}
	return nil
}

// 在match的基础上，返回最大的图片
func GetMaxMatchedImg(m map[string][]Img, lang string, w, h, rule int, isRand bool) *Img {
	imgs, ok := m[lang]
	if !ok {
		return nil
	}

	maxWH := -1
	var rc *Img
	toUpdate := func(img *Img, curMax *int) bool {
		mul := img.Width * img.Height
		if isRand && mul == *curMax {
			// rand select same image
			return rand.Int()%2 == 0
		}
		if mul > *curMax {
			*curMax = mul
			return true
		}
		return false
	}

	for i := 0; i != len(imgs); i++ {
		img := &imgs[i]
		if w == 0 || h == 0 {
			if toUpdate(img, &maxWH) {
				rc = img
				continue
			}
		}

		switch rule {
		case -2: // 比值浮动不超过20%即可
			if img.RatioFuzzyMatch(w, h, 0.2) {
				if toUpdate(img, &maxWH) {
					rc = img
				}
			}
		case -1: // 比值完全匹配即可
			if img.RatioMatch(w, h) {
				if toUpdate(img, &maxWH) {
					rc = img
				}
			}
		case 1: // 绝对值完全匹配
			if img.Match(w, h) {
				if toUpdate(img, &maxWH) {
					rc = img
				}
			}
		case 2: // 比值完全匹配，宽高浮动不超过20%
			if img.AbsFuzzyMatch(w, h, 0.2) {
				if toUpdate(img, &maxWH) {
					rc = img
				}
			}
		case 3: // 比值浮动不超过20%，宽高浮动不超过20%
			if img.FuzzyMatch(w, h, 0.2, 0.2) {
				if toUpdate(img, &maxWH) {
					rc = img
				}
			}
		}
	}
	return rc
}

func (raw *RawAdObj) CreativeId() string {
	if raw.CreativeChosen != nil {
		return raw.CreativeChosen.Id
	}
	return ""
}

func (raw *RawAdObj) VideoIds(ctx *http_context.Context) (ids []string) {
	ids = make([]string, 0, 2)
	videos := raw.GetVideos(ctx)
	for _, video := range videos {
		ids = append(ids, video.Id)
	}
	return
}

func (raw *RawAdObj) HasVideo(ctx *http_context.Context) bool {
	return raw.getVideoByCids(ctx.CidMap) != nil
}

func (raw *RawAdObj) GetImgs(ctx *http_context.Context) []Img {
	if imgs := raw.Creatives[ctx.Lang]; len(imgs) != 0 {
		return imgs
	} else {
		return raw.Creatives["ALL"]
	}
}

func (raw *RawAdObj) GetVideos(ctx *http_context.Context) []Video {
	if videos := raw.Videos[ctx.Lang]; len(videos) != 0 {
		return videos
	} else {
		return raw.Videos["ALL"]
	}
}

// 根据横竖屏匹配最大的图片
func (raw *RawAdObj) VideoGetMacthedImg(ctx *http_context.Context) *Img {
	// 竖屏 1 : 1
	if ctx.VideoScreenType == 2 {
		if img := GetMaxMatchedImg(raw.Creatives, ctx.Lang, 100, 100, -2, false); img != nil {
			return img
		} else {
			return GetMaxMatchedImg(raw.Creatives, "ALL", 100, 100, -2, false)
		}
	} else { // 横屏1.9 : 1
		if img := GetMaxMatchedImg(raw.Creatives, ctx.Lang, 190, 100, -2, false); img != nil {
			return img
		} else {
			return GetMaxMatchedImg(raw.Creatives, "ALL", 190, 100, -2, false)
		}
	}
}

// 视频选择素材
func (raw *RawAdObj) VideoChoseCreatives(ctx *http_context.Context) bool {
	if raw.ImageChosen == nil {
		if img := raw.getImageByCids(ctx.CidMap); img != nil {
			raw.ImageChosen = img
		} else if img := raw.VideoGetMacthedImg(ctx); img != nil {
			raw.ImageChosen = img
		}
	}

	if raw.VideoChosen == nil {
		if video := raw.getVideoByCids(ctx.CidMap); video != nil {
			raw.VideoChosen = video
		} else {
			return false
		}
	}
	return raw.VideoChosen != nil && raw.ImageChosen != nil
}

// 根据视频id选择
func (raw *RawAdObj) ChoseVideoById(cid string) (ok bool) {
	videos := raw.Videos["ALL"]
	for _, v := range videos {
		if v.Id == cid || v.OldId == cid {
			raw.VideoChosen = &v
			return true
		}
	}
	return false
}

func (raw *RawAdObj) getImageByCids(cid map[string]int) *Img {
	if len(raw.Creatives) == 0 {
		return nil
	}
	imgs := raw.Creatives["ALL"]
	for _, img := range imgs {
		if _, ok := cid[img.Id]; ok {
			return &img
		} else if _, ok := cid[img.OldId]; ok {
			return &img
		}
	}
	return nil
}

func (raw *RawAdObj) getVideoByCids(cid map[string]int) *Video {
	if len(raw.Videos) == 0 {
		return nil
	}
	videos := raw.Videos["ALL"]
	for _, v := range videos {
		if _, ok := cid[v.Id]; ok {
			return &v
		} else if _, ok := cid[v.OldId]; ok {
			return &v
		}
	}
	return nil
}

func (raw *RawAdObj) getMatchedIcon(lang string) *Img {
	// icon: 比值match即可
	icon := GetRandMatchedImg(raw.Icons, lang, 1, 1, -1)
	if icon == nil && lang != "ALL" {
		icon = GetRandMatchedImg(raw.Icons, "ALL", 1, 1, -1)
	}
	if icon == nil {
		icon = getRandImg(raw.Icons)
	}
	return icon
}

func (raw *RawAdObj) TryGetMatchedVideo(ctx *http_context.Context) *Video {
	rule := ctx.VideoScreenType
	videos := raw.GetVideos(ctx)
	nVideos := len(videos)
	candidateIndex := -1
	for i := 0; i < nVideos; i++ {
		if videos[i].Url != "" {
			if rule == 0 {
				return &videos[i]
			} else if rule == 1 && videos[i].W >= videos[i].H {
				return &videos[i]
			} else if rule == 2 && videos[i].W < videos[i].H {
				return &videos[i]
			}
			if candidateIndex == -1 {
				candidateIndex = i
			}
		}
	}
	if candidateIndex >= 0 {
		return &videos[candidateIndex]
	} else {
		return nil
	}
}

// rule 1: 横屏 2:竖屏
func (raw *RawAdObj) HasMatchedNativeVideo(ctx *http_context.Context) bool {
	if len(raw.Videos) == 0 {
		return false
	}
	if !raw.HasMatchedCreative(ctx) {
		return false
	}
	if icon := raw.getMatchedIcon(ctx.Lang); icon == nil {
		return false
	}
	if raw.TryGetMatchedVideo(ctx) == nil {
		return false
	}
	return true
}

func (raw *RawAdObj) HasMatchedCreative(ctx *http_context.Context) bool {
	if len(raw.Creatives) == 0 {
		return false
	}
	if GetMaxMatchedImg(raw.Creatives, ctx.Lang, ctx.ImgW, ctx.ImgH, ctx.ImgRule, true) != nil {
		return true
	}
	if ctx.Lang != "ALL" {
		if GetMaxMatchedImg(raw.Creatives, "ALL", ctx.ImgW, ctx.ImgH, ctx.ImgRule, true) != nil {
			return true
		}
	}
	ratio := util.LaplaceSmooth(ctx.ImgW, ctx.ImgH)

	if raw.RenderImgs != nil {
		for sizeStr, _ := range raw.RenderImgs {
			size, _ := strconv.Atoi(sizeStr)
			rw := size / 10000
			rh := size % 10000
			rRatio := util.LaplaceSmooth(rw, rh)
			if math.Abs(rRatio-ratio) < 0.1 {
				return true
			}
		}
	}

	return false
}

// maybe return nil
func (raw *RawAdObj) getMatchedVideo(lang string, w, h int) *Video {
	rule := 1 // 表示横屏
	if w < h {
		rule = 2
	}
	videos := raw.Videos[lang]
	if len(videos) == 0 {
		videos = raw.Videos["ALL"]
	}
	for _, video := range videos {
		if video.Url == "" {
			continue
		}
		if rule == 1 && video.W >= video.H {
			return &video
		} else if rule == 2 && video.W < video.H {
			return &video
		}
	}
	return nil
}

// maybe return nil
func (raw *RawAdObj) getMatchedCreative(lang string, w, h, rule int) *Img {
	creative := GetMaxMatchedImg(raw.Creatives, lang, w, h, rule, true)
	if creative == nil && lang != "ALL" {
		creative = GetMaxMatchedImg(raw.Creatives, "ALL", w, h, rule, true)
	}
	// NOTICE: now, creative maybe zero

	// XXX: get render offer
	if raw.RenderImgs != nil {
		toReplace := false
		renderUrl := ""
		renderW, renderH := 0, 0

		ratio := util.LaplaceSmooth(w, h)

		creativeRatio := 65535.0 // 用一个足够大的数来初始化
		if creative != nil {
			creativeRatio = util.LaplaceSmooth(creative.Width, creative.Height)
		}
		delta := math.Abs(ratio - creativeRatio)

		for sizeStr, url := range raw.RenderImgs {
			size, _ := strconv.Atoi(sizeStr)
			rw := size / 10000
			rh := size % 10000
			rRatio := util.LaplaceSmooth(rw, rh)
			newDelta := math.Abs(rRatio - ratio)
			if newDelta < delta {
				delta = newDelta
				toReplace = true
				renderUrl = url
				renderW = rw
				renderH = rh
			}
		}
		if toReplace {
			return &Img{
				Width:  renderW,
				Height: renderH,
				Url:    renderUrl,
				Lang:   "ALL",
			}
		}
	}
	return creative
}

func (raw *RawAdObj) AddIcon(icon *Img) {
	lang := icon.Lang
	icons, ok := raw.Icons[lang]
	if !ok {
		icons = make([]Img, 0, 4)
	}
	icons = append(icons, *icon)
	raw.Icons[lang] = icons
}

func (raw *RawAdObj) AddIcons(icons []Img) {
	for i := 0; i != len(icons); i++ {
		raw.AddIcon(&icons[i])
	}
}

func (raw *RawAdObj) AddCreative(creative *Img) {
	lang := creative.Lang
	creatives, ok := raw.Creatives[lang]
	if !ok {
		creatives = make([]Img, 0, 4)
	}
	creatives = append(creatives, *creative)
	raw.Creatives[lang] = creatives
}

func (raw *RawAdObj) AddCreatives(creatives []Img) {
	for i := 0; i != len(creatives); i++ {
		raw.AddCreative(&creatives[i])
	}
}

func (raw *RawAdObj) AddTarget(key, value string, belong bool) {
	for i := 0; i != len(raw.Targets); i++ {
		t := &raw.Targets[i]
		if t.key == key {
			if t.belong != belong {
				if belong == true {
					// erase "not in" target conditions
					raw.Targets = append(raw.Targets[:i], raw.Targets[i+1:]...)
					break
				} else {
					// ignore "not in" iff "in" conditions exist
					return
				}
			} else {
				// append value and prevent duplicate
				for j := 0; j != len(t.vals); j++ {
					if t.vals[j] == value {
						return
					}
				}
				t.vals = append(t.vals, value)
			}
			return
		}
	}
	raw.Targets = append(raw.Targets, Target{
		key:    key,
		belong: belong,
		vals:   []string{value},
	})
}

// 对于图片url没有http前缀的，为url加上https前缀
func checkAndFixPrefix(m map[string][]Img) {
	for _, imgs := range m {
		for i := 0; i < len(imgs); i++ {
			// GP上的爬下来的链接，是以//开头的
			if strings.HasPrefix(imgs[i].Url, "//") {
				imgs[i].Url = "https:" + imgs[i].Url
			} else if !strings.HasPrefix(imgs[i].Url, "http://") &&
				!strings.HasPrefix(imgs[i].Url, "https://") {
				imgs[i].Url = "https://" + imgs[i].Url
			}
		}
	}
}

func (raw *RawAdObj) addMaterialTarget() {
	if len(raw.Creatives) == 0 && len(raw.Videos) == 0 {
		return
	}

	if len(raw.Creatives) > 0 {
		raw.AddTarget("material", "img", true)
	}
	if len(raw.Videos) > 0 { // 视频
		raw.AddTarget("material", "video", true)
	}
	raw.AddTarget("material", "any", true)
}

func (raw *RawAdObj) PreProcess() *RawAdObj {
	checkAndFixPrefix(raw.Creatives)
	checkAndFixPrefix(raw.Icons)
	raw.addMaterialTarget()

	return raw.genBloomIndex().genDnf()
}

func (raw *RawAdObj) genBloomIndex() *RawAdObj {
	raw.CtBloomIndex = ct_bloom.CalcIndexes(raw.AppDownload.PkgName)
	return raw
}

func (raw *RawAdObj) genDnf() *RawAdObj {
	lconj, rconj := dnf.GetDelimOfConj()
	lset, rset := dnf.GetDelimOfSet()
	setsep := dnf.GetSeparatorOfSet()

	amts := make([]string, 0, len(raw.Targets))
	for i := 0; i != len(raw.Targets); i++ {
		var op string
		t := raw.Targets[i]
		if t.belong {
			op = "in"
		} else {
			op = "not in"
		}
		amt := fmt.Sprintf("%s %s %c %s %c",
			t.key, op, lset, strings.Join(t.vals, string(setsep)), rset)
		amts = append(amts, amt)
	}
	if len(amts) != 0 {
		raw.Dnf = fmt.Sprintf("%c %s %c", lconj, strings.Join(amts, " and "), rconj)
	}
	return raw
}

func (raw *RawAdObj) genThirdPartyImpTk(ctx *http_context.Context) string {
	if len(raw.ThirdPartyImpTks) <= 0 {
		return ""
	}

	var impArg string

	switch raw.Channel {
	case "glp":
		impArg = raw.genGlispaImpUrlArg(ctx)
	case "nglp":
		impArg = raw.genGlispaImpUrlArg(ctx)
	case "irs":
		impArg = raw.genIronSourceImpUrlArg(ctx)
	case "vtpa":
		impArg = raw.genTpaImpUrlArgs(ctx)
	case "apa":
		return raw.replaceAppiaImpUrl(ctx)
	case "vmvt":
		return raw.genMobvistaImpUrl(ctx)
	case "vym":
		return raw.genYeahmobiImpUrlArg(ctx)
	case "sym":
		return raw.genYeahmobiImpUrlArg(ctx)
	case "v1ym":
		return raw.genYeahmobiImpUrlArg(ctx)
	case "apn":
		return raw.ThirdPartyImpTks[0]
	case "apn2":
		return raw.ThirdPartyImpTks[0]
	case "wdg":
		return raw.ThirdPartyImpTks[0]
	case "mbp":
		return raw.ThirdPartyImpTks[0]
	case "rltm":
		return raw.ThirdPartyImpTks[0]
	// xx和xxj这两个渠道在src/raw_ad/raw_native_ad_obj.go中做宏替换处理
	case "xx":
		return ""
	case "xxj":
		return ""
	case "cht":
		return ""
	default:
		ctx.L.Println("genThirdPartyImpTkArg error, un-handled channel of:", raw.Channel)
		return ""
	}

	if strings.Contains(raw.ThirdPartyImpTks[0], "?") {
		return raw.ThirdPartyImpTks[0] + "&" + impArg
	} else {
		return raw.ThirdPartyImpTks[0] + "?" + impArg
	}
}

func (raw *RawAdObj) genTpaImpUrlArgs(ctx *http_context.Context) string {
	return "tt_sub_aff=" + ctx.SlotId
}

func (raw *RawAdObj) genLiboClkUrlArg(ctx *http_context.Context) string {
	clkUrlArgs := make([]string, 0, 4)
	clkUrlArgs = append(clkUrlArgs, "uc_trans_1=lb")
	clkUrlArgs = append(clkUrlArgs, "uc_trans_2="+ctx.SlotId)

	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	// "oid_country_method_serverid_sv_ran_imp_clkts_gaid_aid_idfa_pkgname_platform_payout"
	clkUrlArgs = append(clkUrlArgs, "uc_trans_3="+fmt.Sprintf(
		"%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		raw.Id,
		ctx.Country,
		ctx.Method,
		ctx.ServerId,
		ctx.SdkVersion,
		ctx.Ran,
		ctx.ImpId,
		ctx.Now.Unix(),
		ctx.Gaid,
		ctx.Aid,
		ctx.Idfa,
		ctx.PkgName,
		ctx.Platform,
		raw.Payout,
		base64OfferPkg,
	))

	return strings.Join(clkUrlArgs, "&")
}

func (raw *RawAdObj) genYeahmobiClkUrlArg(ctx *http_context.Context) string {
	clkUrlArgs := make([]string, 0, 12)

	// XXX: yeahmobi的track_url带了offer_id
	// clkUrlArgs = append(clkUrlArgs, "offer_id="+raw.Id)
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	// aff_sub2到aff_sub7是自定义参数
	clkUrlArgs = append(clkUrlArgs, "aff_sub2="+ctx.SlotId)
	clkUrlArgs = append(clkUrlArgs, "aff_sub3="+ctx.Country+"_"+ctx.Method+"_"+ctx.SdkVersion)
	clkUrlArgs = append(clkUrlArgs, "aff_sub4="+ctx.Ran+"_"+ctx.ServerId)
	clkUrlArgs = append(clkUrlArgs, fmt.Sprintf("aff_sub5=%s_%d_%s", ctx.ImpId, ctx.Now.Unix(), base64OfferPkg))

	if ctx.Platform == "Android" {
		if len(ctx.Gaid) != 0 {
			// yeahmobi's fucking logic
			clkUrlArgs = append(clkUrlArgs, "google_adv_id="+ctx.Gaid)
			clkUrlArgs = append(clkUrlArgs, "gaid="+ctx.Gaid)
			clkUrlArgs = append(clkUrlArgs, "aff_sub7="+ctx.Gaid)
		} else {
			clkUrlArgs = append(clkUrlArgs, "aff_sub7="+ctx.Aid)
		}
	}

	if ctx.Platform == "iOS" {
		if len(ctx.Idfa) != 0 {
			clkUrlArgs = append(clkUrlArgs, "aff_sub7="+ctx.Idfa)
		}
	}

	clkUrlArgs = append(clkUrlArgs, "aff_sub6="+ctx.PkgName)
	clkUrlArgs = append(clkUrlArgs, "idfa="+ctx.Idfa)
	clkUrlArgs = append(clkUrlArgs, "android_id="+ctx.Aid)
	clkUrlArgs = append(clkUrlArgs, "device_os="+ctx.Platform)
	clkUrlArgs = append(clkUrlArgs, "ch="+raw.Channel)

	// aff_sub8是子渠道号
	if raw.Channel == "ym" {
		clkUrlArgs = append(clkUrlArgs, "sub_affiliate_id="+ctx.SlotId)
	} else {
		clkUrlArgs = append(clkUrlArgs, "aff_sub8="+ctx.SlotId)
	}

	if raw.Channel == "v1ym" {
		clkUrlArgs = append(clkUrlArgs, "creative_id="+raw.CreativeId())
	}

	return strings.Join(clkUrlArgs, "&")
}

func (raw *RawAdObj) genGlispaImpUrlArg(ctx *http_context.Context) string {
	args := make([]string, 0, 16)
	args = append(args, "placement="+ctx.SlotId)
	args = append(args, "country="+ctx.Country)

	if ctx.AdType != "4" {
		// 纯无感不给包名
		pkg := getPkg(ctx.PkgName)
		if len(pkg) != 0 {
			args = append(args, "aid="+pkg)
		}
	}

	if len(ctx.Aid) != 0 {
		args = append(args, "m.androidid="+ctx.Aid)
		args = append(args, "m.androidid_md5="+ctx.AidMd5)
		args = append(args, "m.androidid_sha1="+ctx.AidSha1)
	}
	if len(ctx.Gaid) != 0 {
		args = append(args, "m.gaid="+ctx.Gaid)
		args = append(args, "m.gaid_md5="+ctx.GaidMd5)
		args = append(args, "m.gaid_sha1="+ctx.GaidSha1)
	}
	if len(ctx.Idfa) != 0 {
		args = append(args, "m.idfa="+ctx.Idfa)
		args = append(args, "m.idfa_md5="+ctx.IdfaMd5)
		args = append(args, "m.idfa_sha1="+ctx.IdfaSha1)
	}

	args = append(args, "ch="+raw.Channel)

	return strings.Join(args, "&")
}

func (raw *RawAdObj) genGlispaClkUrlArg(ctx *http_context.Context) string {
	args := make([]string, 0, 16)
	args = append(args, "placement="+ctx.SlotId)

	args = append(args, "subid1="+ctx.Aid+"_"+ctx.Idfa+"_"+ctx.Gaid)
	args = append(args, "subid2="+ctx.PkgName)

	args = append(args, "subid3="+ctx.Platform+"_"+ctx.Country+"_"+ctx.Method+"_"+ctx.SdkVersion)

	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	args = append(args, "subid4="+ctx.Ran+"_"+ctx.ServerId+"_"+fmt.Sprintf("%.3f", raw.Payout)+"_"+base64OfferPkg)
	args = append(args, fmt.Sprintf("subid5=%s_%d", ctx.ImpId, ctx.Now.Unix()))

	if len(ctx.Aid) != 0 {
		args = append(args, "m.androidid="+ctx.Aid)
		args = append(args, "m.androidid_md5="+ctx.AidMd5)
		args = append(args, "m.androidid_sha1="+ctx.AidSha1)
	}
	if len(ctx.Gaid) != 0 {
		args = append(args, "m.gaid="+ctx.Gaid)
		args = append(args, "m.gaid_md5="+ctx.GaidMd5)
		args = append(args, "m.gaid_sha1="+ctx.GaidSha1)
	}
	if len(ctx.Idfa) != 0 {
		args = append(args, "m.idfa="+ctx.Idfa)
		args = append(args, "m.idfa_md5="+ctx.IdfaMd5)
		args = append(args, "m.idfa_sha1="+ctx.IdfaSha1)
	}

	args = append(args, "ch="+raw.Channel)

	return strings.Join(args, "&")
}

func (raw *RawAdObj) genIronSourceImpUrlArg(ctx *http_context.Context) string {
	args := make([]string, 0, 4)
	if ctx.Platform == "Android" && len(ctx.Gaid) > 0 {
		args = append(args, "deviceId="+ctx.Gaid)
	} else if ctx.Platform == "iOS" && len(ctx.Idfa) > 0 {
		args = append(args, "deviceId="+ctx.Idfa)
	}
	if ctx.AdType != "4" {
		// 纯无感不给包名
		pkg := getPkg(ctx.PkgName)
		if len(pkg) != 0 {
			args = append(args, "aff_sub1="+pkg)
		}
	}
	args = append(args, "ch="+raw.Channel)
	return strings.Join(args, "&")
}

func (raw *RawAdObj) genIronSourceClkUrlArg(ctx *http_context.Context) string {
	args := make([]string, 0, 16)
	if len(ctx.Gaid) > 0 {
		args = append(args, "deviceId="+ctx.Gaid)
	} else if len(ctx.Idfa) > 0 {
		args = append(args, "deviceId="+ctx.Idfa)
	}

	args = append(args, "p1=aig") // aid + idfa + gaid
	args = append(args, "v1="+ctx.Aid+"_"+ctx.Idfa+"_"+ctx.Gaid)

	args = append(args, "p2=os") //offerid + slot
	args = append(args, "v2="+raw.Id+"_"+ctx.SlotId)

	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	args = append(args, "p3=rsp") // ran + server_id + payout + offer_pkg
	args = append(args, fmt.Sprintf("v3=%s_%s_%.3f_%s", ctx.Ran, ctx.ServerId, raw.Payout, base64OfferPkg))

	args = append(args, "p4=pkg") // pkgName
	args = append(args, "v4="+ctx.PkgName)

	args = append(args, "p5=ip") // impid + platform
	args = append(args, "v5="+ctx.ImpId+"_"+ctx.Platform)

	args = append(args, "p6=cm") // country + method
	args = append(args, "v6="+ctx.Country+"_"+ctx.Method+"_"+ctx.SdkVersion)

	args = append(args, "p7=time") // timestamp
	args = append(args, fmt.Sprintf("v7=%d", ctx.Now.Unix()))

	args = append(args, "ch="+raw.Channel)
	args = append(args, "aff_sub1="+ctx.SlotId)
	return strings.Join(args, "&")
}

func (raw *RawAdObj) genPapayaClkUrlArg(ctx *http_context.Context) string {
	args := make([]string, 0, 8)
	// click 是前缀 ppy + 7位随机数字 : ppy1234567
	args = append(args, "aff_sub="+util.GenClickId("ppy"))

	args = append(args, "aff_sub2="+raw.Id+"_"+ctx.SlotId+"_"+ctx.Ran+"_"+
		ctx.ServerId+"_"+fmt.Sprintf("%.3f", raw.Payout))

	if ctx.Platform == "Android" {
		args = append(args, "aff_sub3="+ctx.Gaid)
	} else if ctx.Platform == "iOS" {
		args = append(args, "aff_sub3="+ctx.Idfa)
	} else {
		ctx.L.Println("[PAPAYA] genPapayaClkUrlArg, platform unknown: ", ctx.Platform)
	}

	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	args = append(args, "aff_sub4="+ctx.Aid+"_"+ctx.Idfa+"_"+ctx.Gaid+"_"+base64PkgName)
	args = append(args, fmt.Sprintf("aff_sub5=%s_%s_%s_%s_%s_%d_%s",
		ctx.ImpId, ctx.Platform, ctx.Country, ctx.Method, ctx.SdkVersion, ctx.Now.Unix(), base64OfferPkg))
	args = append(args, "aff_sub6="+ctx.SlotId)
	args = append(args, "ch="+raw.Channel)

	return strings.Join(args, "&")
}

func (raw *RawAdObj) genAppNextClkUrlArg(ctx *http_context.Context) string {
	args := make([]string, 0, 8)

	//urlApp":"https://admin.appnext.com/appLink.aspx?b=1183&e=6780&q=

	apnParam := "aid=" + ctx.Aid +
		"&idfa=" + ctx.Idfa +
		"&gaid=" + ctx.Gaid +
		"&pn=" + ctx.PkgName +
		"&country=" + ctx.Country +
		"&method=" + ctx.Method +
		"&sid=" + ctx.ServerId +
		"&ran=" + ctx.Ran +
		"&payout=" + fmt.Sprintf("%.3f", raw.Payout) +
		"&imp=" + ctx.ImpId +
		"&clk_ts=" + fmt.Sprintf("%d", ctx.Now.Unix()) +
		"&slot=" + ctx.SlotId +
		"&sv=" + ctx.SdkVersion +
		"&offer=" + raw.Id +
		"&offer_pkg=" + raw.AppDownload.PkgName

	args = append(args, "subid="+ctx.SlotId)

	if ctx.Platform == "Android" {
		args = append(args, "d="+ctx.Gaid)
	} else if ctx.Platform == "iOS" {
		args = append(args, "d="+ctx.Idfa)
	} else {
		ctx.L.Println("[APP NEXT] genAppNextClkUrlArg, platform unknown:", ctx.Platform)
	}
	args = append(args, "ch="+raw.Channel)

	return base64.StdEncoding.EncodeToString([]byte(apnParam)) + "&" + strings.Join(args, "&")
}

func (raw *RawAdObj) genYouAppiClkUrl(ctx *http_context.Context) string {
	// link: http://track.x.com/x/x?accesstoken=xxx&appid=72664&campaignpartnerid=285321&subid=&publishertoken=&publishername=&usertoken=&deviceAndroidId=&deviceIfa=&age=&gender=&publisher_type=&format=
	originURL := raw.AppDownload.TrackLink
	args := strings.Split(originURL, "&")
	if len(args) != 13 {
		ctx.L.Println("[FATAL] untouched code here: YouAppi split track link result != 13, len(args) = ", len(args))
		return originURL
	}

	baseURL := args[0]
	for i := 1; i < len(args); i++ {
		key := args[i]

		if strings.Contains(key, "appid=") {
			baseURL = baseURL + "&" + key
			continue
		}

		if strings.Contains(key, "campaignpartnerid=") {
			baseURL = baseURL + "&" + key
			continue
		}

		switch key {
		case "subid=":
			// "http://logger.zcoup.com/all/v1/conversion?channel=yai&click_id={YOUR_CLICK_ID}&idfa={IDFA}&gaid={GOOGLE_ADV_ID}&aid={ANDROID_ID}&payout={OFFER_PRICE}&offer_id={OFFER_ID}"
			arg := "aid=" + ctx.Aid +
				"&idfa=" + ctx.Idfa +
				"&gaid=" + ctx.Gaid +
				"&pn=" + ctx.PkgName +
				"&country=" + ctx.Country +
				"&method=" + ctx.Method +
				"&sid=" + ctx.ServerId +
				"&ran=" + ctx.Ran +
				"&imp=" + ctx.ImpId +
				"&clk_ts=" + fmt.Sprintf("%d", ctx.Now.Unix()) +
				"&slot=" + ctx.SlotId +
				"&sv=" + ctx.SdkVersion +
				"&platform=" + ctx.Platform +
				"&payout=" + fmt.Sprintf("%.3f", raw.Payout) +
				"&offer_id=" + raw.Id +
				"&offer_pkg=" + raw.AppDownload.PkgName

			baseURL = baseURL + "&" + key + url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(arg)))

		case "publishertoken=":
			baseURL = baseURL + "&" + key + ctx.SlotId

		case "publishername=":
			baseURL = baseURL + "&" + key + ctx.PkgName

		case "usertoken=":
			baseURL = baseURL + "&" + key + ctx.UserId

		case "deviceAndroidId=":
			if ctx.Platform == "Android" {
				var arg string
				if len(ctx.Gaid) > 0 {
					arg = key + ctx.Gaid
				} else if len(ctx.Aid) > 0 {
					arg = key + ctx.Aid
				} else {
					ctx.L.Println("nogaid: deviceAndroidId: gaid", ctx.Gaid, " aid: ", ctx.Aid, "userid: ", ctx.UserId)
					arg = key
				}
				baseURL = baseURL + "&" + arg
			}

		case "deviceIfa=":
			if ctx.Platform == "iOS" {
				var arg string
				if len(ctx.Idfa) > 0 {
					arg = key + ctx.Idfa
				} else {
					ctx.L.Println("no deviceIfa: idfa: ", ctx.Idfa)
					arg = key
				}
				baseURL = baseURL + "&" + arg
			}

		case "age=":
			// TODO
			baseURL = baseURL + "&" + key

		case "gender=":
			// TODO
			baseURL = baseURL + "&" + key

		case "publisher_type=": // app or web
			baseURL = baseURL + "&" + key + "app"

		case "format=": // banner fullscreen app_wall in_content
			if ctx.IntegralWall {
				baseURL = baseURL + "&" + key + "app_wall"
			} else {
				baseURL = baseURL + "&" + key + "banner"
			}

		default:
			ctx.L.Println("[YouAppi] unknown arg: ", key)
		}
	}

	baseURL = baseURL + "&ch=" + raw.Channel
	return baseURL
}

func (raw *RawAdObj) replaceAppiaImpUrl(ctx *http_context.Context) string {
	// http://imps.appia.com/v2/impressionAd.jsp?campaignId=20817&siteId=[YOUR_SITE_ID]&aaid=[USER_AAID]&subSite=[subID]
	if len(raw.ThirdPartyImpTks[0]) != 0 {
		imp := strings.Replace(raw.ThirdPartyImpTks[0], "[USER_IDFA]", ctx.Idfa, 1)
		imp = strings.Replace(imp, "[USER_ANDROID_ID]", ctx.Aid, 1)
		imp = strings.Replace(imp, "[USER_AAID]", ctx.Gaid, 1)
		imp = strings.Replace(imp, "[subID]", ctx.SlotId, 1)
		return imp
	}
	return ""
}

func (raw *RawAdObj) genAppiaClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.AppDownload.TrackLink

	ts := ctx.Now.Unix()
	clkUrl = strings.Replace(clkUrl, "[TIME_STAMP]", fmt.Sprintf("%d", ts), 1)
	clkUrl = strings.Replace(clkUrl, "[USER_IDFA]", ctx.Idfa, 1)
	clkUrl = strings.Replace(clkUrl, "[USER_ANDROID_ID]", ctx.Aid, 1)
	clkUrl = strings.Replace(clkUrl, "[USER_AAID]", ctx.Gaid, 1)
	clkUrl = strings.Replace(clkUrl, "[subID]", ctx.SlotId, 1)

	// https://logger.cloudmobi.net/all/v1/conversion?channel=apa&click_id={clickId}&payout={payout}&offer_id={offerId}&pkg_name={pkgName}&country={country}&sub1={sub1}&sub2={sub2}&sub3={sub3}&sub4={sub4}&sub5={sub5}
	clickId := util.GenClickId("apa_")
	args := make([]string, 0, 32)

	args = append(args, "cv1n=click_id")
	args = append(args, "cv1v="+clickId)

	args = append(args, "cv2n=payout")
	args = append(args, "cv2v="+fmt.Sprintf("%.3f", raw.Payout))

	args = append(args, "cv3n=offer_id")
	args = append(args, "cv3v="+raw.Id)

	args = append(args, "cv4n=pkgname")
	args = append(args, "cv4v="+ctx.PkgName)

	args = append(args, "cv5n=country")
	args = append(args, "cv5v="+ctx.Country)

	// Aid_Idfa_Gaid
	args = append(args, "cv6n=sub1")
	args = append(args, "cv6v="+ctx.Aid+"_"+ctx.Idfa+"_"+ctx.Gaid)

	// method_sid_ran
	args = append(args, "cv7n=sub2")
	args = append(args, "cv7v="+ctx.Method+"_"+ctx.ServerId+"_"+ctx.Ran)

	// imp_platform_slotid
	args = append(args, "cv8n=sub3")
	args = append(args, "cv8v="+ctx.ImpId+"_"+ctx.Platform+"_"+ctx.SlotId)

	// clickts_channel_rawpkg
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	args = append(args, "cv9n=sub4")
	args = append(args, "cv9v="+fmt.Sprintf("%d", ts)+"_"+raw.Channel+"_"+base64OfferPkg)

	// sv
	args = append(args, "cv10n=sub5")
	args = append(args, "cv10v="+ctx.SdkVersion)

	return clkUrl + "&" + strings.Join(args, "&") + "&ch=" + raw.Channel
}

func (raw *RawAdObj) genZoomyClkUrlArg(ctx *http_context.Context) string {
	// https://logger.cloudmobi.net/all/v1/conversion?channel=zom&click_id=%SID%&payout=%PAYOUT%&offer_id=%CAMPID%&sid1=%SID1%&sid2=%SID2%&sid3=%SID3%&sid4=%SID4%&sid5=%SID5%
	// http://clinkadtracking.com/tracking?camp=10053603&pubid=2297&sid=123&aid=xxx&gaid=xxx&idfa=xxx&sid1={sid1}&sid2={sid2}&sid3={sid3}&sid4={sid4}&sid5={sid5}
	args := make([]string, 0, 16)

	args = append(args, "subpubid="+ctx.SlotId)
	// click_id: zom1234567
	args = append(args, "sid="+util.GenClickId("zom"))
	args = append(args, "aid="+ctx.Aid)
	args = append(args, "gaid="+ctx.Idfa)
	args = append(args, "idfa="+ctx.Gaid)

	args = append(args, "sid1="+ctx.SlotId+"_"+ctx.Ran+"_"+ctx.ServerId)
	args = append(args, "sid2="+ctx.PkgName)
	args = append(args, "sid3="+fmt.Sprintf("%.3f", raw.Payout))
	args = append(args, "sid4="+ctx.ImpId+"_"+ctx.Platform+"_"+ctx.Country)

	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	args = append(args, "sid5="+ctx.Method+"_"+ctx.SdkVersion+"_"+
		fmt.Sprintf("%d", ctx.Now.Unix())+"_"+base64OfferPkg)
	args = append(args, "ch="+raw.Channel)

	return strings.Join(args, "&")
}

func (raw *RawAdObj) genMundoClkUrlArg(ctx *http_context.Context) string {
	// https://publisher-api.mm-tracking.com/#campaignList 离线API文档
	// https://logger.cloudmobi.net/all/v1/conversion?channel=mnd&opt_info=%OPT_INFO%&subid1=%SUBID1%&subid2=%SUBID2%&subid3=%SUBID3%&subid4=%SUBID4%&subid5=%SUBID5%&device_id=%DEVICE_ID%&placement=%PLACEMENT%
	// http://kuaptrk.com/mt/as14/OPT_INFO=&subid1=&subid2=&subid3=&subid4=&subid5=&device_id=&format=
	args := make([]string, 0, 12)
	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	args = append(args, "awesomeinfo") // the OPT_INFO

	args = append(args, "subid1="+util.GenClickId(raw.Channel)) // clickid
	subid2 := ctx.Country + "_" + base64PkgName + "_" + ctx.Method + "_" + ctx.SdkVersion + "_" + ctx.Ran + "_" + ctx.ImpId
	args = append(args, "subid2="+subid2)
	subid3 := ctx.Platform + "_" + ctx.ServerId + "_" + ctx.Aid + "_" + ctx.Idfa + "_" +
		ctx.Gaid + "_" + fmt.Sprintf("%d", ctx.Now.Unix()) + "_" + base64OfferPkg
	args = append(args, "subid3="+subid3) // platform

	args = append(args, "subid4="+fmt.Sprintf("%.3f", raw.Payout)) // payout
	args = append(args, "subid5="+raw.Id)                          // offerid

	args = append(args, "placement="+ctx.SlotId) // slotid

	if ctx.Platform == "Android" {
		if len(ctx.Gaid) > 0 {
			args = append(args, "device_id="+ctx.Gaid)
		} else if len(ctx.Aid) > 0 {
			args = append(args, "device_id="+ctx.Aid)
		}
	} else if ctx.Platform == "iOS" {
		if len(ctx.Idfa) > 0 {
			args = append(args, "device_id="+ctx.Idfa)
		}
	}
	args = append(args, "format=")
	args = append(args, "ch="+raw.Channel)

	return strings.Join(args, "&")
}

func (raw *RawAdObj) genPingStartClkUrl(ctx *http_context.Context) string {
	// http://track.pingstart.com/api/v4/click?campaign_id=3349913&publisher_id=52&publisher_slot=&sub_1=&pub_gaid=&pub_idfa=&pub_aid=
	copyUrl := raw.AppDownload.TrackLink
	sliceUrl := strings.Split(copyUrl, "&")

	baseUrl := sliceUrl[0]
	for i := 1; i < len(sliceUrl); i++ {
		key := sliceUrl[i]
		switch key {
		case "publisher_id=52": // our id is 52
			baseUrl = baseUrl + "&" + key
		case "publisher_slot=":
			baseUrl = baseUrl + "&" + key + ctx.SlotId
		case "app_name=":
			baseUrl = baseUrl + "&" + key + delPkgNamePostfix.ReplaceAllString(ctx.PkgName, "")
		case "sub_1=": // clickid
			baseUrl = baseUrl + "&" + key + util.GenClickId("pst")
		case "pub_gaid=":
			if len(ctx.Gaid) > 0 {
				baseUrl = baseUrl + "&" + key + ctx.Gaid
			} else {
				baseUrl = baseUrl + "&" + key
			}
		case "pub_aid=":
			if len(ctx.Aid) > 0 {
				baseUrl = baseUrl + "&" + key + ctx.Aid
			} else {
				baseUrl = baseUrl + "&" + key
			}
		case "pub_idfa=":
			if len(ctx.Idfa) > 0 {
				baseUrl = baseUrl + "&" + key + ctx.Idfa
			} else {
				baseUrl = baseUrl + "&" + key
			}
		default:
			baseUrl += "&" + key
		}
	}

	// 添加自定义参数 sub_2, sub_3
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	para := make([]string, 0, 2)
	para = append(para, "sub_2="+ctx.Ran+"_"+ctx.ServerId+"_"+
		fmt.Sprintf("%.3f", raw.Payout)+"_"+ctx.ImpId+"_"+
		ctx.Platform+"_"+fmt.Sprintf("%d", ctx.Now.Unix())+"_"+base64OfferPkg)

	para = append(para, "sub_3="+url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))+
		"_"+ctx.Country+"_"+ctx.Method+"_"+ctx.SdkVersion+"_"+ctx.Aid+"_"+ctx.Gaid+"_"+ctx.Idfa)

	return baseUrl + "&" + strings.Join(para, "&") + "&ch=" + raw.Channel
}

func (raw *RawAdObj) genPingStart3ClkUrl(ctx *http_context.Context) string {
	trackURL := raw.AppDownload.TrackLink

	trackURL = strings.Replace(trackURL, "app_name=", "", 1)

	params := make([]string, 0, 7)

	params = append(params, "publisher_slot="+ctx.SlotId)
	params = append(params, "pub_gaid="+ctx.Gaid)
	params = append(params, "pub_aid="+ctx.Aid)
	params = append(params, "pub_idfa="+ctx.Idfa)
	params = append(params, "app_name="+delPkgNamePostfix.ReplaceAllString(ctx.PkgName, ""))

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	params = append(params, fmt.Sprintf(
		"sub_1=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg, raw.CreativeId()))

	params = append(params, "ch="+raw.Channel)

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}
	return trackURL
}

func (raw *RawAdObj) genPingStartTClkUrl(ctx *http_context.Context) string {
	trackURL := raw.AppDownload.TrackLink

	// trackURL = strings.Replace(trackURL, "app_name=", "", 1) // 保留track链接中原有的app_name=

	params := make([]string, 0, 4)

	slotId := util.ToMd5("#" + ctx.SlotId + "#")
	params = append(params, "publisher_slot="+slotId)
	params = append(params, "pub_gaid="+ctx.Gaid)
	params = append(params, "pub_aid="+ctx.Aid)
	params = append(params, "pub_idfa="+ctx.Idfa)

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}
	return trackURL
}

func (raw *RawAdObj) genMaasClkUrl(ctx *http_context.Context) string {
	// http://c.o5o4o6.com/?a=360&c=10645&af_pid=183723&E=u5rhadBFX%2f4%3d&s1={Sub_ID}&s2={Clickid}&s3={Sub3}&s4={Sub4}&s5={Sub5}&udid={GAID/IDFA}
	trackURL := raw.AppDownload.TrackLink // copy a url
	clickId := util.GenClickId("mas")
	// replace macro
	trackURL = strings.Replace(trackURL, "{Sub_ID}", ctx.SlotId, 1)
	//trackURL = strings.Replace(trackURL, "{Clickid}", clickId, 1)
	if len(ctx.Idfa) > 0 {
		trackURL = strings.Replace(trackURL, "{GAID/IDFA}", ctx.Idfa, 1)
	} else if len(ctx.Gaid) > 0 {
		trackURL = strings.Replace(trackURL, "{GAID/IDFA}", ctx.Gaid, 1)
	} else {
		fmt.Println("genMaasClkUrl no idfa or gaid, idfa: ", ctx.Idfa, " gaid: ", ctx.Gaid)
	}

	sub3 := ctx.Idfa + "_" + ctx.Aid + "_" + ctx.Gaid + "_" + ctx.SlotId + "_" +
		ctx.Ran + "_" + ctx.ServerId + "_" + raw.Id + "_" + clickId

	sub4 := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))

	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	sub5 := fmt.Sprintf("%.3f", raw.Payout) + "_" + ctx.ImpId + "_" +
		ctx.Platform + "_" + ctx.Country + "_" + ctx.Method + "_" +
		ctx.SdkVersion + "_" + fmt.Sprintf("%d", ctx.Now.Unix()) + "_" + base64OfferPkg

	var params string
	params = params + sub3 + "_"
	params = params + sub4 + "_"
	params = params + sub5

	trackURL = strings.Replace(trackURL, "{Clickid}", params, 1)

	trackURL = strings.Replace(trackURL, "{Sub3}", sub3, 1)
	trackURL = strings.Replace(trackURL, "{Sub4}", sub4, 1)
	trackURL = strings.Replace(trackURL, "{Sub5}", sub5, 1)

	trackURL = trackURL + "&ch=" + raw.Channel

	return trackURL
}

func (raw *RawAdObj) genMobiMaxClkUrl(ctx *http_context.Context) string {
	// https://logger.cloudmobi.net/all/v1/conversion?channel=mbx&parameter1={sub1}&parameter2={sub2}&parameter3={sub3}

	args := make([]string, 0, 8)
	urls := strings.Split(raw.AppDownload.TrackLink, "?")
	if len(urls) != 2 {
		fmt.Println("[MobiMax] spilt trackurl failed, url: ", raw.AppDownload.TrackLink)
	}
	trackUrl := urls[0]
	timestamp := ctx.Now.Unix()
	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	args = append(args, urls[1])
	// parameter1: clickId_aid_idfa_gaid_slotid_ran_serverid_pkgname
	// parameter2: payout_impid_platform_country_method_sdkver_timestamp_offerid_rawpkg
	// parameter3: slotid
	args = append(args, fmt.Sprintf("sub1=%s_%s_%s_%s_%s_%s_%s_%s",
		util.GenClickId("mbx"), ctx.Aid, ctx.Idfa, ctx.Gaid, ctx.SlotId, ctx.Ran, ctx.ServerId, base64PkgName))
	args = append(args, fmt.Sprintf("sub2=%.3f_%s_%s_%s_%s_%s_%d_%s_%s",
		raw.Payout, ctx.ImpId, ctx.Platform, ctx.Country, ctx.Method, ctx.SdkVersion, timestamp, raw.Id, base64OfferPkg))
	args = append(args, fmt.Sprintf("sub3=%s", ctx.SlotId))

	if ctx.Platform == "Android" {
		if ctx.Aid == "" {
			args = append(args, "aid="+ctx.Gaid)
		} else {
			args = append(args, "aid="+ctx.Aid)
		}
		args = append(args, "gaid="+ctx.Gaid)
	} else if ctx.Platform == "iOS" {
		args = append(args, "idfa="+ctx.Idfa)
	}
	args = append(args, fmt.Sprintf("time=%d", timestamp))
	args = append(args, "ch="+raw.Channel)

	urlArgs := strings.Join(args, "&")

	// 需要MD5， 秘钥写死在代码中
	secret := "www.cloudmobi.net"
	hash := md5.New()
	io.WriteString(hash, fmt.Sprintf("%s%s", urlArgs, secret))
	sign := hash.Sum(nil)

	trackUrl += fmt.Sprintf("?%s&sign=%x", urlArgs, sign)
	return trackUrl
}

func (raw *RawAdObj) genWadogoClkUrlArg(ctx *http_context.Context) string {
	// http://wadogo.go2cloud.rog/aff_c?aff_sub={click_id}&aff_sub2={sub_publisher_id}&google_aid={gaid}&ios_ifa={idfa}
	var params = make([]string, 0, 5)
	// 1.clickid_2.offerid_3.slotid_4.country_5.method_6.sdkversion_7.ran_8.serverid_9.imp_10.clkts_11.gaid_12.aid_13.idfa_14.pkgName_15.platform_16.payout

	// NOTE wdg aff_click_id => wdg2 aff_sub
	aff_sub := "aff_click_id"
	if raw.Channel == "wdg2" {
		aff_sub = "aff_sub"
	}
	// aff_click_id
	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	params = append(params, fmt.Sprintf(
		"%s=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		aff_sub, util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg))
	// aff_sub2
	params = append(params, "aff_sub2="+ctx.SlotId)
	// gaid
	params = append(params, "google_aid="+ctx.Gaid)
	// idfa
	params = append(params, "ios_ifa="+ctx.Idfa)
	// aff_sub4
	params = append(params, "aff_sub4="+ctx.PkgName)

	return strings.Join(params, "&")
}

func (raw *RawAdObj) genTapticaClkUrlArg(ctx *http_context.Context) string {
	// http://clk.taptica.com/aff_c?tt_aff_clickid={}&tt_sub_aff={slotId}&tt_idfa={idfa}&tt_advertising_id={gaid}&tt_app_name={pkg_name}
	var params = make([]string, 0, 6)
	// 1.clickid_2.offerid_3.slotid_4.country_5.method_6.sdkversion_7.ran_8.serverid_9.imp_10.clkts_11.gaid_12.aid_13.idfa_14.pkgName_15.platform_16.payout
	// tt_aff_clickid
	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	params = append(params, "tt_aff_clickid="+fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion, ctx.Ran,
		ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa, base64PkgName, ctx.Platform,
		raw.Payout, base64OfferPkg))

	// tt_sub_aff
	params = append(params, "tt_sub_aff="+ctx.SlotId)

	// tt_idfa
	params = append(params, "tt_idfa="+ctx.Idfa)

	// tt_advertising_id
	params = append(params, "tt_advertising_id="+ctx.Gaid)

	// tt_app_name
	params = append(params, "tt_app_name="+getPkgName(ctx.PkgName))

	params = append(params, "ch="+raw.Channel)

	return strings.Join(params, "&")
}

func (raw *RawAdObj) genAdstrackClkUrlArg(ctx *http_context.Context) string {
	// http://adstract.trackclk.com/click?a=75171717&o=74949210&sub_id={}&sub_id2={}&sub_id3={}&sub_id4={}&sub_id5={}&creative_id={}
	var params = make([]string, 0, 7)
	// sub_id click_id
	params = append(params, "sub_id="+util.GenClickId("adst"))
	// sub_id2 slotid
	params = append(params, "sub_id2="+ctx.SlotId)

	// sub_id3 idfa/gaid
	if raw.Platform == "Android" {
		params = append(params, "sub_id3="+ctx.Gaid)
	} else {
		params = append(params, "sub_id3="+ctx.Idfa)
	}

	// sub_id4 offerId_contry_method_sdkversion_ran_Gaid_aid_idfa_pkgName
	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	params = append(params, "sub_id4="+raw.Id+"_"+ctx.Country+"_"+ctx.Method+"_"+
		ctx.SdkVersion+"_"+ctx.Ran+"_"+ctx.Gaid+"_"+ctx.Aid+"_"+ctx.Idfa+"_"+base64PkgName)

	// sub_id5 serverId_impid_timeStamp_platform_payout
	params = append(params, "sub_id5="+fmt.Sprintf("%s_%s_%d_%s_%.3f_%s", ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Platform, raw.Payout, base64OfferPkg))
	// creative_id
	params = append(params, "creative_id="+raw.CreativeId())

	params = append(params, "ch=adst")
	return strings.Join(params, "&")
}

func (raw *RawAdObj) genMatomyClkUrlArg(ctx *http_context.Context) string {
	// http://network.adsmarket.com/click/j2dymmefqZqMZHKXZcp6w4iQbJZfoIGckWqYnGSig5iPkGqdYqCAm7dibZlgo3-V
	var params = make([]string, 0, 8)
	// dp=clickid
	params = append(params, "dp="+util.GenClickId(raw.Channel))
	// dp2=slotid
	params = append(params, "dp2="+ctx.SlotId)
	if len(ctx.Gaid) > 0 {
		params = append(params, "gaid="+ctx.Gaid)
	}
	if len(ctx.Idfa) > 0 {
		params = append(params, "idfa="+ctx.Idfa)
	}
	// ran_serverid_impid_clkts_payout_platform_pkg
	params = append(params, "dp3="+fmt.Sprintf("%s_%s_%s_%d_%.3f_%s_%s", ctx.Ran, ctx.ServerId, ctx.ImpId,
		ctx.Now.Unix(), raw.Payout, raw.Platform, url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))))
	// gaid_aid_idfa_offerid_country_method_sdkversion
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	params = append(params, "dp4="+ctx.Gaid+"_"+ctx.Aid+"_"+ctx.Idfa+"_"+raw.Id+"_"+ctx.Country+"_"+ctx.Method+"_"+ctx.SdkVersion+"_"+base64OfferPkg)
	params = append(params, "ch=mtm")

	return strings.Join(params, "&")
}

func (raw *RawAdObj) genInplayableClkUrl(ctx *http_context.Context) string {
	trackURL := raw.AppDownload.TrackLink

	params := make([]string, 0, 6)

	params = append(params, "channel="+ctx.SlotId)
	params = append(params, "andid="+ctx.Aid)
	params = append(params, "gaid="+ctx.Gaid)
	params = append(params, "idfa="+ctx.Idfa)

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	params = append(params, fmt.Sprintf(
		"aff_sub=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg, raw.CreativeId()))

	params = append(params, "ch="+raw.Channel)

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}
	return trackURL
}

func (raw *RawAdObj) genWebeyeClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.AppDownload.TrackLink

	// 1.clickid_2.offerid_3.slotid_4.country_5.method_6.sdkversion_7.ran_8.serverid
	// 9.imp_10.clkts_11.gaid_12.aid_13.idfa_14.pkgname_15.platform_16.payout

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	transaction_id := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country,
		ctx.Method, ctx.SdkVersion, ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix())
	p := fmt.Sprintf("%s_%s_%s_%s_%s_%.3f_%s", ctx.Gaid, ctx.Aid,
		ctx.Idfa, base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg)

	base64T := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(transaction_id)))
	base64P := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(p)))

	clkUrl = strings.Replace(clkUrl, "{transaction_id}", base64T, 1)
	clkUrl = strings.Replace(clkUrl, "{geo}", ctx.Country, 1)
	clkUrl = strings.Replace(clkUrl, "{aid}", ctx.Aid, 1)
	clkUrl = strings.Replace(clkUrl, "{client_version}", ctx.SdkVersion, 1)
	clkUrl = strings.Replace(clkUrl, "{gaid}", ctx.Gaid, 1)
	clkUrl = strings.Replace(clkUrl, "{p}", base64P, 1)

	return clkUrl + "&ch=" + raw.Channel
}

func (raw *RawAdObj) genWebeye2ClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.AppDownload.TrackLink

	// 1.clickid_2.offerid_3.slotid_4.country_5.method_6.sdkversion_7.ran_8.serverid
	// 9.imp_10.clkts_11.gaid_12.aid_13.idfa_14.pkgname_15.platform_16.payout

	// http://track.clickhubs.com/v1/ad/click?pubid=***&campid=***&geo={geo}&aid={aid}&os_version={os_version}&gaid={gaid}&sub_id={sub_id}&sub={sub}

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	sub := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country,
		ctx.Method, ctx.SdkVersion, ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(),
		ctx.Gaid, ctx.Aid, ctx.Idfa, base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg)

	clkUrl = strings.Replace(clkUrl, "{geo}", ctx.Country, 1)
	clkUrl = strings.Replace(clkUrl, "{aid}", ctx.Aid, 1)
	clkUrl = strings.Replace(clkUrl, "{os_version}", ctx.SdkVersion, 1)
	clkUrl = strings.Replace(clkUrl, "{gaid}", ctx.Gaid, 1)
	clkUrl = strings.Replace(clkUrl, "{idfa}", ctx.Idfa, 1)
	clkUrl = strings.Replace(clkUrl, "{sub}", sub, 1)
	clkUrl = clkUrl + "&sub_id=" + ctx.SlotId

	return clkUrl + "&ch=" + raw.Channel
}

func (raw *RawAdObj) genWebeyetClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.AppDownload.TrackLink

	clkUrl = strings.Replace(clkUrl, "{geo}", ctx.Country, 1)
	clkUrl = strings.Replace(clkUrl, "{aid}", ctx.Aid, 1)
	clkUrl = strings.Replace(clkUrl, "{os_version}", ctx.Osv, 1)
	clkUrl = strings.Replace(clkUrl, "{gaid}", ctx.Gaid, 1)
	clkUrl = strings.Replace(clkUrl, "{idfa}", ctx.Idfa, 1)

	appId := util.ToMd5("#" + ctx.SlotId + "#")
	subId := util.ToMd5("$" + ctx.SlotId + "$")
	clkUrl = clkUrl + "&sub_id=" + subId
	clkUrl = strings.Replace(clkUrl, "{sub}", appId, 1)

	return clkUrl
}

func (raw *RawAdObj) genInterestMobClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.AppDownload.TrackLink

	clkUrl += "&gaid=" + ctx.Gaid
	clkUrl += "&android_id=" + ctx.Aid
	clkUrl += "&idfa=" + ctx.Idfa

	appId := util.ToMd5("#" + ctx.SlotId + "#")
	clkUrl += "&sub_affid=" + appId

	clkUrl += fmt.Sprintf("&p=%s_%.3f&p1=%s", ctx.Country, raw.Payout, raw.Id)

	return clkUrl
}

func (raw *RawAdObj) genAppSntClkUrlArg(ctx *http_context.Context) string {
	// https://logger.cloudmobi.net/all/v1/conversion?channel=apsnt&click_id={aff_sub}&params1={aff_sub2}&params2={aff_sub3}
	var params = make([]string, 0, 7)
	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	// sub channel id
	params = append(params, "sub_channel="+ctx.SlotId)

	// aff_sub
	params = append(params, fmt.Sprintf("aff_sub=%s_%s_%s_%s_%s_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion))

	// aff_sub2
	params = append(params, fmt.Sprintf("aff_sub2=%s_%s_%s_%d_%s_%s",
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid))

	// aff_sub3
	params = append(params, fmt.Sprintf("aff_sub3=%s_%s_%s_%.3f_%s",
		ctx.Idfa, base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg))

	// idfa
	params = append(params, "idfa="+ctx.Idfa)

	// gaid
	params = append(params, "gaid="+ctx.Gaid)

	// channel
	params = append(params, "ch="+raw.Channel)

	return strings.Join(params, "&")
}

func (raw *RawAdObj) genUcUnionClkUrlArg(ctx *http_context.Context) string {

	// 1.clickid_2.offerid_3.slotid_4.country_5.method_6.sdkversion_7.ran_8.serverid
	// 9.imp_10.clkts_11.gaid_12.aid_13.idfa_14.pkgname_15.platform_16.payout
	var params = make([]string, 0, 3)

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	params = append(params,
		"uc_trans_1="+fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
			util.GenClickId("uci"), raw.Id, ctx.SlotId, ctx.Country, ctx.Method,
			ctx.SdkVersion, ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(),
			ctx.Gaid, ctx.Aid, ctx.Idfa, base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg))
	params = append(params, "subpub="+ctx.SlotId)

	params = append(params, "ch=uci")

	return strings.Join(params, "&")
}

func (raw *RawAdObj) genPubNativeClkUrl(ctx *http_context.Context) string {
	trackURL := raw.AppDownload.TrackLink

	params := make([]string, 0, 4)

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	// aff_sub
	params = append(params, fmt.Sprintf(
		"aff_sub=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg))
	// aff_sub2

	// pub_sub_id
	params = append(params, "pub_sub_id="+ctx.SlotId)

	// channel
	params = append(params, "ch="+raw.Channel)

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}

	return trackURL
}

func (raw *RawAdObj) genStartAppClkUrl(ctx *http_context.Context) string {
	trackUrl := raw.AppDownload.TrackLink

	params := make([]string, 0, 4)

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	// click id
	params = append(params, fmt.Sprintf(
		"clickId=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId("stp"), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg,
	))

	// prod
	params = append(params, "prod="+ctx.SlotId)

	// advId
	if raw.Platform == "Android" {
		params = append(params, "segId=200666528") // segId
		params = append(params, "advId="+ctx.Gaid)
	} else if raw.Platform == "iOS" {
		params = append(params, "segId=200284583") // segId
		params = append(params, "advId="+ctx.Idfa)

	}

	params = append(params, "ch=stp")

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackUrl, "&") {
		trackUrl += args
	} else {
		trackUrl += "&" + args
	}

	return trackUrl
}

func (raw *RawAdObj) genYodaClkUrl(ctx *http_context.Context) string {
	// http://logger.cloudmobi.net/all/v1/conversion?channel=yoda&clid={clid}&payout={payout}&conv_ip={conv_ip}&currency=(USD|RMB)
	args := make([]string, 0, 8)

	args = append(args, "uc_trans_3="+ctx.Idfa)
	args = append(args, "uc_trans_4="+ctx.Aid)
	args = append(args, "uc_trans_5="+ctx.Country)
	args = append(args, "uc_trans_6="+ctx.SlotId)

	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	clid := ctx.SlotId + "_" + ctx.Idfa + "_" + ctx.Aid + "_" +
		raw.Id + "_" + ctx.Platform + "_" + ctx.PkgName + "_" +
		ctx.Country + "_" + ctx.Method + "_" + ctx.Ran + "_" +
		ctx.SdkVersion + "_" + ctx.ServerId + "_" + ctx.ImpId +
		"_" + fmt.Sprintf("%.3f_%d_%s", raw.Payout, ctx.Now.Unix(), base64OfferPkg)
	args = append(args, "uc_trans_7="+clid)
	args = append(args, "ch=yoda")

	return strings.Join(args, "&")
}

func (raw *RawAdObj) genYoda2ClkUrl(ctx *http_context.Context) string {
	// http://logger.cloudmobi.net/all/v1/conversion?channel=yoda2&clid={clid}&payout={payout}&conv_ip={conv_ip}&currency=(USD|RMB)
	args := make([]string, 0, 8)
	args = append(args, "idfa="+ctx.Idfa)
	args = append(args, "aid="+ctx.Aid)
	args = append(args, "country="+ctx.Country)
	args = append(args, fmt.Sprintf("price=%.3f", raw.Payout))
	args = append(args, "campaign=djj")

	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	clid := ctx.SlotId + "_" + ctx.Idfa + "_" + ctx.Aid + "_" +
		raw.Id + "_" + ctx.Platform + "_" + ctx.PkgName + "_" +
		ctx.Country + "_" + ctx.Method + "_" + ctx.Ran + "_" +
		ctx.SdkVersion + "_" + ctx.ServerId + "_" + ctx.ImpId +
		"_" + fmt.Sprintf("%.3f_%d_%s", raw.Payout, ctx.Now.Unix(), base64OfferPkg)
	args = append(args, "clid="+clid)

	return strings.Join(args, "&")
}

func (raw *RawAdObj) genAppcoachClkUrl(ctx *http_context.Context) string {

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	s1 := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%.3f_%s",
		raw.Id, ctx.Country, ctx.Method, ctx.SdkVersion, ctx.Ran, ctx.ServerId,
		ctx.ImpId, ctx.Now.Unix(), base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg)

	args := make([]string, 0, 8)
	args = append(args, "s1="+s1)
	args = append(args, "sc="+ctx.SlotId)
	args = append(args, "s2="+ctx.Aid)
	args = append(args, "s3="+ctx.Gaid)
	args = append(args, "s4="+ctx.Idfa)
	args = append(args, "s5="+util.GenClickId(raw.Channel))

	args = append(args, "ch=apc")

	return strings.Join(args, "&")
}

func (raw *RawAdObj) genAppliftClkUrl(ctx *http_context.Context) string {
	trackURL := raw.AppDownload.TrackLink

	params := make([]string, 0, 6)

	// source
	params = append(params, "source="+ctx.SlotId)

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	// aff_sub
	params = append(params, fmt.Sprintf(
		"aff_click_id=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId("aft"), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg))

	if ctx.Platform == "iOS" {
		params = append(params, "ios_ifa="+ctx.Idfa)
	}
	if ctx.Platform == "Android" {
		params = append(params, "unid="+ctx.Gaid)
	}

	// channel
	params = append(params, "ch=aft")

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}

	return trackURL
}

func (raw *RawAdObj) genMobusiClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.AppDownload.TrackLink

	// 1.clickid_2.offerid_3.slotid_4.country_5.method_6.sdkversion_7.ran_8.serverid
	// 9.imp_10.clkts_11.gaid_12.aid_13.idfa_14.pkgname_15.platform_16.payout

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	click_id := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country,
		ctx.Method, ctx.SdkVersion, ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(),
		ctx.Gaid, ctx.Aid, ctx.Idfa, base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg)

	base64ClickId := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(click_id)))

	clkUrl = strings.Replace(clkUrl, "{pubid}", ctx.SlotId, -1)
	clkUrl = strings.Replace(clkUrl, "{click_id}", base64ClickId, 1)

	return clkUrl + "&ch=" + raw.Channel
}

func (raw *RawAdObj) genMobuppsClkUrl(ctx *http_context.Context) string {
	trackURL := raw.AppDownload.TrackLink

	params := make([]string, 0, 5)

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	// aff_sub
	params = append(params, fmt.Sprintf(
		"aff_sub=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg))

	// source
	params = append(params, "source="+ctx.SlotId)

	// gaid, idfa
	params = append(params, "google_aid="+ctx.Gaid)
	params = append(params, "ios_ifa="+ctx.Idfa)

	// channel
	params = append(params, "ch=mbp")

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}

	return trackURL
}

func (raw *RawAdObj) genSmarterClkUrl(ctx *http_context.Context) string {
	//https://logger.cloudmobi.net/all/v1/conversion?channel=smt&cid={aff_sub}&payout={payout}&offer_id={offer_id}
	trackURL := raw.AppDownload.TrackLink

	params := make([]string, 0, 6)

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	params = append(params, fmt.Sprintf(
		"aff_sub=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg))
	// gaid
	params = append(params, "gaid="+ctx.Gaid)
	// andid
	params = append(params, "andid="+ctx.Aid)
	// appname
	params = append(params, "appname="+ctx.PkgName)
	// idfa
	params = append(params, "idfa="+ctx.Idfa)
	//slotid
	params = append(params, "channel="+ctx.SlotId)
	// channel
	params = append(params, "ch=smt")

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}
	return trackURL
}

func (raw *RawAdObj) genAdtimingClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.AppDownload.TrackLink

	click_id := fmt.Sprintf("%s_%s_%d_%s_%.3f", raw.Id, ctx.Country, ctx.Now.Unix(), ctx.Platform, raw.Payout)

	if ctx.Platform == "Android" && len(ctx.Gaid) > 0 {
		clkUrl = strings.Replace(clkUrl, "{device_id}", strings.ToLower(ctx.Gaid), 1)
	} else if ctx.Platform == "iOS" && len(ctx.Idfa) > 0 {
		clkUrl = strings.Replace(clkUrl, "{device_id}", strings.ToUpper(ctx.Idfa), 1)
	}

	clkUrl = strings.Replace(clkUrl, "{click_id}", click_id, 1)
	clkUrl = strings.Replace(clkUrl, "{subid}", ctx.SlotId, 1)

	return clkUrl + "&ch=" + raw.Channel
}

func (raw *RawAdObj) genMobiSummerClkUrl(ctx *http_context.Context) string {
	// http://logger.cloudmobi.net/all/v1/conversion?channel=mbm&click_id={aff_sub2}&sub={aff_sub}&payout={payout}
	// http://click.howdoesin.net/aff_c?offer_id=28230815&affiliate_id=4701&aid={idfa}&device_id={device_id}
	clkUrl := raw.AppDownload.TrackLink
	// 替换链接里的宏如： {idfa},{gaid}

	params := make([]string, 0, 5)

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	if ctx.Platform == "Android" {
		params = append(params, "gaid="+ctx.Gaid)
	} else if ctx.Platform == "iOS" {
		params = append(params, "aid="+ctx.Idfa)
	} else {
		ctx.L.Println("[genMobiSummerClkUrl] unknown platform: ", ctx.Platform)
	}

	params = append(params, fmt.Sprintf(
		"aff_sub=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg))

	params = append(params, "aff_sub2="+util.GenClickId(raw.Channel))
	params = append(params, "aff_sub5="+ctx.SlotId)
	params = append(params, "ch=mbm")

	args := strings.Join(params, "&")

	if strings.HasSuffix(clkUrl, "&") {
		clkUrl += args
	} else {
		clkUrl += "&" + args
	}
	return clkUrl
}

func (raw *RawAdObj) genMobvistaTClkUrl(ctx *http_context.Context) string {
	link := raw.AppDownload.TrackLink
	params := make([]string, 0, 8)

	if ctx.Platform == "Android" {
		params = append(params, "mb_gaid="+ctx.Gaid)
	} else if ctx.Platform == "iOS" {
		params = append(params, "mb_idfa="+ctx.Idfa)
	}

	params = append(params, fmt.Sprintf(
		"aff_sub=country_%s,clkpayout_%.3f,offerid_%s,idfa_%s,gaid_%s",
		ctx.Country, raw.Payout, raw.Id, ctx.Idfa, ctx.Gaid))

	subId := util.ToMd5("$" + ctx.SlotId + "$")
	params = append(params, "mb_subid="+subId)

	params = append(params, "mb_package=com.quvideo.XiaoYing")

	args := strings.Join(params, "&")

	if strings.HasSuffix(link, "&") {
		link += args
	} else {
		link += "&" + args
	}
	return link
}

func (raw *RawAdObj) genMobvistaUrl(link string, ctx *http_context.Context) string {
	//https://logger.cloudmobi.net/all/v1/conversion?channel=mvt&payout={mb_payout}&clickid={aff_sub}&subid={mb_subid}

	params := make([]string, 0, 5)

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	if ctx.Platform == "Android" {
		params = append(params, "mb_gaid="+ctx.Gaid)
	} else if ctx.Platform == "iOS" {
		params = append(params, "mb_idfa="+ctx.Idfa)
	} else {
		ctx.L.Println("[genMobvistaUrl] unknown platform: ", ctx.Platform, " link: ", link)
	}

	params = append(params, fmt.Sprintf(
		"aff_sub=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg))

	params = append(params, "mb_subid="+ctx.SlotId)
	params = append(params, "ch="+raw.Channel)

	args := strings.Join(params, "&")

	if strings.HasSuffix(link, "&") {
		link += args
	} else {
		link += "&" + args
	}
	return link
}

func (raw *RawAdObj) genWecloudClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.AppDownload.TrackLink

	// 1.clickid_2.offerid_3.slotid_4.country_5.method_6.sdkversion_7.ran_8.serverid
	// 9.imp_10.clkts_11.gaid_12.aid_13.idfa_14.pkgname_15.platform_16.payout

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	click_id := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country,
		ctx.Method, ctx.SdkVersion, ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(),
		ctx.Gaid, ctx.Aid, ctx.Idfa, base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg)

	clkUrl = strings.Replace(clkUrl, "{aid}", ctx.Aid, 1)
	clkUrl = strings.Replace(clkUrl, "{adid}", ctx.Gaid, 1)

	params := make([]string, 0, 3)
	params = append(params, "aff_sub="+click_id)
	params = append(params, "aff_sub5="+ctx.SlotId)
	params = append(params, "ch="+raw.Channel)

	args := strings.Join(params, "&")

	if strings.HasSuffix(clkUrl, "&") {
		clkUrl += args
	} else {
		clkUrl += "&" + args
	}
	return clkUrl
}

func (raw *RawAdObj) genCentrixlinkClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.AppDownload.TrackLink

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	click_id := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country,
		ctx.Method, ctx.SdkVersion, ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(),
		ctx.Gaid, ctx.Aid, ctx.Idfa, base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg)

	params := make([]string, 0, 2)
	params = append(params, "aff_click_id="+click_id)
	params = append(params, "sub_affid="+ctx.SlotId)
	if ctx.Platform == "Android" {
		params = append(params, "device_id="+ctx.Gaid)
	} else if ctx.Platform == "iOS" {
		params = append(params, "device_id="+ctx.Idfa)
	} else {
		ctx.L.Println("genCentrixlinkClkUrl nuknow platform: ", ctx.Platform)
	}
	params = append(params, "ch="+raw.Channel)
	args := strings.Join(params, "&")

	if strings.HasSuffix(clkUrl, "&") {
		clkUrl += args
	} else {
		clkUrl += "&" + args
	}

	return clkUrl
}

func (raw *RawAdObj) genAppsFlyerClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.ClkUrl
	if len(clkUrl) == 0 {
		clkUrl = raw.AppDownload.TrackLink
	}

	// 1.clickid_2.offerid_3.slotid_4.country_5.method_6.sdkversion_7.ran_8.serverid
	// 9.imp_10.clkts_11.gaid_12.aid_13.idfa_14.pkgname_15.platform_16.payout_17.offername_18.广告主

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	click_id := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country,
		ctx.Method, ctx.SdkVersion, ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(),
		ctx.Gaid, ctx.Aid, ctx.Idfa, base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg, raw.Spon)

	clkUrl = strings.Replace(clkUrl, "{clickid}", click_id, 1)
	clkUrl = strings.Replace(clkUrl, "{af_siteid}", ctx.SlotId, 1)
	clkUrl = strings.Replace(clkUrl, "{android_id}", ctx.Aid, 1)
	clkUrl = strings.Replace(clkUrl, "{af_adset}", delPkgNamePostfix.ReplaceAllString(ctx.PkgName, ""), 1)
	clkUrl = strings.Replace(clkUrl, "{advertising_id}", ctx.Gaid, 1)
	clkUrl = strings.Replace(clkUrl, "{idfa}", ctx.Idfa, 1)
	clkUrl = strings.Replace(clkUrl, "{af_channel}", raw.Channel, 1)

	return clkUrl + "&ch=" + raw.Channel
}

func (raw *RawAdObj) genReyunClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.ClkUrl
	if len(clkUrl) == 0 {
		clkUrl = raw.AppDownload.TrackLink
	}

	// 1.clickid_2.offerid_3.slotid_4.country_5.method_6.sdkversion_7.ran_8.serverid
	// 9.imp_10.clkts_11.gaid_12.aid_13.idfa_14.pkgname_15.platform_16.payout_17.offername
	// 18.广告主_19.channel

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	clickid := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s_%s_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country,
		ctx.Method, ctx.SdkVersion, ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(),
		ctx.Gaid, ctx.Aid, ctx.Idfa, base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg, raw.Spon, raw.Channel)

	clkUrl = strings.Replace(clkUrl, "{idfa}", ctx.Idfa, 1)
	clkUrl = strings.Replace(clkUrl, "{subchannel}", ctx.SlotId, 1)
	clkUrl = strings.Replace(clkUrl, "{clickid}", clickid, 1)

	return clkUrl + "&ch=" + raw.Channel
}

func (raw *RawAdObj) genApptrackClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.ClkUrl
	if len(clkUrl) == 0 {
		clkUrl = raw.AppDownload.TrackLink
	}

	clickid := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country,
		ctx.Method, ctx.SdkVersion, ctx.ImpId, ctx.Now.Unix(),
		ctx.Gaid, ctx.Aid, ctx.Idfa, ctx.PkgName, ctx.Platform, raw.Payout, raw.Channel)

	clkUrl = strings.Replace(clkUrl, "{_idfa_}", ctx.Idfa, 1)
	clkUrl = strings.Replace(clkUrl, "{_imei_}", "", 1) //第三方要求必传，但我们只监测iOS，所以置空
	clkUrl = strings.Replace(clkUrl, "{_clickid_}", base64.StdEncoding.EncodeToString([]byte(clickid)), 1)

	return clkUrl + "&ch=" + raw.Channel
}

func (raw *RawAdObj) genAdjustClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.ClkUrl
	if len(clkUrl) == 0 {
		clkUrl = raw.AppDownload.TrackLink
	}

	// 2.offerid_3.slotid_4.country_5.method_6.sdkversion_7.ran_8.serverid
	// 9.imp_10.clkts_11.gaid_12.aid_13.idfa_15.platform_16.payout_
	// 18.广告主_19.channel

	click_id := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%.3f_%s_%s",
		raw.Id, ctx.SlotId, ctx.Country,
		ctx.Method, ctx.SdkVersion, ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(),
		ctx.Gaid, ctx.Aid, ctx.Idfa, ctx.Platform, raw.Payout, raw.Spon, raw.Channel)

	postBack := url.QueryEscape("https://logger.cloudmobi.net/all/v1/conversion?channel=adj&clickid=" + click_id)
	clkUrl += "?install_callback=" + postBack

	return clkUrl + "&ch=" + raw.Channel
}

func (raw *RawAdObj) genAffleClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.AppDownload.TrackLink

	click_id := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%.3f",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country,
		ctx.Method, ctx.SdkVersion, ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(),
		ctx.Gaid, ctx.Aid, ctx.Idfa, ctx.Platform, raw.Payout)

	clkUrl = strings.Replace(clkUrl, "{Sub_ID}", ctx.SlotId, 1)
	clkUrl = strings.Replace(clkUrl, "{Clickid}", click_id, 1)
	clkUrl = strings.Replace(clkUrl, "{Sub3}", "", 1)
	clkUrl = strings.Replace(clkUrl, "{Sub4}", ctx.PkgName, 1)
	clkUrl = strings.Replace(clkUrl, "{Sub5}", "", 1)

	if ctx.Platform == "Android" && len(ctx.Gaid) > 0 {
		clkUrl = strings.Replace(clkUrl, "{GAID/IDFA}", strings.ToLower(ctx.Gaid), 1)
	} else if ctx.Platform == "iOS" && len(ctx.Idfa) > 0 {
		clkUrl = strings.Replace(clkUrl, "{GAID/IDFA}", strings.ToUpper(ctx.Idfa), 1)
	}

	return clkUrl + "&ch=" + raw.Channel
}

func (raw *RawAdObj) genStarmobsClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.AppDownload.TrackLink

	click_id := fmt.Sprintf("%s_%s_%s_%s_%s_%.3f_%s",
		raw.Id, ctx.SlotId, ctx.Country,
		ctx.SdkVersion, ctx.ServerId,
		raw.Payout, raw.Channel)

	params := make([]string, 0, 5)
	params = append(params, fmt.Sprintf("platform=%s", ctx.Platform))

	if ctx.Platform == "Android" {
		if len(ctx.Gaid) > 0 {
			params = append(params, fmt.Sprintf("gaid=%s", ctx.Gaid))
		} else if len(ctx.Aid) > 0 {
			params = append(params, fmt.Sprintf("aid=%s", ctx.Aid))
		} else {
			params = append(params, "gaid=")
		}
	} else if ctx.Platform == "iOS" {
		params = append(params, fmt.Sprintf("idfa=%s", ctx.Idfa))
	}

	params = append(params, fmt.Sprintf("click_id=%s", click_id))

	paramsStr := strings.Join(params, "&")

	if strings.HasSuffix(clkUrl, "&") {
		clkUrl += paramsStr
	} else {
		clkUrl += "&" + paramsStr
	}
	return clkUrl
}

func (raw *RawAdObj) genMobvistaClkUrl(ctx *http_context.Context) string {
	//https://logger.cloudmobi.net/all/v1/conversion?channel=mvt&payout={mb_payout}&clickid={aff_sub}&subid={mb_subid}
	return raw.genMobvistaUrl(raw.AppDownload.TrackLink, ctx)
}

func (raw *RawAdObj) genMobvistaImpUrl(ctx *http_context.Context) string {
	return raw.genMobvistaUrl(raw.ThirdPartyImpTks[0], ctx)
}

// 直客offer参数拼接
// Talking Data
func (raw *RawAdObj) genTalkingDataClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.ClkUrl
	if len(clkUrl) == 0 {
		clkUrl = raw.AppDownload.TrackLink
	}

	// 1.clickid_2.offerid_3.slotid_4.country_5.method_6.sdkversion_7.ran_8.serverid
	// 9.imp_10.clkts_11.gaid_12.aid_13.idfa_14.pkgname_15.platform_16.payout_17.广告主_18.channel

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	click_id := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s_%s_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country,
		ctx.Method, ctx.SdkVersion, ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(),
		ctx.Gaid, ctx.Aid, ctx.Idfa, base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg, raw.Spon, raw.Channel)

	clkUrl = strings.Replace(clkUrl, "{click_id}", click_id, 1)
	if raw.Platform == "Android" {
		clkUrl = strings.Replace(clkUrl, "{aid}", ctx.Aid, 1)
		clkUrl = strings.Replace(clkUrl, "{gaid}", ctx.Gaid, 1)
	} else if raw.Platform == "iOS" {
		clkUrl = strings.Replace(clkUrl, "{idfa}", ctx.Idfa, 1)
	}

	return clkUrl + "&ch=" + raw.Channel
}

func (raw *RawAdObj) genOnewayClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.AppDownload.TrackLink

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	clickid := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country,
		ctx.Method, ctx.SdkVersion, ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(),
		ctx.Gaid, ctx.Aid, ctx.Idfa, base64PkgName, ctx.Platform, raw.Payout,
		base64OfferPkg, raw.CreativeId())

	clkUrl = strings.Replace(clkUrl, "{sub_id}", ctx.SlotId, 1)
	clkUrl = strings.Replace(clkUrl, "{idfa}", ctx.Idfa, 1)
	clkUrl = strings.Replace(clkUrl, "{gaid}", ctx.Gaid, 1)
	clkUrl = strings.Replace(clkUrl, "{clickid}", clickid, 1)

	return clkUrl + "&ch=" + raw.Channel
}

func (raw *RawAdObj) genAvazuClkUrl(ctx *http_context.Context) string {
	trackURL := raw.AppDownload.TrackLink

	params := make([]string, 0, 5)

	params = append(params, "nw_sub_aff="+ctx.SlotId)
	if raw.Platform == "Android" {
		params = append(params, "device_id="+ctx.Gaid)
	} else {
		params = append(params, "device_id="+ctx.Idfa)
	}

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	params = append(params, fmt.Sprintf(
		"dv1=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg, raw.CreativeId()))

	params = append(params, "dv5="+ctx.SlotId)

	params = append(params, "ch="+raw.Channel)

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}
	return trackURL
}

func (raw *RawAdObj) genAvazutClkUrl(ctx *http_context.Context) string {
	trackURL := raw.AppDownload.TrackLink

	params := make([]string, 0, 8)

	subId := util.ToMd5("$" + ctx.SlotId + "$")
	params = append(params, "nw_sub_aff="+subId)
	if raw.Platform == "Android" {
		params = append(params, "device_id="+ctx.Gaid)
	} else {
		params = append(params, "device_id="+ctx.Idfa)
	}

	params = append(params, fmt.Sprintf("dv3=offerid_%s,country_%s,payout_%.3f", raw.Id, ctx.Country, raw.Payout))
	params = append(params, "dv4=platform_"+raw.Platform)
	params = append(params, "dv5=subid_"+subId)

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}
	return trackURL
}

func (raw *RawAdObj) genSmarter2ClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.AppDownload.TrackLink

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	clickid := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country,
		ctx.Method, ctx.SdkVersion, ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(),
		ctx.Gaid, ctx.Aid, ctx.Idfa, base64PkgName, ctx.Platform, raw.Payout,
		base64OfferPkg, raw.CreativeId())

	clkUrl = strings.Replace(clkUrl, "{aff_sub}", clickid, 1)
	clkUrl = strings.Replace(clkUrl, "{channel}", ctx.SlotId, 1)
	clkUrl = strings.Replace(clkUrl, "{android}", ctx.Aid, 1)
	clkUrl = strings.Replace(clkUrl, "{gaid}", ctx.Gaid, 1)
	clkUrl = strings.Replace(clkUrl, "{idfa}", ctx.Idfa, 1)

	return clkUrl + "&ch=" + raw.Channel
}

func (raw *RawAdObj) genOffersLookClkUrl(ctx *http_context.Context) string {
	trackUrl := raw.AppDownload.TrackLink

	params := make([]string, 0, 6)

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	clickid := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country,
		ctx.Method, ctx.SdkVersion, ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(),
		ctx.Gaid, ctx.Aid, ctx.Idfa, base64PkgName, ctx.Platform, raw.Payout,
		base64OfferPkg, raw.CreativeId())

	if raw.Platform == "Android" {
		params = append(params, "google_aid="+ctx.Gaid)
	} else {
		params = append(params, "ios_idfa="+ctx.Idfa)
	}
	params = append(params, "aff_sub1="+clickid)
	params = append(params, "aff_sub5="+ctx.PkgName)
	params = append(params, "source_id="+ctx.SlotId)
	params = append(params, "ch="+raw.Channel)

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackUrl, "&") {
		trackUrl += args
	} else {
		trackUrl += "&" + args
	}
	return trackUrl
}

func (raw *RawAdObj) genDuUnionClkUrl(ctx *http_context.Context) string {
	trackURL := raw.AppDownload.TrackLink

	params := make([]string, 0, 6)

	// gaid or idfa
	if raw.Platform == "Android" {
		params = append(params, "google_aid="+ctx.Gaid)
	} else {
		params = append(params, "ios_ifa="+ctx.Idfa)
	}
	// aff_sub click id
	clickId := util.GenClickId(raw.Channel)
	params = append(params, "aff_sub="+clickId)
	// aff_sub2 slotid
	params = append(params, "aff_sub2="+ctx.SlotId)
	// aff_sub4  appname
	params = append(params, "aff_sub4="+ctx.PkgName)

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	params = append(params, fmt.Sprintf(
		"aff_sub5=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s_%s",
		clickId, raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg, raw.CreativeId()))

	params = append(params, "ch="+raw.Channel)

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}
	return trackURL
}

func (raw *RawAdObj) genDuUnionTClkUrl(ctx *http_context.Context) string {
	trackURL := raw.AppDownload.TrackLink

	params := make([]string, 0, 6)

	// gaid or idfa
	if raw.Platform == "Android" {
		params = append(params, "google_aid="+ctx.Gaid)
	} else {
		params = append(params, "ios_ifa="+ctx.Idfa)
	}

	clickId := util.GenClickId("")
	params = append(params, "aff_sub="+clickId)
	// aff_sub2 slotid
	subId := util.ToMd5("$" + ctx.SlotId + "$")
	params = append(params, "aff_sub2="+subId)

	params = append(params, "aff_sub4=com.quvideo.XiaoYing")

	sub5 := fmt.Sprintf("aff_sub5=country_%s,clkpayout_%0.3f,offerid_%s,idfa_%s,gaid_%s,aid_%s",
		ctx.Country, raw.Payout, raw.Id, ctx.Idfa, ctx.Gaid, ctx.Aid)
	params = append(params, sub5)

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}
	return trackURL
}

func (raw *RawAdObj) genInmobiClkUrl(ctx *http_context.Context) string {
	trackURL := raw.AppDownload.TrackLink
	params := make([]string, 0, 8)

	clickId := util.GenClickId("")
	params = append(params, "aff_sub="+clickId)
	// aff_sub2 slotid
	subId := util.ToMd5("$" + ctx.SlotId + "$")
	params = append(params, "aff_sub2="+subId)

	if raw.Platform == "Android" {
		params = append(params, "google_aid="+ctx.Gaid)
		runeGaid := []rune(ctx.Gaid)
		if len(runeGaid) > 0 && int(runeGaid[0])%3 == 0 {
			params = append(params, "aff_sub4=cn.xender")
		} else {
			params = append(params, "aff_sub4=com.quvideo.XiaoYing")
		}
	} else {
		params = append(params, "ios_ifa="+ctx.Idfa)
		params = append(params, "aff_sub4=com.quvideo.XiaoYing")
	}

	sub5 := fmt.Sprintf("aff_sub5=country_%s,clkpayout_%0.3f,offerid_%s,idfa_%s,gaid_%s,aid_%s",
		ctx.Country, raw.Payout, raw.Id, ctx.Idfa, ctx.Gaid, ctx.Aid)
	params = append(params, sub5)

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}
	return trackURL
}

func (raw *RawAdObj) genShootMediaClkUrl(ctx *http_context.Context) string {
	trackURL := raw.AppDownload.TrackLink

	params := make([]string, 0, 6)

	params = append(params, "aff_sub="+ctx.SlotId)
	params = append(params, "gaid="+ctx.Gaid)
	params = append(params, "android_id="+ctx.Aid)
	params = append(params, "idfa="+ctx.Idfa)

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	params = append(params, fmt.Sprintf(
		"click=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg, raw.CreativeId()))

	params = append(params, "ch="+raw.Channel)

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}
	return trackURL
}

func (raw *RawAdObj) genMobcastleClkUrl(ctx *http_context.Context) string {
	clkUrl := raw.AppDownload.TrackLink

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))

	clickid := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country,
		ctx.Method, ctx.SdkVersion, ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(),
		ctx.Gaid, ctx.Aid, ctx.Idfa, base64PkgName, ctx.Platform, raw.Payout,
		base64OfferPkg, raw.CreativeId())

	clkUrl = strings.Replace(clkUrl, "{clickid}", clickid, 1)
	clkUrl = strings.Replace(clkUrl, "{subpubid}", ctx.SlotId, 1)
	clkUrl = strings.Replace(clkUrl, "{idfa}", ctx.Idfa, 1)

	return clkUrl + "&ch=" + raw.Channel
}

func (raw *RawAdObj) genSeaecClkUrl(ctx *http_context.Context) string {
	trackURL := raw.AppDownload.TrackLink

	params := make([]string, 0, 2)

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	params = append(params, fmt.Sprintf(
		"aff_sub2=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg, raw.CreativeId()))

	params = append(params, "ch="+raw.Channel)

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}
	return trackURL
}

func (raw *RawAdObj) genMediuminClkUrl(ctx *http_context.Context) string {
	trackURL := raw.AppDownload.TrackLink

	params := make([]string, 0, 5)

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	params = append(params, fmt.Sprintf(
		"aff_sub=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg, raw.CreativeId()))

	params = append(params, "sub_channel="+ctx.SlotId)
	if ctx.Platform == "Android" {
		params = append(params, "gaid="+ctx.Gaid)
	} else {
		params = append(params, "idfa="+ctx.Idfa)
	}

	params = append(params, "ch="+raw.Channel)

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}
	return trackURL
}

func (raw *RawAdObj) genBingmobClkUrl(ctx *http_context.Context) string {
	trackURL := raw.AppDownload.TrackLink

	params := make([]string, 0, 5)

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	params = append(params, fmt.Sprintf(
		"aff_sub1=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg, raw.CreativeId()))

	params = append(params, "source_id="+ctx.SlotId)
	if ctx.Platform == "Android" {
		params = append(params, "google_aid="+ctx.Gaid)
	} else {
		params = append(params, "ios_idfa="+ctx.Idfa)
	}

	params = append(params, "ch="+raw.Channel)

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}
	return trackURL
}

func (raw *RawAdObj) genSingleDogClkUrl(ctx *http_context.Context) string {
	trackURL := raw.AppDownload.TrackLink

	params := make([]string, 0, 4)

	base64PkgName := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(ctx.PkgName)))
	base64OfferPkg := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(raw.AppDownload.PkgName)))
	params = append(params, fmt.Sprintf("clickid=%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		base64PkgName, ctx.Platform, raw.Payout, base64OfferPkg))

	if ctx.Platform == "Android" {
		params = append(params, "gaid="+ctx.Gaid)
	} else {
		params = append(params, "idfa="+ctx.Idfa)
	}
	params = append(params, "sub_channel="+ctx.SlotId)

	params = append(params, "ch="+raw.Channel)

	args := strings.Join(params, "&")

	if strings.HasSuffix(trackURL, "&") {
		trackURL += args
	} else {
		trackURL += "&" + args
	}
	return trackURL
}

func adcamieGenClkUrl(raw *RawAdObj, ctx *http_context.Context) string {
	clickid := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		ctx.PkgName, ctx.Platform, raw.Payout, raw.AppDownload.PkgName)

	// 替换click_id
	clkurl := strings.Replace(raw.AppDownload.TrackLink,
		"{click_id}", url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(clickid))), 1)

	// 替换{publisher_id}
	clkurl = strings.Replace(clkurl, "{publisher_id}", ctx.SlotId, 1)

	return clkurl
}

func (raw *RawAdObj) genLeadboltClkUrl(ctx *http_context.Context) string {
	params := make([]string, 0, 4)

	clickid := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		ctx.PkgName, ctx.Platform, raw.Payout, raw.AppDownload.PkgName)

	params = append(params, "sid="+url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(clickid))))

	if ctx.Platform == "Android" {
		params = append(params, "devad_id="+ctx.Gaid)
	} else {
		params = append(params, "devad_id="+ctx.Idfa)
	}
	params = append(params, "gid="+ctx.SlotId)
	params = append(params, "ch="+raw.Channel)

	return raw.AppDownload.TrackLink + "?" + strings.Join(params, "&")
}

func genAppleadsClkUrl(raw *RawAdObj, ctx *http_context.Context) string {
	params := make([]string, 0, 4)

	clickid := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		ctx.PkgName, ctx.Platform, raw.Payout, raw.AppDownload.PkgName)

	params = append(params, "aff_sub1="+url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(clickid))))
	params = append(params, "source_id="+ctx.SlotId)
	if ctx.Platform == "Android" {
		params = append(params, "google_aid="+ctx.Gaid)
	} else {
		params = append(params, "ios_idfa="+ctx.Idfa)
	}
	params = append(params, "ch="+raw.Channel)

	return raw.AppDownload.TrackLink + "&" + strings.Join(params, "&")
}

func (raw *RawAdObj) genMobduosClkUrl(ctx *http_context.Context) string {
	params := make([]string, 0, 4)

	clickid := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%s_%s_%s_%s_%s_%.3f_%s",
		util.GenClickId(raw.Channel), raw.Id, ctx.SlotId, ctx.Country, ctx.Method, ctx.SdkVersion,
		ctx.Ran, ctx.ServerId, ctx.ImpId, ctx.Now.Unix(), ctx.Gaid, ctx.Aid, ctx.Idfa,
		ctx.PkgName, ctx.Platform, raw.Payout, raw.AppDownload.PkgName)

	if ctx.Platform == "Android" {
		params = append(params, "gaid="+ctx.Gaid)
	} else {
		params = append(params, "idfa="+ctx.Idfa)
	}

	params = append(params, "ext1="+clickid)
	params = append(params, "sub_id="+ctx.SlotId)
	params = append(params, "ch="+raw.Channel)

	return raw.AppDownload.TrackLink + "&" + strings.Join(params, "&")
}

func (raw *RawAdObj) genYeahmobiImpUrlArg(ctx *http_context.Context) string {
	impTrack := raw.ThirdPartyImpTks[0]
	if len(impTrack) == 0 {
		return ""
	}
	impTrack = fmt.Sprintf("%s&offer_id=%s&creative_id=%s", impTrack, raw.Id, raw.CreativeId())
	return impTrack
}

func (raw *RawAdObj) IsFuyuEnabled() bool  { return raw.FuyuEnabled }
func (raw *RawAdObj) IsJsTagEnabled() bool { return raw.JsTagEnabled }
func (raw *RawAdObj) IsWuganEnabled() bool { return raw.WuganEnabled }

func (raw *RawAdObj) AddToDnf(h *dnf.Handler) error {
	docId := raw.UniqId

	// 添加定向条件
	if len(raw.Countries) != 0 {
		for _, country := range raw.Countries {
			raw.AddTarget("country", country, true)
		}
		raw.AddTarget("country", "DEBUG", true)
	} else {
		return fmt.Errorf("has no country")
	}

	if len(raw.Regions) != 0 {
		for _, region := range raw.Regions {
			raw.AddTarget("region", strings.Replace(region, " ", "", -1), true)
		}
		raw.AddTarget("region", "DEBUG", true)
	}

	if len(raw.Cities) != 0 {
		for _, city := range raw.Cities {
			raw.AddTarget("city", strings.Replace(city, " ", "", -1), true)
		}
		raw.AddTarget("city", "DEBUG", true)
	}

	if strings.ToLower(raw.Platform) == "android" {
		raw.AddTarget("platform", "Android", true)
	} else if strings.ToLower(raw.Platform) == "ios" {
		raw.AddTarget("platform", "iOS", true)
	} else {
		return fmt.Errorf("unknow platform")
	}

	if raw.Network == 1 { // wifi
		raw.AddTarget("network", "wifi", true)
	} else if raw.Network == 2 { // unwifi
		raw.AddTarget("network", "mobile", true)
	}

	// os version (仅定向主版本号)
	if len(raw.Versions) > 0 {
		versions := make(map[int]bool, 4)
		for _, osv := range raw.Versions {
			major, _, _ := util.GetOsVersion(osv)
			versions[major] = true
		}
		for version := range versions {
			raw.AddTarget("version", strconv.Itoa(version), true)
		}
		raw.AddTarget("version", "any", true)
	}

	// device
	if len(raw.SuppDevices) > 0 {
		for _, device := range raw.SuppDevices {
			raw.AddTarget("device", device, true)
		}
		raw.AddTarget("device", "any", true)
	}

	// 黑名单过滤
	if raw.BlackPkg {
		return fmt.Errorf("[AddDnf] pkg black offer: %s pkg: %s", docId, raw.AppDownload.PkgName)
	}

	if raw.TempBlack == 1 { // 1: temp black
		log.Println("add ", docId, " to temp black")
		raw.AddTarget("pmt", "true", true)
		raw.genDnf()
	} else if raw.TempBlack == 2 { // 2: permanent black
		return fmt.Errorf("black offer: %s", docId)
	}

	// 预处理
	raw.PreProcess()

	if err := h.AddDoc("", docId, raw.Dnf, raw); err != nil {
		return fmt.Errorf("%v, dnf: %v", err, raw.Dnf)
	}

	return nil
}
