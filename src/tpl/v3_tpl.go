package tpl

import (
	"compress/gzip"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"ssp"
)

type tlV3Resp struct {
	Status  int               `json:"status"`
	Error   string            `json:"error"`
	Monitor map[string]string `json:"monitor,omitempty"`
	Update  int               `json:"update"`

	Tpls  []*ssp.AppTpl  `json:"template,omitempty"`
	Slots []*ssp.AppSlot `json:"slot,omitempty"`

	Conf       map[string]int `json:"conf,omitempty"`        // {"fb":0, "ad_c":1, ...}
	ActiveConf map[string]int `json:"active_conf,omitempty"` // {"fb":1, "ad_c":2, ...}

	Xd *ssp.PreClickController `json:"xd,omitempty"`
	Ag *ssp.ActiveGuard        `json:"ag,omitempty"`

	Vc *ssp.VideoControl        `json:"video,omitempty"`
	Rv []*ssp.RewardedVideoItem `json:"rewarded_video,omitempty"`
	Vi *ssp.VideoIntegration    `json:"video_integration,omitempty"`

	DomainList   []string `json:"d_list,omitempty"`
	DomainReport string   `json:"d_report,omitempty"`
	DomainStatus int      `json:"d_status"` // https://github.com/cloudadrd/Document/blob/master/slot_conf_v3.md

	TaoBaoKe string `json:"tbk,omitempty"`

	ThirdClkMoniter string `json:"third_clk_monitor,omitempty"`
	ThirdImpMoniter string `json:"third_imp_monitor,omitempty"`

	Subs     int      `json:"subs"`     // 订阅开关：1代表请求订阅广告，2代表不请求订阅广告
	SubsSdks []string `json:"subs_sdk"` // 订阅sdk开关，代表打开对应渠道的订阅sdk

	appManager *ssp.AppSlotManager

	useGzip bool
}

type videoSort struct {
	SlotId   string   `json:"slot"`
	UserId   string   `json:"user_id"`
	Country  string   `json:"country"`
	Platform string   `json:"platform"`
	Sources  []Source `json:"sources"`
}

type Source struct {
	Name      string `json:"id"`
	Placement string `json:"pid"`
}

type videoSortData struct {
	Name string  `json:"id"`
	Ecpm float32 `json:"ecpm"`
}

type Datas []videoSortData

type videoSortResp struct {
	ErrMsg string `json:"errmsg"`
	Method string `json:"method"`
	Tot    int    `json:"tot"`
	Data   Datas  `json:"data"`
}

func (d Datas) Len() int           { return len(d) }
func (d Datas) Less(i, j int) bool { return d[i].Ecpm > d[j].Ecpm }
func (d Datas) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }

var videoSortUrl string

func getSortResult(vs *videoSort, userHash string) *videoSortResp {
	url := videoSortUrl + userHash
	jsonVs, err := json.Marshal(vs)
	if err != nil {
		log.Println("[getSortResult] marshal videoSort err: ", err, " vs: ", *vs)
		return nil
	}

	resp, err := http.Post(url, "application/json; charset=utf-8", strings.NewReader(string(jsonVs)))
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		log.Println("[getSortResult] post err: ", err, " body: ", string(jsonVs))
		return nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("[getSortResult] read body err: ", err, " body: ", string(body))
		return nil
	}

	var vsr videoSortResp
	if err := json.Unmarshal(body, &vsr); err != nil {
		log.Println("[getSortResult] Unmarshal body err: ", err, " body: ", string(body))
		return nil
	}
	return &vsr
}

func sortVideo(app *ssp.AppInfo, manager *ssp.AppSlotManager, slotId, userId, country, userHash string) {
	var platform string
	if app.Platform == 1 {
		platform = "Android"
	} else if app.Platform == 2 {
		platform = "iOS"
	} else {
		platform = ""
	}
	sources := make([]Source, 0, 8)
	if manager.VideoIntegration == nil || len(manager.VideoIntegration.Tokens) == 0 || len(manager.VideoIntegration.Slots) == 0 {
		return
	}
	for _, t := range manager.VideoIntegration.Tokens {
		for _, p := range t.Placements {
			var s = Source{
				Name:      t.AppName,
				Placement: p,
			}
			sources = append(sources, s)
		}

	}
	var vs = videoSort{
		SlotId:   slotId,
		UserId:   userId,
		Country:  country,
		Platform: platform,
		Sources:  sources,
	}
	vsr := getSortResult(&vs, userHash)
	if vsr == nil {
		return
	}
	if vsr.ErrMsg != "ok" {
		log.Println("[sortVideo] sort err: ", vsr.ErrMsg)
		return
	}
	sort.Sort(vsr.Data)
	slots := manager.VideoIntegration.Slots
	for j := 0; j < len(slots); j++ {
		priority := 0
		if slots[j].Id == slotId { // 找到排序的slot
			for i := 0; i < len(vsr.Data); i++ { // 给不同的优先级
				if _, ok := slots[j].Priority[vsr.Data[i].Name]; ok {
					slots[j].Priority[vsr.Data[i].Name] = priority
					priority++
				}
			}
			if _, ok := slots[j].Priority["cloudmobi"]; ok && priority != 0 {
				slots[j].Priority["cloudmobi"] = priority
			}
			break
		}
	}
}

func newV3TlResp(err string, status, updateTs int) *tlV3Resp {
	if status != 0 {
		return &tlV3Resp{
			Status: status,
			Update: updateTs,
			Error:  err,
		}
	}

	return &tlV3Resp{
		ActiveConf: map[string]int{ // default value
			"fb":   1, // 1: on, 2: off
			"ct":   1,
			"ad_d": 1,
			"ad_c": 1,
		},
		Error:  "ok",
		Status: 0,
		Update: updateTs,
	}
}

func (resp *tlV3Resp) SetGzip() {
	resp.useGzip = true
}

func (resp *tlV3Resp) WriteJson(w http.ResponseWriter) (int, error) {
	// status == 2 返回{"status":2}
	if resp.Status == 2 {
		return w.Write([]byte(`{"status":2}`))
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	b, err := json.Marshal(resp)
	if err != nil {
		return 0, err
	}
	if resp.useGzip && len(b) > 1000 {
		enc := gzip.NewWriter(w)
		defer enc.Close()
		w.Header().Set("Content-Encoding", "gzip")
		return enc.Write(b)
	}
	return w.Write(b)
}

func (s *Service) tlAllV3Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		resp := newV3TlResp("method: GET required", 1, 0)
		resp.WriteJson(w)
		return
	}

	s.stat.IncrTot()

	r.ParseForm()

	slotId := r.Form.Get("slot_id")
	userId := r.Form.Get("user_id")
	country := r.Form.Get("country")
	updateTimeStr := r.Form.Get("update_time")
	userHash := r.Form.Get("user_hash")

	var updateTime int
	var err error

	if updateTime, err = strconv.Atoi(updateTimeStr); err != nil {
		s.stat.IncrUpdateTimeErrTot()
		s.printWithRate("update_time: interger required", updateTimeStr)
		newV3TlResp("update_time: interger required", 1, 0).WriteJson(w)
		return
	}

	slotStore := ssp.GetGlobalSlotStore()
	if slotStore == nil {
		s.stat.IncrNoDataTot()
		newV3TlResp("no data", 1, 0).WriteJson(w)
		return
	}

	slots, slotErr := slotStore.GetAllSlotInApp(slotId)
	if slotErr != nil {
		s.stat.IncrGetSlotErrTot()
		s.printWithRate("get all slots err", slotErr)
		newV3TlResp(slotErr.Error(), 1, 0).WriteJson(w)
		return
	}

	if len(slots) == 0 {
		newV3TlResp("no slot", 1, 0).WriteJson(w)
		return
	}

	var taobaoTokenEnc string
	taobaoTokenEnc = slotStore.GetTbkInfo(slotId)

	shouldUpdate, newTs := shouldUpdateSlots(slots, updateTime)
	if !shouldUpdate && len(taobaoTokenEnc) == 0 {
		s.stat.IncrNoUpdateTot()
		newV3TlResp("", 2, newTs).WriteJson(w)
		return
	}

	firstSlot := slots[0]
	appStore := ssp.GetGlobalAppStore()
	if appStore == nil {
		s.stat.IncrNoDataTot()
		newV3TlResp("no data", 1, 0).WriteJson(w)
		return
	}

	app := appStore.Get(firstSlot.AppId)
	if app == nil {
		newV3TlResp("app error", 1, 0).WriteJson(w)
		return
	}

	resp := newV3TlResp("", 0, newTs)
	resp.Conf = firstSlot.Priority
	if ssp.IsVideoCache(slots) {
		resp.Vc = ssp.NewVideoControl(firstSlot, 1)
	} else {
		resp.Vc = ssp.NewVideoControl(firstSlot, 2)
	}

	resp.Subs = 2
	doSub, subsSdk := ssp.GetSubscriptionInfo(slotId)
	if doSub {
		resp.Subs = 1
	}
	resp.SubsSdks = subsSdk

	appManager := app.Manager

	if appManager != nil {
		sortVideo(app, appManager, slotId, userId, country, userHash)
		resp.Rv = appManager.RewardedVideos
		resp.Vi = appManager.VideoIntegration
		resp.Tpls = appManager.Tpls
		resp.Slots = appManager.Slots
	}

	// resp.DomainList = append(resp.DomainList, aes.Encrypt(/* TODO: add some domain */))
	// resp.DomainReport = "http://logger.cloudmobi.net/ios/v1/d_report" // 该日志没人看

	// 关闭所有的openurl拦截
	resp.DomainStatus = 4

	resp.TaoBaoKe = taobaoTokenEnc

	if accEnc := r.Header.Get("Accept-Encoding"); strings.Contains(accEnc, "gzip") {
		resp.SetGzip()
	}
	resp.WriteJson(w)
	return
}
