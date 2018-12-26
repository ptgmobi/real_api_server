package http_context

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/brg-liuwei/gotools"

	"aes"
	"cache"
	"ct_bloom"
	"set"
	"ssp"
	"util"
)

type printer interface {
	Println(...interface{})
}

var possiableArgs []string = []string{
	"msv",
	"country",
	"pn",
	"user_id",
	"ran",
	"platform",
	"gaid",
	"tz",
	"ck",
	"dt",
	"nt",
	"aid",
	"idfa",
	"imei",
	"ip",
	"icc",
	"gp",
	"dmf",
	"dml",
	"dpd",
	"so",
	"ds",
	"mcc",
	"mnc",
	"cn",
	"la",
	"lo",
	"ua",
	"srnc",
	"carrier",
	"f",     // f: n表示后截来自通知栏，b表示后截来自系统广播
	"n",     // n: 1表示该设备能拿到通知栏消息，2表示不能拿到通知栏消息
	"ispre", // 视频素材是否是初始化阶段拉取
	"lang",  // 手机语言
}

var keywordsReg *regexp.Regexp

// 为汽车之家做的修改
var btnTextSlot1 map[string]bool = map[string]bool{
	"87065741": true,
	"23367373": true,
	"15101879": true,
}

var btnTextSlot2 map[string]bool = map[string]bool{
	"78519999": true,
	"68572006": true,
}

func init() {
	keywordsReg = regexp.MustCompile("[\\s!\\-&:]")
}

type Context struct {
	r    *http.Request
	L    printer
	es   *gotools.Estimate
	form *url.Values

	Now                time.Time
	UseAes             bool
	UseGzip            bool
	UseGzipCT          bool
	RankUseGzip        bool
	TestEncode         string
	FreqDaysInRedis    float32 // 广告频次周期
	PreFreqDaysInRedis float32 // 预加载频次周期
	Phase              string  // 当前req阶段
	FuyuPhase          string  // fuyu req phase

	// get from url parameter
	IsDebug        bool
	IsDemsg        bool
	AdNum          int
	InstalledPkg   string
	ApiVersion     string
	IntegralWall   bool // 积分墙(应用墙)
	SlotId         string
	AdType         string
	DetailReqType  string // 详细的请求类型，如jsta有实时api，h5等
	CreativeType   string
	Platform       string // iOS or Android
	Osv            string // os version
	Version        string // version of iOS or Android
	Device         string // device phone, ipad, tablet
	Country        string
	Region         string
	City           string
	Carrier        string
	Channel        string
	Ck             string // cookie
	Lang           string
	ImgRule        int
	FreqInfo       FreqInfo
	UserId         string
	FuyuUserId     string
	UserHash       int      // range: [0, 1000)
	PkgName        string   // 媒体app包名
	AdPkgName      []string // 推广的offer的app包名
	FuyuPrePkgName []string // fuyu pre_click pkgname
	Ran            string
	Gaid           string
	GaidMd5        string
	GaidSha1       string
	Idfa           string
	IdfaMd5        string
	IdfaSha1       string
	Aid            string
	AidMd5         string
	AidSha1        string
	SdkVersion     string
	AdCat          string
	NotifyFrom     string // n: 表示后截来自于通知栏，b:表示后截来自系统
	Note           map[string]string
	Subs           bool // 是否支持订阅广告 1: 支持

	Match    string
	KeyWords []string

	AutoTks  bool
	ServerId string
	Method   string

	// imgw and imgh, from template when sdk-ad and from url params from native-ad
	ImgH    int
	ImgW    int
	ScreenH int
	ScreenW int

	// video screen type 1: 横屏 2: 竖屏
	VideoScreenType int
	VideoCacheNum   int // 视频缓冲数

	CidMap       map[string]int // 存放在客户端的视频v4素材id
	ServerCidKey string         // 存放在服务端的素材播放次数key: cfreq_user
	ServerCidMap map[string]int // 存放在服务端的素材播放次数map: cfreq_user -> creative_display_freq

	// get from template
	Template   *ssp.TemplateObj
	H5tpl      []byte
	ButtonText string

	ImpId      string
	ReqId      string
	PreClick   bool
	PreImpTks  []string
	PreClkTks  []string
	PostImpTks []string
	PostClkTks []string

	PostImpTksDebug []string
	PostClkTksDebug []string

	Fake bool

	Nrtv int // retrieval ad num
	Nfy  int // fuyu ad num

	PossiableArgs string

	CtBF *ct_bloom.BloomFilter
	NgBF *ct_bloom.BloomFilter

	IP      string
	UA      string
	Network string // ios: [3: mobile, 5:wifi, 0:没网] android: [0: mobile, 1: wifi](产品定义为int字段但是用了0表示状态，这里还是用string方便区分)
	Mcc     string
	Mnc     string

	IosConvKey string   // iCONV_user
	ConvPkgSet *set.Set // 来自转化日志的user installed pkg集合

	LoadTime  int // https://github.com/cloudadrd/offer_server/issues/493
	ClickTime int

	// debug调试记录信息
	demsg map[string][]string

	// 插屏广告
	VastServerApi string
	// Vast JS脚本地址
	VastJsUrl string

	// pagead 需要的参数
	PageadObj

	SubChannel string
}

type PageadObj struct {
	AdW       int    // 广告宽
	AdH       int    // 广告高
	Output    string // 输出格式，默认json
	ReqOrigin string // 请求来源

	Format        string      // 匹配广告类型
	Control       interface{} // 控制数据
	Adapters      []string    // c_wxh, 模板适配器
	StaticBaseUrl string      // 静态资源存储路径
}

type FreqInfo struct {
	FreqKey string         // 正常频控: user -> pkg -> value
	FreqMap map[string]int // offer_id ==> user_offer_display_cnt

	PFreqKey    string         // (wugan)服预频控: user -> pkg -> value
	PFreqFields []string       // pre_click pkgname or offer_id
	PFreqMap    map[string]int // pre_click freq map

	VCompleteKey string // 激励视频complete频控: vfreq_user_adtype -> slot -> complete
	VCompletes   int    //激励视频complete次数
	VRequestKey  string //激励视频request频控: req_user -> slot -> request
	VRequests    int    //激励视频request次数
	VFreqField   string // slot with prefix
}

func (ctx *Context) get(key string) string {
	return ctx.form.Get(key)
}

func (ctx *Context) Params(key string) string {
	return ctx.get(key)
}

type reflectHelper struct {
	fieldKey string
	reqKey   string
}

var refHelperSlice []reflectHelper = []reflectHelper{
	{"Version", "version"},
	{"Ck", "ck"},
	{"Ran", "ran"},
	{"Gaid", "gaid"},
	{"Idfa", "idfa"},
	{"Aid", "aid"},
	{"Method", "method"},
	{"AdCat", "adcat"},
	{"ServerId", "server_id"},
	{"FuyuUserId", "fuyu_user_id"},
	{"Channel", "channel"},
	{"Platform", "platform"},
	{"Osv", "osv"},
	{"SdkVersion", "sv"},
	{"IP", "ip"},
	{"TestEncode", "test_encode"},
	{"Mcc", "mcc"},
	{"Mnc", "mnc"},
}

func (ctx *Context) initByReflectString(slice []reflectHelper) {
	refCtx := reflect.ValueOf(ctx)
	elem := refCtx.Elem()

	for _, v := range slice {
		val := elem.FieldByName(v.fieldKey)
		if val.IsValid() {
			val.SetString(ctx.get(v.reqKey))
		} else {
			panic("fieldKey error: " + v.fieldKey)
		}
	}
}

type ctxParamHandler func(ctx *Context) error

var ctxInitHelper map[string]ctxParamHandler = map[string]ctxParamHandler{
	"req_id": func(ctx *Context) error {
		ctx.ReqId = util.Rand8Bytes()
		return nil
	},
	"auto_tks": func(ctx *Context) error {
		if ctx.get("auto_tks") == "1" {
			ctx.AutoTks = true
		}
		return nil
	},
	"adnum": func(ctx *Context) error {
		ctx.AdNum, _ = strconv.Atoi(ctx.get("adnum"))
		if ctx.AdNum <= 0 {
			ctx.AdNum = 1
		}
		return nil
	},
	"isdebug": func(ctx *Context) error {
		if ctx.get("isdebug") == "2" {
			ctx.IsDebug = true
			if ctx.get("isdemsg") == "1" {
				ctx.IsDemsg = true
			}
		}
		return nil
	},
	"os": func(ctx *Context) error {
		if ctx.Platform == "" {
			ctx.Platform = ctx.get("os")
		}
		return nil
	},
	"country": func(ctx *Context) error {
		ctx.Country = ctx.get("country")
		ctx.FreqDaysInRedis = 1
		ctx.PreFreqDaysInRedis = 1

		var icc = ""
		if strings.HasPrefix(strings.ToLower(ctx.get("lang")), "zh") {
			icc = "CN"
		} else {
			icc = strings.ToUpper(ctx.get("icc"))
		}
		if len(icc) != 2 {
			icc = ctx.Country
		}

		ctx.Region = util.NormalizeRegion(ctx.get("region"))
		ctx.City = util.NormalizeRegion(ctx.get("city"))

		switch icc {
		case "CN":
			slot := ctx.get("slot_id")
			if btnTextSlot1[slot] {
				ctx.ButtonText = "立即下载"
			} else if btnTextSlot2[slot] {
				ctx.ButtonText = "下载"
			} else {
				ctx.ButtonText = "立刻查看"
			}
		case "TW":
			ctx.ButtonText = "點擊查看"
		case "RU":
			ctx.ButtonText = "подробности"
		case "JP":
			ctx.ButtonText = "ビュー"
		case "ID":
			ctx.ButtonText = "Free Download"
		default:
			ctx.ButtonText = "Install Now"
		}
		return nil
	},
	"use_gzip": func(ctx *Context) error {
		if strings.Contains(ctx.r.Header.Get("Accept-Encoding"), "gzip") {
			ctx.UseGzip = true
		}
		if ctx.r.Header.Get("CT-Accept-Encoding") == "gzip" { // v3 gzip
			ctx.UseGzipCT = true
		}
		return nil
	},
	"slot_id": func(ctx *Context) error {
		ctx.SlotId = ctx.get("slot_id")
		if len(ctx.SlotId) == 0 {
			ctx.SlotId = ctx.get("token")
			if ctx.SlotId == "" {
				return errors.New("url parameter slot_id empty")
			}
		}
		ctx.AdType = ctx.get("adtype")
		ctx.CreativeType = ctx.get("creativetype")
		return nil
	},
	"f": func(ctx *Context) error {
		p := ctx.get("platform")
		ctx.NotifyFrom = ctx.get("f")
		if len(ctx.NotifyFrom) == 0 {
			if p == "iOS" {
				ctx.NotifyFrom = "c" // ios的都算是半后劫
			} else {
				ctx.NotifyFrom = "b" // 先前版本没有该参数则都来自系统
			}
		}
		return nil
	},
	"dml": func(ctx *Context) error {
		dmlStr, _ := url.QueryUnescape(ctx.get("dml"))
		dpdStr, _ := url.QueryUnescape(ctx.get("dpd"))
		if strings.Contains(dmlStr, "Galaxy J7") || strings.Contains(dpdStr, "Galaxy J7") {
			return fmt.Errorf("Android don't support Galaxy J7 device")
		}
		return nil
	},
	"lang": func(ctx *Context) error {
		ctx.Lang = ctx.get("lang")
		if len(ctx.Lang) == 0 {
			ctx.Lang = ctx.get("country")
		}
		if len(ctx.Lang) == 0 {
			ctx.Lang = "ALL"
		}
		return nil
	},
	"img_rule": func(ctx *Context) error {
		// imgRule:
		//    -2 - 比值浮动不超过10%
		//    -1 - 比值完全匹配即可
		//     1 - 宽高值绝对匹配
		//     2 - 宽高比值绝对匹配，宽高本身可上下浮动20%
		//     3 - 宽高比值上下浮动10%，宽高本身可上下浮动20%
		ctx.ImgRule, _ = strconv.Atoi(ctx.get("img_rule"))
		if ctx.ImgRule == 0 {
			ctx.ImgRule = -2
		}
		return nil
	},
	"user_id": func(ctx *Context) error {
		ctx.UserId = ctx.get("user_id")
		var err error
		userHash := ctx.get("user_hash")
		if userHash == "" {
			userHash = util.GetUserHash(ctx.UserId)
		}
		ctx.UserHash, err = strconv.Atoi(userHash)
		if err != nil {
			ctx.L.Println("user_hash Atoi error, user_hash: ", ctx.get("user_hash"))
			return err
		}
		return nil
	},
	"possiable_args": func(ctx *Context) error {
		args := make([]string, 0, len(possiableArgs))
		for _, k := range possiableArgs {
			v := ctx.get(k)
			if len(v) != 0 {
				if k == "pn" {
					v = strings.Replace(v, "_", "-", -1)
				}
				args = append(args, k+"="+url.QueryEscape(v))
			}
		}
		ctx.PossiableArgs = strings.Join(args, "&")
		return nil
	},
	"imgw": func(ctx *Context) error {
		imgW, imgH := ctx.get("imgw"), ctx.get("imgh")
		ctx.ImgW, _ = strconv.Atoi(imgW)
		ctx.ImgH, _ = strconv.Atoi(imgH)
		return nil
	},
	"screen_w": func(ctx *Context) error {
		screenW, screenH := ctx.get("screen_w"), ctx.get("screen_h")
		ctx.ScreenW, _ = strconv.Atoi(screenW)
		ctx.ScreenH, _ = strconv.Atoi(screenH)
		return nil
	},
	"integral_wall": func(ctx *Context) error {
		integralWall := ctx.get("integral_wall")
		ctx.IntegralWall = (integralWall == "1")
		return nil
	},
	"installed_pkg": func(ctx *Context) error {
		ctx.ApiVersion = ctx.get("api_version")
		ctx.UseAes = ctx.ApiVersion != "v1"

		installedPkg := ctx.get("spn")
		if len(installedPkg) != 0 {
			if ctx.UseAes {
				ctx.InstalledPkg = strings.ToLower(aes.Decrypt(installedPkg))
			} else {
				ctx.InstalledPkg = strings.ToLower(installedPkg)
			}
		}
		return nil
	},
	"note": func(ctx *Context) error {
		noteAes := ctx.get("note")
		if len(noteAes) != 0 {
			note := aes.Decrypt(noteAes)
			if jsonErr := json.Unmarshal([]byte(note), &ctx.Note); jsonErr != nil {
				return errors.New("Unmarshal note err: " + jsonErr.Error() + " note: " + note)
			}
		}
		return nil
	},
	"ct_bloom_filter": func(ctx *Context) error {
		base64str := ctx.get("ctbf")
		if len(base64str) != 0 {
			ctx.CtBF = ct_bloom.NewBloomFilter(base64str)
			if ctx.CtBF == nil {
				ctx.L.Println("create bloom error: ", base64str, ", uid: ", ctx.get("user_id"))
			}
		}
		return nil
	},
	"ngp_bloom_filter": func(ctx *Context) error {
		base64str := ctx.get("ngbf")
		if len(base64str) != 0 {
			ctx.NgBF = ct_bloom.NewBloomFilter(base64str)
			if ctx.NgBF == nil {
				ctx.L.Println("create ngp bloom error: ", base64str, ", uid: ", ctx.get("user_id"))
			}
		}
		return nil
	},
	"keywords": func(ctx *Context) error {
		if kw := ctx.get("keywords"); len(kw) != 0 {
			ctx.KeyWords = strings.Split(strings.ToLower(kw), ",")
			for i := 0; i != len(ctx.KeyWords); i++ {
				ctx.KeyWords[i] = keywordsReg.ReplaceAllLiteralString(ctx.KeyWords[i], "")
			}
			ctx.Match = strings.ToLower(ctx.get("match"))
		}
		return nil
	},
	"pn": func(ctx *Context) error {
		ctx.PkgName = strings.Replace(ctx.get("pn"), "_", "-", -1)
		return nil
	},
	"ua": func(ctx *Context) error {
		rawUa := ctx.get("ua")
		if util.IsBase64(rawUa) {
			if uaBytes, err := base64.StdEncoding.DecodeString(rawUa); err == nil {
				ctx.UA = string(uaBytes)
			}
		} else {
			if ua, err := url.QueryUnescape(rawUa); err == nil {
				ctx.UA = ua
			} else {
				ctx.UA = rawUa
			}
		}
		return nil
	},
	"carrier": func(ctx *Context) error {
		rawCarrier := ctx.get("carrier")
		if util.IsBase64(rawCarrier) {
			if carrierBytes, err := base64.StdEncoding.DecodeString(rawCarrier); err == nil {
				ctx.Carrier = strings.ToLower(string(carrierBytes))
			}
		} else {
			ctx.Carrier = string(rawCarrier)
		}
		return nil
	},
	"cids": func(ctx *Context) error {
		cids := strings.Split(ctx.get("cids"), ",")
		ncid := len(cids)
		cidMap := make(map[string]int, ncid/2)
		for i := 0; i < ncid-1; i += 2 {
			times, _ := strconv.Atoi(cids[i+1])
			cidMap[cids[i]] = times
		}
		ctx.CidMap = cidMap
		return nil
	},
	"nt": func(ctx *Context) error {
		nt := ctx.get("nt")
		if len(nt) > 0 {
			if _, err := strconv.Atoi(nt); err != nil {
				ctx.L.Println("http_context get nt err: ", err, " nt: ", nt)
			} else {
				ctx.Network = nt
			}
		}
		return nil
	},
	"subs": func(ctx *Context) error {
		subs := ctx.get("subs")
		if len(subs) > 0 {
			if subs == "1" {
				ctx.Subs = true
			}
		}
		return nil
	},
	"dt": func(ctx *Context) error {
		dt := ctx.get("dt")
		if dt == "phone" || dt == "ipad" || dt == "tablet" {
			ctx.Device = dt
		}
		return nil
	},
	// pagead
	"adw": func(ctx *Context) error {
		adw, adh := ctx.get("ad_w"), ctx.get("ad_h")
		ctx.AdW, _ = strconv.Atoi(adw)
		ctx.AdH, _ = strconv.Atoi(adh)
		return nil
	},
	"output": func(ctx *Context) error {
		ctx.Output = ctx.get("output")
		if ctx.Output == "" {
			ctx.Output = "json" // 默认json
		}
		return nil
	},
	"req_origin": func(ctx *Context) error {
		ctx.ReqOrigin = ctx.get("req_origin")
		if ctx.ReqOrigin == "" {
			ctx.ReqOrigin = "sdk" // 默认sdk
		}
		return nil
	},
}

func NewContext(r *http.Request, pr printer) (*Context, error) {
	var form *url.Values
	r.ParseForm()
	if r.Method == "GET" {
		form = &r.Form
	} else if r.Method == "POST" {
		form = &r.PostForm
	} else {
		return nil, errors.New("unexpected method: " + r.Method)
	}

	ctx := &Context{
		r:     r,
		L:     pr,
		form:  form,
		Now:   time.Now().UTC(),
		Phase: "Un-inited",
	}

	for k, f := range ctxInitHelper {
		if err := f(ctx); err != nil {
			ctx.L.Println("init context error, key: ", k, ", errmsg: ", err,
				", requestURI: ", r.URL.RequestURI(), "=>", form.Encode())
			return nil, err
		}
	}

	ctx.initByReflectString(refHelperSlice)

	if len(ctx.Aid) != 0 {
		ctx.AidMd5 = fmt.Sprintf("%x", md5.Sum([]byte(ctx.Aid)))
		ctx.AidSha1 = fmt.Sprintf("%x", sha1.Sum([]byte(ctx.Aid)))
	}

	if len(ctx.Idfa) != 0 {
		ctx.IdfaMd5 = fmt.Sprintf("%x", md5.Sum([]byte(ctx.Idfa)))
		ctx.IdfaSha1 = fmt.Sprintf("%x", sha1.Sum([]byte(ctx.Idfa)))
	}

	if len(ctx.Gaid) != 0 {
		ctx.GaidMd5 = fmt.Sprintf("%x", md5.Sum([]byte(ctx.Gaid)))
		ctx.GaidSha1 = fmt.Sprintf("%x", sha1.Sum([]byte(ctx.Gaid)))
	}

	if rand.Intn(1000) < 1 {
		ctx.es = gotools.NewEstimate("[" + ctx.SlotId + "]")
		ctx.es.SetLogger(pr)
	}

	if ctx.Platform == "iOS" {
		ctx.IosConvKey = util.StrJoinUnderline("iCONV", ctx.UserId)
		ctx.ConvPkgSet = set.NewSet()
	}

	ctx.ServerCidKey = util.StrJoinUnderline("cfreq", ctx.UserId)
	ctx.ServerCidMap = make(map[string]int)

	ctx.FreqInfo.FreqKey = util.StrJoinUnderline("freq", ctx.UserId, ctx.AdType)
	ctx.FreqInfo.FreqMap = make(map[string]int)
	ctx.FreqInfo.PFreqKey = util.StrJoinUnderline("pfreq", ctx.UserId)
	ctx.FreqInfo.PFreqMap = make(map[string]int)

	ctx.FreqInfo.VCompleteKey = util.StrJoinUnderline("vfreq", ctx.UserId, ctx.AdType)
	ctx.FreqInfo.VRequestKey = util.StrJoinUnderline("req", ctx.UserId)
	ctx.FreqInfo.VFreqField = util.StrJoinUnderline("slot", ctx.SlotId)

	// XXX AndroidSDK打包图片匹配规则填错，统一改为-2
	if ctx.Platform == "Android" && ctx.ImgRule == 3 {
		ctx.ImgRule = -2
	}

	return ctx, nil
}

func (ctx *Context) GetRawPath() string {
	return "/get_subs_ad?" + ctx.form.Encode() + "&subChannel=" + ctx.SubChannel
}

func (ctx *Context) Estimate(phase string) {
	if ctx.es != nil {
		ctx.es.Add(phase)
	}
}

func (ctx *Context) LogEstimate() {
	if ctx.es != nil {
		ctx.es.Write()
	}
}

// 用户总曝光频控
func (ctx *Context) UserFreqCap(limit int) bool {
	if ctx.IntegralWall {
		return true
	}

	if ctx.FreqInfo.FreqMap == nil {
		return true
	}

	var cnt int
	for _, num := range ctx.FreqInfo.FreqMap {
		cnt += num
	}

	if cnt < limit {
		return true
	}

	return false
}

func (ctx *Context) InFreqCap(key string, limitation int) (inCap, isFirst bool) {
	if ctx.IntegralWall {
		// 应用墙不做频控
		return true, true
	}
	if ctx.FreqInfo.FreqMap != nil {
		if cnt, ok := ctx.FreqInfo.FreqMap[key]; ok {
			if cnt >= limitation {
				return false, false
			}
			return true, false
		}
	}
	return true, true
}

func (ctx *Context) PreClickInFreqCap(key string, limitation int) bool {
	if ctx.FreqInfo.PFreqMap != nil {
		if cnt, ok := ctx.FreqInfo.PFreqMap[key]; ok {
			return cnt < limitation
		}
		return true
	}
	return true
}

func (ctx *Context) VideoInCompleteCap(limit int) bool {
	if limit == -1 {
		return true
	}
	if limit == 0 {
		return false
	}

	return ctx.FreqInfo.VCompletes < limit
}

func (ctx *Context) VideoInRequestCap() bool {
	return true //暂时停用该功能，媒体那边反馈量级上不去，影响正常用户的填充
	//return ctx.SlotId != "14326496" || ctx.FreqInfo.VRequests < 10
}

func (ctx *Context) IncrPreClickFreq() error {
	if ctx.IntegralWall || len(ctx.FreqInfo.PFreqFields) == 0 {
		// 积分墙不做频次控制
		return nil
	}

	var expire int64
	if ctx.Country == "CN" && ctx.Platform == "iOS" {
		expire = 43200 - time.Now().Unix()%43200
	} else {
		expire = 86400 - time.Now().Unix()%86400
	}
	return cache.IncrFreq(ctx.UserHash, ctx.FreqInfo.PFreqKey, ctx.FreqInfo.PFreqFields, expire)
}

func (ctx *Context) GetFreq() error {
	var err error
	// XXX 兼容以前版本，adtype可能在之后会修改，再次生成频控key
	ctx.FreqInfo.FreqKey = util.StrJoinUnderline("freq", ctx.UserId, ctx.AdType)

	if ctx.IsWugan() {
		ctx.FreqInfo.PFreqMap, err = cache.HGetAllFreq(ctx.UserHash, ctx.FreqInfo.PFreqKey)
	} else if ctx.IsRewardedVideo() {
		ctx.ServerCidMap, ctx.ConvPkgSet, ctx.FreqInfo.VCompletes, ctx.FreqInfo.VRequests, err = cache.GetAndSetVideoCtrlInfos(ctx.UserHash, ctx.ServerCidKey, ctx.IosConvKey, ctx.FreqInfo.VCompleteKey, ctx.FreqInfo.VRequestKey, ctx.FreqInfo.VFreqField)
	} else {
		ctx.FreqInfo.FreqMap, err = cache.HGetAllFreq(ctx.UserHash, ctx.FreqInfo.FreqKey)
	}
	return err
}

func (ctx *Context) IsIosInstalledPkg(pkg string) bool {
	if ctx.ConvPkgSet == nil {
		return false
	}
	return ctx.Platform == "iOS" && ctx.ConvPkgSet.Size() > 0 && ctx.ConvPkgSet.Has(pkg)
}

func (ctx *Context) IsRewardedVideo() bool {
	return ctx.AdType == "7"
}

func (ctx *Context) IsWugan() bool {
	return ctx.AdType == "3" || ctx.AdType == "4" || ctx.AdType == "9"
}

func (ctx *Context) IsWifi() bool {
	if ctx.Platform == "Android" && ctx.get("nt") == "1" {
		return true
	}
	if ctx.Platform == "iOS" && ctx.get("nt") == "5" {
		return true
	}
	return false
}

func (ctx *Context) IsPromote() bool {
	return ctx.AdType == "8"
}

func (ctx *Context) IsVastInOfferServer() bool {
	return (ctx.Platform == "Android" && ctx.SdkVersion >= "a-2.1.0") ||
		(ctx.Platform == "iOS" && ctx.SdkVersion >= "i-2.2.0")
}

func (ctx *Context) CreativeCdnConv(url, domesticCdn string) string {
	if ctx.Country == "CN" && len(domesticCdn) > 0 {
		return domesticCdn
	}
	return url
}

// 展示广告使用imp_rank接口
func (ctx *Context) ImpRank() bool {
	return ctx.AdType == "0" || ctx.AdType == "1" || ctx.AdType == "2" ||
		ctx.AdType == "6" || ctx.AdType == "16" || ctx.AdType == "17"
}

// interstitial 插屏广告
func (ctx *Context) IsVideoTpl() bool {
	if ctx.Template == nil {
		return false
	}

	if ctx.Template.StyleType == 11 || ctx.Template.StyleType == 12 {
		return true
	}

	return false
}

// pagead 根据output选择模板
func (ctx *Context) PageadTpl() *template.Template {
	if ctx.Output == "html" {
		return ssp.HtmlTpl
	}
	return ssp.MraidTpl
}

func (ctx *Context) Debug(id, msg string) {
	if ctx.IsDemsg && msg != "" {
		if ctx.demsg == nil {
			ctx.demsg = make(map[string][]string)
		}
		if _, ok := ctx.demsg[msg]; !ok {
			ctx.demsg[msg] = make([]string, 0, 8)
		}
		ctx.demsg[msg] = append(ctx.demsg[msg], id)
	}
}

func (ctx *Context) GetDemsg() interface{} {
	if !ctx.IsDemsg {
		return nil
	}
	return ctx.demsg
}
