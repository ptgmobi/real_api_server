package tpl

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/brg-liuwei/gotools"

	"ssp"
	"util"
)

type Conf struct {
	AppInfoPullFmt  string `json:"appinfo_pull_fmt"`
	SlotInfoPullFmt string `json:"slotinfo_pull_fmt"`

	VideoSortUrl string `json:"video_sort_url"`

	LogPath        string `json:"log_path"`
	LogRotateNum   int    `json:"log_rotate_backup"`
	LogRotateLines int    `json:"log_rotate_lines"`

	S3Region  string `json:"s3_region"`
	S3Bucket  string `json:"s3_bucket"`
	SspPrefix string `json:"ssp_prefix"`
	DownPath  string `json:"down_path"`
}

type Service struct {
	conf *Conf
	l    *gotools.RotateLogger
	stat Statistic
}

func NewService(conf *Conf) (*Service, error) {
	if conf == nil {
		return nil, errors.New("[TPL] NewService conf is nil")
	}
	l, err := gotools.NewRotateLogger(conf.LogPath, "[TPL] ",
		log.LstdFlags|log.LUTC, conf.LogRotateNum)
	if err != nil {
		return nil, errors.New("[TPL] NewRotateLogger failed: " + err.Error())
	}
	l.SetLineRotate(conf.LogRotateLines)

	if !strings.HasSuffix(conf.SspPrefix, "/") {
		conf.SspPrefix += "/"
	}

	svc := &Service{
		conf: conf,
		l:    l,
	}

	videoSortUrl = conf.VideoSortUrl

	return svc, nil
}

const (
	slotGobFile = "/tmp/offer_server_slot.gob"
	appGobFile  = "/tmp/offer_server_app.gob"
)

func (s *Service) updateApps() {
	availiable := false

	go func() {
		for {
			setThirdPartTaobaoToken()
			time.Sleep(10 * time.Second)
		}
	}()

	// firstly, load from local file
	slotStore := ssp.NewSlotStore()
	appStore := ssp.NewAppStore()

	if slotStore.LoadFromFile(slotGobFile) == nil &&
		appStore.LoadFromFile(appGobFile) == nil {

		slotStore.GenBrothers(appStore)

		ssp.SetGlobalSlotStore(slotStore)
		ssp.SetGlobalAppStore(appStore)

		appStore.GenAuxiliaryGlobalConf()
	}

	for {

		if availiable {
			time.Sleep(2*time.Minute + time.Duration(rand.Int63n(60))*time.Second)
		}

		appStore := ssp.NewAppStore()
		if err := ssp.UpdateViaS3(appStore, s.conf.S3Region, s.conf.S3Bucket,
			s.conf.SspPrefix+"app", s.conf.DownPath); err != nil {
			s.l.Println("update app info error: ", err)
			continue
		}

		if err := appStore.SaveToFile(appGobFile); err != nil {
			s.l.Println("save app gob file error: ", err)
		}

		slotStore := ssp.NewSlotStore()
		if err := ssp.UpdateViaS3(slotStore, s.conf.S3Region, s.conf.S3Bucket,
			s.conf.SspPrefix+"slot", s.conf.DownPath); err != nil {
			s.l.Println("update slot info error: ", err)
			continue
		}
		if err := slotStore.SaveToFile(slotGobFile); err != nil {
			s.l.Println("save slot gob file error: ", err)
		}

		slotStore.GenBrothers(appStore)

		ssp.SetGlobalSlotStore(slotStore)
		ssp.SetGlobalAppStore(appStore)

		appStore.GenAuxiliaryGlobalConf()

		availiable = true
	}
}

func setThirdPartTaobaoToken() {
	resp, err := http.Get("http://116.31.99.158:8303/api/k")
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		ssp.SetTaobaoToken("")
		return
	}
	if resp.StatusCode != 200 {
		ssp.SetTaobaoToken("")
		return
	}
	tokenBytes, _ := ioutil.ReadAll(resp.Body)
	ssp.SetTaobaoToken(string(tokenBytes))
}

func (s *Service) statistic() {
	for {
		time.Sleep(time.Second)
		s.l.Println(s.stat.QpsStat())
		moreStr := s.stat.MoreQpsStat()
		if len(moreStr) > 0 {
			s.l.Println(moreStr)
		}
	}
}

type tmpl struct {
	Id      int    `json:"id"`
	Tl      string `json:"tl,omitempty"`
	Active  int    `json:"active"`             // 1: on, 2: off, 3: 只关闭无感
	FbId    string `json:"fb_id,omitempty"`    // facebook id
	AdmobId string `json:"admob_id,omitempty"` // admob id
}

type tmplForHotFix struct {
	Id      string `json:"id"`
	Tl      string `json:"tl,omitempty"`
	Active  int    `json:"active"`             // 1: on, 2: off, 3: 只关闭无感
	FbId    string `json:"fb_id,omitempty"`    // facebook id
	AdmobId string `json:"admob_id,omitempty"` // admob id
}

type tlResp struct {
	// Tpls       []tmpl         `json:"template"` // for hot fix 柚宝宝(slot_id: f847f2c3)
	Tpls       interface{}    `json:"template"`
	Conf       map[string]int `json:"conf"`
	ActiveConf map[string]int `json:"active_conf"` // 广告开关，防止客户端崩溃，临时设置为所有广告均开启

	Xd      *ssp.PreClickController `json:"xd,omitempty"`
	Ag      *ssp.ActiveGuard        `json:"ag,omitempty"`
	Vc      *ssp.VideoControl       `json:"video"`
	Monitor map[string]string       `json:"monitor,omitempty"`
	Update  int                     `json:"update"`
	Error   string                  `json:"error"`
	Status  int                     `json:"status"` // 0:正常; 1:异常; 2:不用更新
	UseGzip bool                    `json:"-"`
}

func newTlResp(err string, status, updateTs int) *tlResp {
	return &tlResp{
		Tpls: []tmpl{},
		Conf: make(map[string]int),
		ActiveConf: map[string]int{
			"fb":   1, // 1: on, 2: off
			"ct":   1,
			"ad_d": 1,
			"ad_c": 1,
		},
		Error:   err,
		Status:  status,
		Update:  updateTs,
		UseGzip: false,
	}
}

func (resp *tlResp) SetGzip() {
	resp.UseGzip = true
}

func (resp *tlResp) WriteJson(w http.ResponseWriter) (int, error) {
	// status == 2 返回{"status":2}
	if resp.Status == 2 {
		return w.Write([]byte(`{"status":2}`))
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	b, _ := json.Marshal(resp)
	if resp.UseGzip && len(b) > 1000 {
		enc := gzip.NewWriter(w)
		defer enc.Close()
		w.Header().Set("Content-Encoding", "gzip")
		return enc.Write(b)
	}
	return w.Write(b)
}

func shouldUpdateSlots(slots []*ssp.SlotInfo, ts int) (should bool, newUpdateTs int) {
	for _, slot := range slots {
		if slot.UpdateTime > newUpdateTs {
			newUpdateTs = slot.UpdateTime
		}
		if slot.UpdateTime > ts {
			should = true
		}
	}
	return
}

func (s *Service) printWithRate(msg ...interface{}) {
	if rand.Intn(100) == 0 {
		s.l.Println(msg...)
	}

}

var hotFixSlotIds map[string]bool = map[string]bool{
	"f847f2c3": true,
	"9c3a79b2": true,
	"c85b9123": true,
	"f48141df": true,
	"e9ce275c": true,
	"a26d7f31": true,
	"472bb332": true,
	"27f471ba": true,
	"079a01f3": true,
	"360551bd": true,
}

func (s *Service) tlAllHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		resp := newTlResp("method: GET required", 1, 0)
		resp.WriteJson(w)
		return
	}

	s.stat.IncrTot()

	r.ParseForm()
	slotId := r.Form.Get("slot_id")
	updateTimeStr := r.Form.Get("update_time")

	shouldFix := hotFixSlotIds[slotId] // hot fix for 8 characters slot id

	var updateTime int
	var err error

	if updateTime, err = strconv.Atoi(updateTimeStr); err != nil {
		s.stat.IncrUpdateTimeErrTot()
		s.printWithRate("update_time: interger required", updateTimeStr)
		resp := newTlResp("update_time: interger required", 1, 0)
		resp.WriteJson(w)
		return
	}

	slotStore := ssp.GetGlobalSlotStore()
	if slotStore == nil {
		s.stat.IncrNoDataTot()
		resp := newTlResp("no data", 1, 0)
		resp.WriteJson(w)
		return
	}
	slots, slotErr := slotStore.GetAllSlotInApp(slotId)
	if slotErr != nil {
		s.stat.IncrGetSlotErrTot()
		s.printWithRate("get all slots err", slotErr)
		resp := newTlResp(slotErr.Error(), 1, 0)
		resp.WriteJson(w)
		return
	}
	shouldUpdate, newTs := shouldUpdateSlots(slots, updateTime)
	if !shouldUpdate {
		s.stat.IncrNoUpdateTot()
		resp := newTlResp("", 2, newTs)
		resp.WriteJson(w)
		return
	}

	tpls := make([]tmpl, len(slots))
	for i, slot := range slots {
		intId, err := strconv.Atoi(slot.IdStr)
		if err != nil {
			// XXX: hot fix for 8 characters slot id
			if hotFixSlotIds[slot.IdStr] {
				shouldFix = true
				break // XXX 既然下面需要hot fix这里就跳出吧
			}
			tpls[i].Id = slot.Id // XXX: 对于v3版本，这里无法升级使用slot.IdStr
		} else {
			tpls[i].Id = intId
		}
		if slot.SlotSwitch != 2 && len(slot.Templates) > 0 {
			if len(slot.Third.FbId) > 0 || len(slot.Third.AdmobId) > 0 { // 这里只有slot有聚合才给h5
				b, _ := util.Base64Encode([]byte(slot.Templates[0].H5))
				tpls[i].Tl = string(b)
			}
			if slot.PreClick == 2 {
				tpls[i].Active = 3 // active = 3: 表示关闭增效广告
			} else {
				tpls[i].Active = 1
			}
			tpls[i].FbId = slot.Third.FbId
			tpls[i].AdmobId = slot.Third.AdmobId
		} else {
			tpls[i].Tl = ""
			tpls[i].Active = 2
		}
	}

	// XXX: hot fix for 8 characters slot id
	var tplsHotFix []tmplForHotFix
	if shouldFix {
		tplsHotFix = make([]tmplForHotFix, len(slots))
		for i, slot := range slots {
			if len(slot.IdStr) == 0 {
				tplsHotFix[i].Id = strconv.Itoa(slot.Id)
			} else {
				tplsHotFix[i].Id = slot.IdStr
			}

			if slot.SlotSwitch == 1 && len(slot.Templates) > 0 {
				if len(slot.Third.FbId) > 0 || len(slot.Third.AdmobId) > 0 {
					b, _ := util.Base64Encode([]byte(slot.Templates[0].H5))
					tplsHotFix[i].Tl = string(b)
				}
				if slot.PreClick == 2 {
					tplsHotFix[i].Active = 3
				} else {
					tplsHotFix[i].Active = 1
				}
				tplsHotFix[i].FbId = slot.Third.FbId
				tplsHotFix[i].AdmobId = slot.Third.AdmobId
			} else {
				tplsHotFix[i].Tl = ""
				tplsHotFix[i].Active = 2
			}
		}
	}

	resp := newTlResp("", 0, newTs)
	if shouldFix {
		resp.Tpls = tplsHotFix
	} else {
		resp.Tpls = tpls
	}

	if len(slots) > 0 {
		resp.Conf = slots[0].Priority
	}

	// 是否需要缓存广告
	if ssp.IsVideoCache(slots) {
		resp.Vc = &ssp.VideoControl{
			VNum:       3,
			VCap:       -1,
			IsPreload:  1,
			NoWifiLoad: 0,
		}
	} else {
		resp.Vc = &ssp.VideoControl{
			VNum:       3,
			VCap:       -1,
			IsPreload:  0,
			NoWifiLoad: 0,
		}
	}

	if accEnc := r.Header.Get("Accept-Encoding"); strings.Contains(accEnc, "gzip") {
		resp.SetGzip()
	}
	resp.WriteJson(w)
	return
}

type tlSingleResp struct {
	Error    string `json:"error"`
	Tpl      string `json:"template"`
	PreClick bool   `json:"pre_click"`

	WhiteListFilter []string `json:"whitelist_filter"`
	BlackListFilter []string `json:"blacklist_filter"`
}

func newTlSingleResp(err string, slot *ssp.SlotInfo) *tlSingleResp {
	resp := &tlSingleResp{
		Error: err,
	}
	if slot != nil && len(slot.Templates) > 0 {
		resp.Tpl = slot.Templates[0].H5
		if slot.PreClick == 1 {
			resp.PreClick = true
		}
	}
	return resp
}

func (resp *tlSingleResp) WriteJson(w http.ResponseWriter) {
	b, _ := json.Marshal(resp)
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.Write(b)
}

func tlHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		resp := newTlSingleResp("method: GET required", nil)
		resp.WriteJson(w)
		return
	}

	r.ParseForm()
	slotId := r.Form.Get("slot_id")

	slotStore := ssp.GetGlobalSlotStore()
	if slotStore == nil {
		resp := newTlSingleResp("no data", nil)
		resp.WriteJson(w)
		return
	}

	slot := slotStore.Get(slotId)
	if slot == nil {
		resp := newTlSingleResp("no slot", nil)
		resp.WriteJson(w)
		return
	}

	if slot.SlotSwitch != 1 {
		resp := newTlSingleResp("slot closed", nil)
		resp.WriteJson(w)
		return
	}

	resp := newTlSingleResp("ok", slot)
	resp.WhiteListFilter = slot.WhiteListFilter
	resp.BlackListFilter = slot.BlackListFilter
	resp.WriteJson(w)
}

func (s *Service) SlotDumper(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	slotStore := ssp.GetGlobalSlotStore()
	if slotStore == nil {
		w.Write([]byte("[]"))
		return
	}
	slotStore.DumpAllSlots(w)
}

func (s *Service) AppDumper(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	appStore := ssp.GetGlobalAppStore()
	if appStore == nil {
		w.Write([]byte("[]"))
		return
	}
	appStore.DumpAllApps(w)
}

func StartTlServer(conf *Conf) {
	s, err := NewService(conf)
	if err != nil {
		panic(err)
	}
	if s != nil {
		go s.updateApps()
	}

	go s.statistic()

	http.HandleFunc("/get_tl", tlHandler)               // for debug (v2)
	http.HandleFunc("/get_all_tl", s.tlAllHandler)      // v2 interface
	http.HandleFunc("/get_all_tl_v3", s.tlAllV3Handler) // v3 interface

	http.HandleFunc("/get_life", s.lifeHandler)

	http.HandleFunc("/dump_slot", s.SlotDumper) // slot dumper
	http.HandleFunc("/dump_app", s.AppDumper)   // app dumper

	http.HandleFunc("/s2s_callback", s.S2sCallback) // s2s callback

	panic(http.ListenAndServe(":33333", nil))
}
