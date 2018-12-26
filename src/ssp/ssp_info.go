package ssp

import (
	"bytes"
	"crypto/md5"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"store"
	"strconv"
	"strings"
	"sync"

	"aes"
	"ios_pmt"
	"util"
)

const (
	BANNER_320X50          = "banner_320x50"
	BANNER_320X100         = "banner_320x100"
	BANNER_300X250         = "banner_300x250"
	INTERSTITIAL_LANDSCAPE = "interstitial_landscape"
	INTERSTITIAL_PORTRAIT  = "interstitial_portrait"
)

var tplAdapters = map[string][]string{
	BANNER_320X50: []string{
		"i_320x50",
		"i_0x0",
	},
	BANNER_320X100: []string{
		"i_320x100",
		"i_190x100",
	},
	BANNER_300X250: []string{
		"i_300x250",
		"i_190x100",
	},
	INTERSTITIAL_PORTRAIT: []string{
		"i_190x100+v_190x100",
		"i_100x190",
		"i_190x100",
	},
	INTERSTITIAL_LANDSCAPE: []string{
		"i_190x100+v_190x100",
		"i_190x100",
	},
}

var err error
var imgReg *regexp.Regexp
var offlineSlots map[string]bool

var largeTpl, middleTpl, smallTpl string
var tpl9, tpl10, tpl11, tpl12 string

var MraidTpl, HtmlTpl *template.Template

func init() {
	imgReg = regexp.MustCompile("\\{\\$img_(\\d+)x(\\d+)\\}")
	offlineSlots = map[string]bool{
		"633": true,
		"753": true,
		"884": true,
		"897": true,
		"941": true,
	}

	largeTpl = readTxtFile("tpl_files/TemplateL001.txt")
	middleTpl = readTxtFile("tpl_files/TemplateM002.txt")
	smallTpl = readTxtFile("tpl_files/TemplateS003.txt")

	// 新插屏广告标准模板
	// 横屏大图
	tpl9 = readTxtFile("tpl_files/TemplateLHFJ009.txt")
	// 竖屏大图
	tpl10 = readTxtFile("tpl_files/TemplateLVF010.txt")
	// 视频横图
	tpl11 = readTxtFile("tpl_files/TemplateLHV011.txt")
	// 视频竖图
	tpl12 = readTxtFile("tpl_files/TemplateLVV012.txt")

	// pagead 模板
	MraidTpl, err = template.New("mraid").Parse(readTxtFile("tpl_files/MraidTpl.html"))
	if err != nil {
		panic(err)
	}
	HtmlTpl, err = template.New("app").Parse(readTxtFile("tpl_files/HtmlTpl.html"))
	if err != nil {
		panic(err)
	}
}

func readTxtFile(name string) string {
	f, err := os.Open(name)
	if err != nil {
		return ""
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return ""
	}
	return string(b)
}

func matchTplSize(b []byte) (w, h int) {
	sub := imgReg.FindSubmatch(b)
	if len(sub) != 3 {
		return
	}
	w, _ = strconv.Atoi(string(sub[1]))
	h, _ = strconv.Atoi(string(sub[2]))
	return
}

type RWLockerUpdater interface {
	Lock()
	Unlock()
	Update(r io.Reader) (cont bool)
}

func UpdateViaS3(u RWLockerUpdater, region, bucket, prefix, path string) error {
	u.Lock()
	defer u.Unlock()
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	keys, err := store.ListFolder(region, path, bucket, prefix)
	if err != nil {
		return fmt.Errorf("s3 list %s/%s/%s file error %v", region, bucket, prefix, err)
	}

	for _, k := range keys {
		data, err := store.DownloadFile(region, path, bucket, k)
		if err != nil {
			fmt.Println("update app or slot error ", k, ", err: ", err)
			continue
		}
		if !u.Update(bytes.NewReader(data)) {
			break
		}
	}
	return nil
}

func UpdateViaUrl(u RWLockerUpdater, urlFormat string) error {
	u.Lock()
	defer u.Unlock()
	size := 20
	stop := false
	for page := 1; !stop; page++ {
		func() {
			url := fmt.Sprintf(urlFormat, page, size)
			resp, err := http.Get(url)
			if resp != nil && resp.Body != nil {
				defer resp.Body.Close()
			}
			if err != nil {
				fmt.Println("update app or slot error, url:", url, ", err:", err)
				stop = true
				return
			}
			if !u.Update(resp.Body) {
				stop = true
				return
			}
		}()
	}
	fmt.Println("UpdateViaUrl: ", urlFormat, " finish")
	return nil
}

type AppInfo struct {
	Id          int           `json:"id"`
	AppName     string        `json:"app_name"`
	SlotsInter  []interface{} `json:"slots,omitempty"`
	Slots       []string      `json:"-"`
	MainTagId   int           `json:"main_tag_id"`
	SecondTagId int           `json:"second_tag_id"`
	Platform    int           `json:"platform"` // 1: Android, 2: iOS
	Keywords    []string      `json:"keywords,omitempty"`
	Version     string        `json:"version"`
	Url         string        `json:"url"` // 应用市场url
	PkgName     string        `json:"package_name"`
	Dau         int           `json:"dau"`                  // dau level
	BlackList   []int         `json:"black_list,omitempty"` // 广告主分类黑名单
	Gps         int           `json:"gps"`                  // 1: on, 2: off
	Charge      int           `json:"charge"`               // 1: on, 2: off

	Male   int `json:"male"` // 应用男性和女性倾向，male + female === 100
	Female int `json:"female"`

	AgeId     []int `json:"age_id,omitempty"` // age level
	AppSwitch int   `json:"app_switch"`       // 应用状态，1: on, 2: off
	PreNum    int   `json:"preclick_num"`     // wugan ads number per webview

	AppVersionInfos []AppVersionInfo `json:"app_relation_version"`

	// eg: "conf":{"fb":0,"ad_c":1,"ct":2,"ad_d":3}
	// XXX: v3 deprecated
	Conf map[string]int `json:"conf"` // 广告配置优先级

	CreateTime int `json:"create_time"` // time.Time.UnixNano() / (1000 * 1000)
	UpdateTime int `json:"update_time"` // time.Time.UnixNano() / (1000 * 1000)

	UpdateUtcTime string `json:"update_utc_time"`

	Manager *AppSlotManager `json:"-"`
}

type AppVersionInfo struct {
	SdkVersion string `json:"sdk_version"`
	AppVersion string `json:"app_version"` // 有些媒体是用三位appVersion
	Status     int    `json:"status"`      // 1:开启后截，2:关闭后截
}

func (app *AppInfo) ToJson() (b []byte) {
	b, _ = json.Marshal(app)
	return
}

func (app *AppInfo) GetAllSlots() (slots []*SlotInfo, err error) {

	slots = make([]*SlotInfo, 0, len(app.Slots))

	allSlots := GetGlobalSlotStore()
	if allSlots == nil {
		return slots, fmt.Errorf("no data")
	}

	for _, slotId := range app.Slots {
		if slot := allSlots.Get(slotId); slot != nil {
			slots = append(slots, slot)
		}
	}
	return slots, nil
}

type AppStore struct {
	sync.RWMutex
	m map[int]*AppInfo
}

func NewAppStore() *AppStore {
	return &AppStore{
		m: make(map[int]*AppInfo),
	}
}

func (this *AppStore) SaveToFile(file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	this.Lock()
	defer this.Unlock()

	w := gob.NewEncoder(f)
	return w.Encode(this.m)
}

func (this *AppStore) LoadFromFile(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	this.Lock()
	defer this.Unlock()

	r := gob.NewDecoder(f)
	return r.Decode(&this.m)
}

func (app *AppStore) DumpAllApps(w io.Writer) {
	slice := make([]*AppInfo, 0, len(app.m))
	for _, v := range app.m {
		slice = append(slice, v)
	}
	enc := json.NewEncoder(w)
	enc.Encode(slice)
}

func (app *AppStore) Update(r io.Reader) (cont bool) {
	ch, decErr := util.DecodeJsonArrayStream(r,
		func(dec *json.Decoder, ch chan<- interface{}) error {
			var info AppInfo
			if err := dec.Decode(&info); err != nil {
				fmt.Println("[app.update] json decode err: ", err)
				return err
			}
			for _, slotIdInter := range info.SlotsInter {
				if slotIdFloat, ok := slotIdInter.(float64); ok {
					info.Slots = append(info.Slots, strconv.Itoa(int(slotIdFloat)))
				} else if slotIdStr, ok := slotIdInter.(string); ok {
					info.Slots = append(info.Slots, slotIdStr)
				}
			}
			ch <- &info
			return nil
		})
	if decErr != nil {
		fmt.Println("[app.update] decode json array stream error: ", decErr)
		return false
	}

	for appInter := range ch {
		cont = true
		appInfo := appInter.(*AppInfo)
		app.setWithoutLock(appInfo)

		if len(appInfo.AppVersionInfos) > 0 {
			for _, slot := range appInfo.Slots {
				for _, version := range appInfo.AppVersionInfos {
					var slotAppVerInfo string
					if version.SdkVersion == "*" {
						slotAppVerInfo = util.StrJoinHyphen(slot, "*") // 对于线下的应用，做通配处理
					} else {
						slotAppVerInfo = util.StrJoinHyphen(slot, version.SdkVersion, version.AppVersion)
					}
					if appInfo.AppSwitch == 2 {
						ios_pmt.StoreVerMap(slotAppVerInfo, 2) // 如果这个app关闭，那么对应后截也关闭
					} else {
						ios_pmt.StoreVerMap(slotAppVerInfo, version.Status)
					}
				}
			}
		}
	}
	return
}

func (app *AppStore) Get(id int) *AppInfo {
	app.RLock()
	defer app.RUnlock()
	return app.m[id]
}

func (app *AppStore) Set(info *AppInfo) {
	app.Lock()
	defer app.Unlock()
	info.UpdateUtcTime = util.Ts2String(int64(info.UpdateTime))
	app.m[info.Id] = info
}

func (app *AppStore) setWithoutLock(info *AppInfo) {
	info.UpdateUtcTime = util.Ts2String(int64(info.UpdateTime))
	app.m[info.Id] = info
}

var gAppStore *AppStore
var gAppStoreRWMutex sync.RWMutex

func GetGlobalAppStore() *AppStore {
	gAppStoreRWMutex.RLock()
	defer gAppStoreRWMutex.RUnlock()
	return gAppStore
}

func SetGlobalAppStore(app *AppStore) {
	gAppStoreRWMutex.Lock()
	defer gAppStoreRWMutex.Unlock()
	gAppStore = app
}

type PageadTpl struct {
	// 模板适配器
	Adapter map[string][]string `json:"adapters"`
	Control interface{}         `json:"control"`
}

type TemplateObj struct {
	Id   int    `json:"id"` // 模板id
	H5   string `json:"h5"` // h5代码
	Sign string `json:"-"`
	Type int    `json:"type"` // 1: 标准模板; 2: 自定义模板

	// 如果Type是1，style_type 1,2,3表示大、中、小模板(从文件读取到H5)
	// 如果Type是1，style_type 不为1、2、3，则填充任意base64编码字符串到H5即可
	// 如果Type是2，style_type为0，使用ssp提供的h5
	// NOTE: style_type字段 4,5,6,7,8 在ssp里和广告类型一致
	StyleType int `json:"style_type"` // 9 横屏大图, 10 竖屏大图, 11 视频横图, 12 视频竖图

	width   int `json:"-"`
	height  int `json:"-"`
	EncFlag int `json:"-"` // 0: un-init，1: encoded; 2: raw-h5

	InitFlag bool     `json:"-"`
	PosSlice PosSlice `json:"-"`
}

var emptyBase64Html string = "PGh0bWw+PC9odG1sPg==" // base64 val of '<html></html>'

func (t *TemplateObj) Init() {
	if t.Type == 1 {
		switch t.StyleType {
		case 1: // Large Tpl
			t.H5 = largeTpl
		case 2: // Middle Tpl
			t.H5 = middleTpl
		case 3: // Small Tpl
			t.H5 = smallTpl
		case 9:
			t.H5 = tpl9
		case 10:
			t.H5 = tpl10
		case 11:
			t.H5 = tpl11
		case 12:
			t.H5 = tpl12
		default:
			t.H5 = emptyBase64Html
		}
	} else if t.Type == 2 {
		if len(t.H5) == 0 {
			t.H5 = emptyBase64Html
		}
	} else {
		// unexpected type
	}

	md5Sum := md5.Sum([]byte(t.H5))
	t.Sign = string(md5Sum[:])
	if ps, ok := scanTpl(t.H5); ok {
		t.PosSlice = ps
		t.InitFlag = true
		return
	}
	t.InitFlag = false
}

func (t *TemplateObj) Size() (w, h int) { return t.width, t.height }

func (t *TemplateObj) base64Encode() {
	if t.EncFlag < 0 || t.EncFlag > 2 {
		panic("unexpected encode flag of " + strconv.Itoa(t.EncFlag))
	}
	if t.EncFlag == 2 {
		b, _ := util.Base64Encode([]byte(t.H5))
		t.H5 = string(b)
	}
	t.EncFlag = 1
}

func (t *TemplateObj) base64Decode() {
	if t.EncFlag < 0 || t.EncFlag > 2 {
		panic("unexpected encode flag of " + strconv.Itoa(t.EncFlag))
	}
	if t.EncFlag != 2 {
		b, _ := util.Base64Decode([]byte(t.H5))
		if t.EncFlag == 0 {
			// uninitialized
			t.width, t.height = matchTplSize(b)
		}
		t.H5 = string(b)
	}
	t.EncFlag = 2
}

type ThirdSlotId struct {
	FbId    string `json:"facebook"` // facebook id
	AdmobId string `json:"admob"`    // admob id
}

type PreClickController struct {
	Serf int `json:"serf"` // 串行次数
	Palf int `json:"palf"` // 每次并行发送的条数
	Intv int `json:"intv"` // 每次间隔时间
}

type ActiveGuard struct {
	StaticSwitch  int `json:"stc_tr"` // 静态注册保活线程开关, 1: on, 2: off
	DynamicSwitch int `json:"dyn_tr"` // 进程拉起保活开关, 1: on, 2: off
}

type VideoControl struct {
	VOrient     int    `json:"v_orient"`     // video播放朝向 1:横版，2:竖版，（缺省自动适配当前手机方向）
	VNum        int    `json:"v_num"`        // 视频广告缓存个数（默认3）
	MaxPlay     int    `json:"max_play"`     // 每个视频广告最大可播放次数
	VCap        int    `json:"v_cap"`        // 视频广告缓存容量（单位M，默认-1表无限制）
	IsPreload   int    `json:"is_preload"`   // 0：不缓存， 1：缓存（默认1）
	NoWifiLoad  int    `json:"no_wifi_load"` // 0：不继续加载， 1：继续加载（默认0）
	ClickTime   int    `json:"click_time"`   // 发送click url的事件节点
	IsInner     int    `json:"is_inner"`     // 针对iOS应用内开启appstore组件
	LoadTime    int    `json:"load_time"`    // 针对iOS应用内开启时提前加载final url事件节点
	ButtonColor string `json:"button_color"` // 插屏下载按钮配色
	H5Opt       string `json:"h5_opt"`       // 是否用webview去加载h5页面url
	PpUrl       string `json:"pp_url"`       // Video Privacy Policy
}

type VideoIntegrationObj struct {
	Type           string `json:"type"`
	Status         int    `json:"status"`
	ApiKey         string `json:"api_key"`
	ApiAppId       string `json:"api_app_id"`
	ApiPlacementId string `json:"api_placement_id"`
}

type SlotInfo struct {
	Id       int    `json:"id"`
	IdStr    string `json:"slot_str"` // 新版协议，SlotId为字符串类型
	AppId    int    `json:"app_id"`
	SlotName string `json:"slot_name"`

	// 1: 插屏, 2: 原生, 3: 条幅, 4：纯无感，5：应用墙，6：视频，7：激励视频, 8：原生视频,
	// 9：新插屏(含视频)
	Format int `json:"format"`

	Third ThirdSlotId `json:"third_slot_id"`

	Templates []TemplateObj      `json:"templates"`
	Xd        PreClickController `json:"xd"`
	Ag        ActiveGuard        `json:"ag"`
	Vc        VideoControl       `json:"video"`

	FrequencySwitch int     `json:"frequency_switch"` // 1: on, 2: off
	SlotSwitch      int     `json:"slot_switch"`      // 1: on, 2: off
	PreClick        int     `json:"pre_click"`        // 1: on, 2: off`
	ImpressionRate  float64 `json:"impression_rate"`  // [0,1]

	// 流量白名单{googleplaydownload, ddl, subscription}
	WhiteListFilter []string `json:"whitelist_filter"`

	// 应用类型黑名单{adult, games, ...}
	BlackListFilter []string `json:"blacklist_filter"`

	PkgBlackList []string        `json:"package_blacklist"`
	PkgBlackMap  map[string]bool `json:"-"`

	AdNetworkBlackList []string        `json:"ad_network_black_list"`
	AdNetworkBlackMap  map[string]bool `json:"-"`

	AdNetworkWhiteList []string        `json:"ad_network_white_list"`
	AdNetworkWhiteMap  map[string]bool `json:"-"`

	OfferBlackList []string        `json:"offer_black_list"` // fmt: "irs_2359781"
	OfferBlackMap  map[string]bool `json:"-"`

	CreateTime int `json:"create_time"` // time.Time.UnixNano() / (1000 * 1000)
	UpdateTime int `json:"update_time"` // time.Time.UnixNano() / (1000 * 1000)

	UpdateUtcTime string `json:"update_utc_time"`

	// eg: "conf":{"fb":0,"ad_c":1,"ct":2,"ad_d":3}
	Priority map[string]int `json:"priority_list"` // for non-video ad

	// for video integration
	VideoIntegration []VideoIntegrationObj `json:"video_integration"`
	VideoSort        map[string]int        `json:"video_sort"`

	// for rewarded video
	RewardedCurrency string `json:"virtual_currency_name"`
	RewardedAmount   string `json:"rewarded_amount"`
	RewardedCallback string `json:"server_callback_url"` // 服务端回调
	RewardedCbKey    string `json:"server_callback_key"` // 服务端回调签名key

	VideoScreenType int    `json:"video_screen_type"` // video screen type 1: 横屏 2: 竖屏
	AdCacheNum      int    `json:"ad_cache_num"`      // 广告缓存次数
	AdPlayNum       int    `json:"ad_play_num"`       // 广告最大播放次数
	VCap            int    `json:"v_cap"`             //
	IsPreload       int    `json:"is_preload"`        // 是否缓存
	NoWifiLoad      int    `json:"no_wifi_load"`      // 当本地缓存广告数不为0时，无wifi的环境下是否持续加载广告
	ClickTime       int    `json:"click_time"`        // 发送click url的事件节点
	IsInner         int    `json:"is_inner"`          // 针对iOS应用内开启appstore组件	1：应用内开（缺省默认） 2：外开appstore跳转
	LoadTime        int    `json:"load_time"`         // 针对iOS应用内开启时提前加载final url事件节点	整型（0-18）缺省默认为1
	ButtonColor     string `json:"button_color"`      // 插屏下载按钮配色	字符串 (缺省为草绿色 #1adfa3)
	H5Opt           string `json:"h5_opt"`            // 字符串，即h5 url地址 （缺省为常规容器加载
	VideoFreqCap    int    `json:"cap"`               // 控制每个用户在特定slot每天最多观看完成的视频数量（0-100），默认为-1:不做限制
	MaxImpression   int    `json:"max_impression"`    // slot下最大曝光数，默认值(-1:不做限制)。
	MaxComplete     int    `json:"max_complete"`      // slot下激励视频最大观看完成数曝光数(不分用户)，默认值(-1:不做限制)。

	SlotImpNum int `json:"slot_imp_num"` // 通过ssp和redash计算出的曝光数, 由ssp_update更新

	SubscriptionSwitch int      `json:"subscription_switch"` // 订阅开关：1代表请求订阅广告，2代表不请求订阅广告
	SubscriptionSdks   []string `json:"subscription_sdks"`   // 订阅sdk开关

	// 淘口令
	TbStatus int `json:"tb_status"` // 1:开启, 2:关闭

	// pagead tpl
	PageadTpl *PageadTpl `json:"pagead_tpl"`

	PmtSwitch     int `json:"pmt_switch"`      // 1:开启 2:关闭
	SemiPmtSwitch int `json:"semi_pmt_switch"` // 1:开启 2:关闭

	brothers []*SlotInfo

	preNum       int // wugan ads number per webview
	preNumInited bool
}

func NewVideoControl(slot *SlotInfo, isPreLoad int) *VideoControl {
	if slot == nil {
		return &VideoControl{
			VOrient:     1,
			VNum:        3,
			MaxPlay:     3,
			VCap:        -1,
			IsPreload:   isPreLoad,
			NoWifiLoad:  1,
			ClickTime:   0,
			IsInner:     1,
			LoadTime:    1,
			ButtonColor: "#1adfa3",
			H5Opt:       "",
			PpUrl:       "http://en.yeahmobi.com/privacy-policy",
		}
	}
	return &VideoControl{
		VOrient:     slot.VideoScreenType,
		VNum:        slot.AdCacheNum,
		MaxPlay:     slot.AdPlayNum,
		VCap:        slot.VCap,
		IsPreload:   isPreLoad,
		NoWifiLoad:  slot.NoWifiLoad,
		ClickTime:   slot.ClickTime,
		IsInner:     slot.IsInner,
		LoadTime:    slot.LoadTime,
		ButtonColor: slot.ButtonColor,
		H5Opt:       slot.H5Opt,
		PpUrl:       "http://en.yeahmobi.com/privacy-policy",
	}
}

func (slot *SlotInfo) GetPreNum() int {
	if gAppStore == nil {
		return 0
	}

	if slot.preNumInited {
		return slot.preNum
	}

	appInfo := gAppStore.Get(slot.AppId)
	slot.preNum = appInfo.PreNum
	slot.preNumInited = true

	return slot.preNum
}

func (slot *SlotInfo) GetRandomBrother(retry int) *SlotInfo {
	l := len(slot.brothers)
	if l <= 1 || retry <= 0 {
		return slot
	}
	for i := 0; i < retry; i++ {
		var sel *SlotInfo
		n := rand.Intn(l + 1)
		if n == l {
			sel = slot
		} else {
			sel = slot.brothers[n] // n: [0, l-1]
		}
		if sel.SlotSwitch != 2 {
			return sel
		}
	}
	return slot
}

func (slot *SlotInfo) MatchSspFormat(format int) bool {
	return slot.Format == format
}

func (this *SlotStore) GetVideoSlot(someSlotId string) *SlotInfo {
	isActiveVideoSlot := func(info *SlotInfo) bool {
		return info.SlotSwitch != 2 && (info.Format == 6 || info.Format == 7 || info.Format == 8)
	}

	someInfo := this.Get(someSlotId)
	if someInfo == nil {
		return nil
	}

	if isActiveVideoSlot(someInfo) {
		return someInfo
	}

	for _, info := range someInfo.brothers {
		if isActiveVideoSlot(info) {
			return info
		}
	}
	return nil
}

// product category
func (slot *SlotInfo) InWhiteList(cat string) bool {
	for i := 0; i != len(slot.WhiteListFilter); i++ {
		if slot.WhiteListFilter[i] == cat {
			return true
		}
	}
	return false
}

// app category: casino, strategy, ...
func (slot *SlotInfo) AppCategoryInBlackList(cats []string) bool {
	for i := 0; i != len(slot.BlackListFilter); i++ {
		for _, cat := range cats {
			if strings.Contains(slot.BlackListFilter[i], cat) {
				return true
			}
		}
	}
	return false
}

func (slot *SlotInfo) TlBase64Encode() {
	for i := 0; i != len(slot.Templates); i++ {
		slot.Templates[i].base64Encode()
	}
}

func (slot *SlotInfo) TlBase64Decode() {
	for i := 0; i != len(slot.Templates); i++ {
		slot.Templates[i].base64Decode()
	}
}

func (slot *SlotInfo) ToJson() (b []byte) {
	b, _ = json.Marshal(slot)
	return
}

// slot级别的曝光控制
func (slot *SlotInfo) SlotImpInCap(limit int) bool {
	if limit == -1 {
		return true
	}
	if limit == 0 {
		return false
	}

	if slot.SlotImpNum == 0 { // 表示该slot未设置曝光上限
		return true
	}

	ratio := float64(slot.SlotImpNum) / float64(limit)
	if ratio > 1 {
		return false
	} else if ratio > 0.8 { // 超过80%上限后，只有0.2概率允许pass
		return rand.Float64() < 0.2
	} else {
		return true
	}
}

// 更新模板信息
func (slot *SlotInfo) UpdateTpl() {
	// XXX hot fix: 兼容老版插屏和新版插屏
	if slot.Format == 1 && len(slot.Templates) >= 1 {
		slot.Templates = append(slot.Templates, TemplateObj{
			Type:      1, // 标准模板
			StyleType: 9,
		})
		slot.Templates = append(slot.Templates, TemplateObj{
			Type:      1, // 标准模板
			StyleType: 10,
		})
		slot.Templates = append(slot.Templates, TemplateObj{
			Type:      1, // 标准模板
			StyleType: 11,
		})
		slot.Templates = append(slot.Templates, TemplateObj{
			Type:      1, // 标准模板
			StyleType: 12,
		})
	}

	for i := 0; i < len(slot.Templates); i++ {
		slot.Templates[i].Init()
	}
}

// 获取pagead适配器信息
func (slot *SlotInfo) GetPageadTpl(key string) ([]string, interface{}) {
	if slot.PageadTpl != nil {
		return slot.PageadTpl.Adapter[key], slot.PageadTpl.Control
	}

	return tplAdapters[key], nil
}

func (slot *SlotInfo) EnablePmt() bool {
	return slot.PmtSwitch == 1
}

func (slot *SlotInfo) EnableSemiPmt() bool {
	return slot.SemiPmtSwitch == 1
}

func IsVideoCache(slots []*SlotInfo) bool {
	for i := 0; i < len(slots); i++ {
		// 只有激励视频做缓存
		if slots[i].SlotSwitch != 2 && slots[i].Format == 7 {
			return true
		}
	}
	return false
}

func IsVideoSlot(slotId string) bool {
	if slots := GetGlobalSlotStore(); slots != nil {

		slot := slots.Get(slotId)
		if slot != nil && (slot.Format == 6 || slot.Format == 7 || slot.Format == 8) {
			return true
		}
	}
	return false
}

func GetSubscriptionInfo(slotId string) (bool, []string) {
	if slots := GetGlobalSlotStore(); slots != nil {
		slot := slots.Get(slotId)
		if slot == nil {
			return false, nil
		}
		return slot.SubscriptionSwitch == 1, slot.SubscriptionSdks
	}
	return false, nil
}

type SlotStore struct {
	sync.RWMutex
	m map[string]*SlotInfo
}

func NewSlotStore() *SlotStore {
	return &SlotStore{
		m: make(map[string]*SlotInfo),
	}
}

func (this *SlotStore) SaveToFile(file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	this.Lock()
	defer this.Unlock()

	w := gob.NewEncoder(f)
	return w.Encode(this.m)
}

func (this *SlotStore) LoadFromFile(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	this.Lock()
	defer this.Unlock()

	r := gob.NewDecoder(f)
	return r.Decode(&this.m)
}

func (this *SlotStore) GenBrothers(appStore *AppStore) {
	for _, appInfo := range appStore.m {
		if len(appInfo.Slots) > 0 {
			start := appInfo.Slots[0]
			slots, _ := this.GetAllSlotInApp(start)
			for _, slot := range slots {
				if offlineSlots[slot.IdStr] {
					// 离线API里面的slot，不做收益平摊
					continue
				}
				for i := 0; i != len(slots); i++ {
					if slots[i].IdStr != slot.IdStr && !offlineSlots[slots[i].IdStr] {
						slot.brothers = append(slot.brothers, slots[i])
					}
				}
			}
		}
	}
}

func (this *SlotStore) GetAllSlotInApp(id string) (slots []*SlotInfo, err error) {
	appStore := GetGlobalAppStore()
	if appStore == nil {
		err = errors.New("system initializing...")
		return
	}
	slot := this.Get(id)
	if slot == nil {
		err = errors.New(fmt.Sprintf("slot_id %s un-exist", id))
		return
	}

	app := appStore.Get(slot.AppId)
	if app == nil {
		err = errors.New(
			fmt.Sprintf("app_id %d un-exist, slot_id: %s", slot.AppId, id))
		return
	}

	if app.AppSwitch != 1 {
		slot.SlotSwitch = 2
	}
	slots = append(slots, slot)

	for _, s := range app.Slots {
		if s == slot.IdStr {
			continue
		} else if newSlot := this.Get(s); newSlot != nil {
			if app.AppSwitch != 1 {
				slot.SlotSwitch = 2
			}
			slots = append(slots, newSlot)
		}
	}
	return
}

func (slot *SlotStore) DumpAllSlots(w io.Writer) {
	slice := make([]SlotInfo, 0, len(slot.m))
	for _, v := range slot.m {
		s := *v
		for i := 0; i != len(s.Templates); i++ {
			s.Templates[i].H5 = ""
		}
		slice = append(slice, s)
	}
	enc := json.NewEncoder(w)
	enc.Encode(slice)
}

func (slot *SlotStore) Update(r io.Reader) (cont bool) {
	ch, decErr := util.DecodeJsonArrayStream(r,
		func(dec *json.Decoder, ch chan<- interface{}) error {
			var info SlotInfo
			if err := dec.Decode(&info); err != nil {
				return err
			}
			if info.UpdateTime == 0 {
				// to protect bugs from frontend
				info.UpdateTime = util.NowMillisecondTs()
				info.UpdateUtcTime = util.NowString()
			}
			if len(info.AdNetworkBlackList) > 0 {
				info.AdNetworkBlackMap = make(map[string]bool)
				for _, ch := range info.AdNetworkBlackList {
					info.AdNetworkBlackMap[ch] = true
				}
			}
			if len(info.AdNetworkWhiteList) > 0 {
				info.AdNetworkWhiteMap = make(map[string]bool)
				for _, ch := range info.AdNetworkWhiteList {
					info.AdNetworkWhiteMap[ch] = true
				}
			}
			if len(info.OfferBlackList) > 0 {
				info.OfferBlackMap = make(map[string]bool)
				for _, offer := range info.OfferBlackList {
					info.OfferBlackMap[offer] = true
				}
			}
			if len(info.PkgBlackList) > 0 {
				info.PkgBlackMap = make(map[string]bool)
				for _, pkg := range info.PkgBlackList {
					info.PkgBlackMap[pkg] = true
				}
			}

			if len(info.IdStr) == 0 {
				info.IdStr = strconv.Itoa(info.Id) // IdStr，兼容旧版协议
			}

			// 更新info的模板信息，初始化正则
			info.UpdateTpl()
			ch <- &info
			return nil
		})
	if decErr != nil {
		fmt.Println("[slot.update] json decode err: ", decErr)
		return false
	}
	for slotInter := range ch {
		cont = true
		slot.setWithoutLock(slotInter.(*SlotInfo))
	}
	return
}

func (slot *SlotStore) Get(id string) *SlotInfo {
	slot.RLock()
	defer slot.RUnlock()
	return slot.m[id]
}

func (slot *SlotStore) Set(info *SlotInfo) {
	slot.Lock()
	defer slot.Unlock()
	slot.setWithoutLock(info)
}

func (slot *SlotStore) setWithoutLock(info *SlotInfo) {
	info.UpdateUtcTime = util.Ts2String(int64(info.UpdateTime))
	for i := 0; i != len(info.BlackListFilter); i++ {

		// to deal ssp category such as `Game-Casino*`
		elem := info.BlackListFilter[i]
		if arr := strings.Split(elem, "-"); len(arr) > 1 {
			elem = arr[len(arr)-1]
		}
		if arr := strings.Split(elem, "*"); len(arr) > 1 {
			elem = arr[0]
		}

		info.BlackListFilter[i] = strings.ToLower(elem)
	}

	for i := 0; i != len(info.WhiteListFilter); i++ {
		info.WhiteListFilter[i] = strings.ToLower(info.WhiteListFilter[i])
	}

	// store raw h5
	info.TlBase64Decode()

	// xd default value
	if info.Xd.Serf == 0 {
		info.Xd.Serf = 3
	}
	if info.Xd.Palf == 0 {
		info.Xd.Palf = 3
	}
	if info.Xd.Intv == 0 {
		info.Xd.Intv = 20
	}

	// ag default value
	if info.Ag.StaticSwitch == 0 {
		info.Ag.StaticSwitch = 1
	}
	if info.Ag.DynamicSwitch == 0 {
		info.Ag.DynamicSwitch = 1
	}

	slot.m[info.IdStr] = info
}

var gCurTaobaoToken string

func SetTaobaoToken(t string) {
	if len(t) > 0 {
		gCurTaobaoToken = aes.Encrypt(t)
	} else {
		gCurTaobaoToken = ""
	}
}

func GetTaobaoToken() string {
	return gCurTaobaoToken
}

func (this *SlotStore) GetTbkInfo(slotId string) string {
	slot := this.Get(slotId)
	if slot == nil {
		return ""
	}

	if slotId == "1361" || slotId == "1794" || slotId == "23696844" || slotId == "2507" || slotId == "25261535" || slotId == "30133956" || slotId == "3447" || slotId == "52310240" || slotId == "62724276" || slotId == "70555291" || slotId == "75166069" || slotId == "1525" {
		return gCurTaobaoToken
	}

	if slot.TbStatus != 1 { // 1:开启
		return ""
	}

	return gCurTaobaoToken
}

var gSlotStore *SlotStore
var gSlotStoreRWMutex sync.RWMutex

func GetGlobalSlotStore() *SlotStore {
	gSlotStoreRWMutex.RLock()
	defer gSlotStoreRWMutex.RUnlock()
	return gSlotStore
}

func SetGlobalSlotStore(slot *SlotStore) {
	gSlotStoreRWMutex.Lock()
	defer gSlotStoreRWMutex.Unlock()
	gSlotStore = slot
}
