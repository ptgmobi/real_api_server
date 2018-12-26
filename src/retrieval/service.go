package retrieval

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	dnf "github.com/brg-liuwei/godnf"
	"github.com/brg-liuwei/gotools"

	"http_context"
	common "offer"
	"pacing"
	"raw_ad"
	"ssp"
	"util"
)

var (
	rankUseGzipCap = 100
	rankLimitCap   = 1000
)

var possiableArgs []string = []string{
	"country",
	"city",
	"pn",
	"user_id",
	"ran",
	"platform",
	"gaid",
	"tz",
	"ck",
	"osv",
	"dt",
	"nt",
	"sv",
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
}

type Conf struct {
	ServerName       string `json:"server_name"` // default: "0.0.0.0"
	ListenPort       int    `json:"listen_port"`
	Path             string `json:"path"`
	NativePath       string `json:"native_path"`
	RealtimePath     string `json:"realtime_path"`
	PromotionPath    string `json:"promotion_path"`
	VideoPath        string `json:"video_path"`
	WuganPath        string `json:"wugan_path"`
	FunnyPath        string `json:"funny_path"`
	InmobiPath       string `json:"inmobi_path"`
	JstagPath        string `json:"jstag_path"`
	JstagH5Path      string `json:"jstag_h5_path"`
	NgpPath          string `json:"ngp_path"`
	AppWallPath      string `json:"app_wall_path"`
	InterstitialPath string `json:"interstitial_path"`
	PageadPath       string `json:"pagead_path"`
	LifePath         string `json:"life_path"`
	JstagMediaPath   string `json:"jstag_media_path"`

	LogPath        string `json:"log_path"`
	LogRotateNum   int    `json:"log_rotate_backup"`
	LogRotateLines int    `json:"log_rotate_lines"`

	TplHost string `json:"template_host"`
	TplPort int    `json:"template_port"`
	TplPath string `json:"template_path"`

	IosPreImpTks  []string `json:"ios_pre_imp_tks"`
	IosPreClkTks  []string `json:"ios_pre_clk_tks"`
	IosPostImpTks []string `json:"ios_post_imp_tks"`
	IosPostClkTks []string `json:"ios_post_clk_tks"`

	AndroidPreImpTks  []string `json:"android_pre_imp_tks"`
	AndroidPreClkTks  []string `json:"android_pre_clk_tks"`
	AndroidPostImpTks []string `json:"android_post_imp_tks"`
	AndroidPostClkTks []string `json:"android_post_clk_tks"`

	IosVideoPreImpTks  []string `json:"ios_video_pre_imp_tks"`
	IosVideoPreClkTks  []string `json:"ios_video_pre_clk_tks"`
	IosVideoPostImpTks []string `json:"ios_video_post_imp_tks"`
	IosVideoPostClkTks []string `json:"ios_video_post_clk_tks"`

	AndroidVideoPreImpTks  []string `json:"android_video_pre_imp_tks"`
	AndroidVideoPreClkTks  []string `json:"android_video_pre_clk_tks"`
	AndroidVideoPostImpTks []string `json:"android_video_post_imp_tks"`
	AndroidVideoPostClkTks []string `json:"android_video_post_clk_tks"`

	ForceStartSlots []string `json:"promote_start_slots"`

	AutoscalingGroupName string `json:"autoscaling_group_name"`
	AutoscalingRegion    string `json:"autoscaling_group_region"`

	NgpServerApi        string `json:"ngp_server_api"`
	VastServerApi       string `json:"vast_server_api"`
	VastJsUrl           string `json:"vast_js_url"`
	VastJsUrlCN         string `json:"vast_js_url_cn"`
	PageadStaticBaseUrl string `json:"pagead_static_base_url"`
}

type Service struct {
	conf   *Conf
	l      *gotools.RotateLogger
	stat   Statistic
	pc     unsafe.Pointer // pacing controller
	fuyuPc unsafe.Pointer // fuyu pacing controller

}

func NewService(conf *Conf) (*Service, error) {
	if len(conf.ServerName) == 0 {
		conf.ServerName = "0.0.0.0"
	}
	l, err := gotools.NewRotateLogger(conf.LogPath,
		"[RETRIEVAL] ", log.LstdFlags|log.LUTC, conf.LogRotateNum)
	if err != nil {
		return nil, errors.New("[RETRIEVAL] NewRotateLogger failed: " + err.Error())
	}
	l.SetLineRotate(conf.LogRotateLines)

	svc := &Service{
		conf:   conf,
		l:      l,
		pc:     unsafe.Pointer(pacing.NewPacingController(conf.AutoscalingGroupName, conf.AutoscalingRegion)),
		fuyuPc: unsafe.Pointer(pacing.NewPacingController(conf.AutoscalingGroupName, conf.AutoscalingRegion)),
	}

	go func() {
		for range time.NewTicker(15 * time.Minute).C {
			svc.StorePacing(pacing.NewPacingController(conf.AutoscalingGroupName, conf.AutoscalingRegion))
			svc.StoreFuyuPacing(pacing.NewPacingController(conf.AutoscalingGroupName, conf.AutoscalingRegion))
		}
	}()

	return svc, nil
}

func (s *Service) LoadPacing() *pacing.PacingController {
	return (*pacing.PacingController)(atomic.LoadPointer(&s.pc))
}

func (s *Service) LoadFuyuPacing() *pacing.PacingController {
	return (*pacing.PacingController)(atomic.LoadPointer(&s.fuyuPc))
}

func (s *Service) StorePacing(pc *pacing.PacingController) {
	atomic.StorePointer(&s.pc, unsafe.Pointer(pc))
}

func (s *Service) StoreFuyuPacing(pc *pacing.PacingController) {
	atomic.StorePointer(&s.fuyuPc, unsafe.Pointer(pc))
}

func (s *Service) getTpl(slotId string) *ssp.SlotInfo {
	slotStore := ssp.GetGlobalSlotStore()
	if slotStore == nil {
		return nil
	}
	return slotStore.Get(slotId)
}

func (s *Service) getVideoTplAndUpdateSlot(ctx *http_context.Context) *ssp.SlotInfo {
	slotStore := ssp.GetGlobalSlotStore()
	if slotStore == nil {
		return nil
	}
	info := slotStore.GetVideoSlot(ctx.SlotId)
	if info != nil {
		ctx.SlotId = info.IdStr
	}
	return info
}

// 广告通用搜索条件
func (s *Service) adSearch(raw *raw_ad.RawAdObj, ctx *http_context.Context, tpl *ssp.SlotInfo) (string, bool) {
	// debug
	if ctx.IsDebug {
		if ctx.Channel != "" && ctx.Channel != raw.Channel {
			return "", false
		}

		if ctx.Params("offerid") != "" && ctx.Params("offerid") != raw.Id {
			return "", false
		}
	}

	// black channel
	if tpl.AdNetworkBlackMap != nil && tpl.AdNetworkBlackMap[raw.Channel] {
		return "black channel", false
	}

	// 广告类型过滤（目前有googleplaydownload, subscription, ddl三类）
	if !tpl.InWhiteList(raw.ProductCategory) {
		return "not hit product category(gpd, sub, ddl)", false
	}

	// slot white 一些offer只投某些slot
	if !raw.IsHitSlot(ctx.SlotId) {
		return "not hit white slot", false
	}

	// slot 层级offer黑名单
	if tpl.OfferBlackMap != nil && tpl.OfferBlackMap[raw.UniqId] {
		return "offer black", false
	}

	// 无感没有这三个
	if !ctx.IsWugan() {
		// white channel
		if tpl.AdNetworkWhiteMap != nil && !tpl.AdNetworkWhiteMap[raw.Channel] {
			return "not hit white channel", false
		}

		// 包名黑名单, 无感没有改功能
		if tpl.PkgBlackMap != nil && tpl.PkgBlackMap[raw.AppDownload.PkgName] {
			return "pkg black", false
		}

		// 渠道流量比控制, 无感没有
		if rand.Float64() > raw.TrafficRate {
			return "traffic rate", false
		}
	}

	// 布隆过滤器
	if ctx.Platform == "Android" && ctx.CtBF != nil {
		if ctx.CtBF.TestIndex(raw.CtBloomIndex) {
			return "android bloom filter", false
		}
	}

	return "", true
}

func (s *Service) IsDefIcon(platform, icon string) bool {
	if platform == "iOS" {
		return icon == common.AsDefIcon
	}
	if platform == "Android" {
		return icon == common.GpDefIcon
	}
	return true
}

func httpsReplaceCopy(tracks []string) []string {
	// 注意，这里不能直接在数组里面修改，因为ctx的track数组是引用的s.conf.xxxTks，要修改成https，必须复制数组
	newTracks := make([]string, 0, len(tracks))
	for _, tk := range tracks {
		if !strings.HasPrefix(tk, "https") {
			tk = strings.Replace(tk, "http", "https", 1)
		}
		newTracks = append(newTracks, tk)
	}
	return newTracks
}

func (s *Service) SetCtxTks(ctx *http_context.Context) {
	if ctx.Platform == "iOS" {
		ctx.PreImpTks = s.conf.IosPreImpTks
		ctx.PostImpTks = s.conf.IosPostImpTks
		// url for debug
		ctx.PostImpTksDebug = []string{"http://log.ambimob.com/ios/v1/impression"}
		// 3/4/9是无感，不用发点击日志
		if ctx.AdType != "3" && ctx.AdType != "4" && ctx.AdType != "9" {
			ctx.PreClkTks = s.conf.IosPreClkTks
			ctx.PostClkTks = s.conf.IosPostClkTks
			ctx.PostClkTksDebug = []string{"http://log.ambimob.com/ios/v1/click"}
		}
	} else if ctx.Platform == "Android" {
		copy(ctx.PreImpTks, s.conf.AndroidPreImpTks)
		ctx.PostImpTks = s.conf.AndroidPostImpTks
		ctx.PostImpTksDebug = []string{"http://log.ambimob.com/android/v1/impression"}
		// 3/4/9是无感，不用发点击日志
		if ctx.AdType != "3" && ctx.AdType != "4" && ctx.AdType != "9" {
			ctx.PreClkTks = s.conf.AndroidPreClkTks
			ctx.PostClkTks = s.conf.AndroidPostClkTks
			ctx.PostClkTksDebug = []string{"http://log.ambimob.com/android/v1/click"}
		}
	} else {
		// untouch code here
		panic("unexpected platform: " + ctx.Platform)
	}

	// 新版插屏需要
	if ctx.AdType == "15" {
		ctx.VastServerApi = s.conf.VastServerApi
		if ctx.Country == "CN" {
			ctx.VastJsUrl = s.conf.VastJsUrlCN
		} else {
			ctx.VastJsUrl = s.conf.VastJsUrl
		}
	}
}

func (s *Service) makeRetrievalConditions(ctx *http_context.Context) []dnf.Cond {
	conds := make([]dnf.Cond, 0, 4)

	conds = append(conds, dnf.Cond{"platform", ctx.Platform})

	if ctx.Platform == "iOS" {
		if ctx.Network == "3" { // mobile
			conds = append(conds, dnf.Cond{"network", "mobile"})
		} else if ctx.Network == "5" { // wifi
			conds = append(conds, dnf.Cond{"network", "wifi"})
		} else {
			// 其他状态暂时不处理
		}
	} else if ctx.Platform == "Android" {
		if ctx.Network == "1" {
			conds = append(conds, dnf.Cond{"network", "wifi"})
		} else if ctx.Network == "0" {
			conds = append(conds, dnf.Cond{"network", "mobile"})
		} else {
			// 其他状态暂时不处理
		}
	}

	// major version
	if len(ctx.Version) > 0 {
		major, _, _ := util.GetOsVersion(ctx.Version)
		conds = append(conds, dnf.Cond{"version", strconv.Itoa(major)})
	}
	if len(ctx.Country) > 0 {
		conds = append(conds, dnf.Cond{"country", ctx.Country})
	}
	if len(ctx.Region) > 0 {
		conds = append(conds, dnf.Cond{"region", ctx.Region})
	}
	if len(ctx.City) > 0 {
		conds = append(conds, dnf.Cond{"city", ctx.City})
	}
	if ctx.CreativeType == "video" {
		conds = append(conds, dnf.Cond{"material", "video"})
	} else { // 目前除了视频都是图片
		conds = append(conds, dnf.Cond{"material", "img"})
	}

	// device
	if len(ctx.Device) > 0 {
		conds = append(conds, dnf.Cond{"device", ctx.Device})
	} else {
		conds = append(conds, dnf.Cond{"device", "any"})
	}

	return conds
}

func (s *Service) matchKeyWords(raw *raw_ad.RawAdObj, ctx *http_context.Context) bool {
	if len(ctx.KeyWords) == 0 {
		// 搜索功能未开启
		return true
	}

	for _, kw := range ctx.KeyWords {
		if ctx.Match != "desc" {
			// 如果指定只搜索desc，则不搜索title
			if strings.Contains(raw.AppDownload.TitleLC, kw) {
				return true
			}
		}

		if ctx.Match != "title" {
			// 如果指定只搜索title，则不搜索desc
			if strings.Contains(raw.AppDownload.DescLC, kw) {
				return true
			}
		}
	}
	return false
}

func (s *Service) statistic() {
	for i := 0; ; i++ {
		time.Sleep(time.Second)
		s.l.Println(s.stat.QpsStat())
		if i%10 == 0 {
			s.l.Println("@@@ sdkStat: ", s.stat.GetSdkStat().ToString())
			s.l.Println("@@@ natStat: ", s.stat.GetNatStat().ToString())
			s.l.Println("@@@ rtStat: ", s.stat.GetRtStat().ToString())
			s.l.Println("@@@ pmtStat: ", s.stat.GetPmtStat().ToString())
			s.l.Println("@@@ wuganStat: ", s.stat.GetWuganStat().ToString())
			s.l.Println("@@@ jstagStat: ", s.stat.GetJstagStat().ToString())
			s.l.Println("@@@ realtimeStat: ", s.stat.GetRltStat().ToString())
			s.l.Println("@@@ jstagH5Stat: ", s.stat.GetJstagH5Stat().ToString())
		}
	}
}

func (s *Service) Serve() {
	http.HandleFunc(s.conf.Path, s.retrievalHandler)
	http.HandleFunc(s.conf.NativePath, s.nativeHandler)
	http.HandleFunc(s.conf.PromotionPath, s.promoteHandler)
	http.HandleFunc(s.conf.VideoPath, s.videoHandler)
	http.HandleFunc(s.conf.WuganPath, s.wuganHandler)
	http.HandleFunc("/get_sub_ad", s.subHandler)
	http.HandleFunc(s.conf.FunnyPath, s.funnyHandler)
	http.HandleFunc(s.conf.JstagPath, s.jstagHandler)
	http.HandleFunc(s.conf.JstagMediaPath, s.jstagMediaHandler)
	http.HandleFunc(s.conf.RealtimePath, s.realtimeHandler)
	http.HandleFunc(s.conf.JstagH5Path, s.jstagH5Handler)
	http.HandleFunc(s.conf.AppWallPath, s.appWallHandler)
	http.HandleFunc(s.conf.InterstitialPath, s.interstitialHandler)
	// pagead
	http.HandleFunc(s.conf.PageadPath, s.pageadHandler)
	http.HandleFunc(s.conf.LifePath, s.lifeHandler)

	http.HandleFunc("/video/v4/creative/get", s.videoCreativeHandler)
	http.HandleFunc("/video/v4/ad/get", s.videoAdHandler)
	http.HandleFunc("/video/v4/native/get", s.videov4NativeHandler)

	go s.statistic()

	panic(http.ListenAndServe(fmt.Sprintf("%s:%d", s.conf.ServerName, s.conf.ListenPort), nil))
}
