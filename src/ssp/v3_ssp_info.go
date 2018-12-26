package ssp

import (
	"util"
)

type AppSlot struct {
	Id      string `json:"id"`
	FbId    string `json:"fb_id,omitempty"`
	AdmobId string `json:"admob_id,omitempty"`
	Active  int    `json:"active"`
	Format  int    `json:"format"`
	TplId   int    `json:"tpl_id"`
}

type AppTpl struct {
	Id  int    `json:"id"`
	Tpl string `json:"tpl"`
}

type RewardedVideoItem struct {
	VOrient     int    `json:"v_orient"`     // 视频播放朝向
	LoadTime    int    `json:"load_time"`    // 应用内开启时提前加载final url事件节点
	ClickTime   int    `json:"click_time"`   // 发送click url的事件节点
	IsMute      int    `json:"is_mute"`      // 1:不静音（默认）, 2:静音
	IsInner     int    `json:"is_inner"`     // 1：应用内开（缺省默认） 2：外开appstore跳转
	ButtonColor string `json:"button_color"` // 插屏下载按钮配色
	PpUrl       string `json:"pp_url"`       // privacy policy url 地址
	Name        string `json:"name"`
	Value       string `json:"value"`
	Slot        string `json:"slot"`
}

// see https://github.com/cloudadrd/Document/blob/master/slot_conf_v3.md for details
type VIToken struct {
	AppName    string   `json:"appname"`
	AppId      string   `json:"appid"`
	Placements []string `json:"placements"`
}

type VISlot struct {
	Id         string            `json:"id"`
	Priority   map[string]int    `json:"priority"`
	MaxPlay    map[string]int    `json:"max_play"`
	Placements map[string]string `json:"placements"`
}

type VideoIntegration struct {
	Tokens []*VIToken `json:"token"`
	Slots  []*VISlot  `json:"slot"`

	appIdHelper        map[string]string
	appPlacementHelper map[string][]string
}

type AppSlotManager struct {
	Slots            []*AppSlot           `json:"slot"`
	Tpls             []*AppTpl            `json:"template"`
	RewardedVideos   []*RewardedVideoItem `json:"rewarded_video"`
	VideoIntegration *VideoIntegration    `json:"video_integration"`

	tplMap map[string]int
	tplIdx int
}

func NewAppSlotManager() *AppSlotManager {
	return &AppSlotManager{
		tplMap: make(map[string]int, 2),
		tplIdx: 0,
	}
}

func (manager *AppSlotManager) AddSlot(slot *SlotInfo) {
	if slot.SlotSwitch != 1 {
		// slot closed
		return
	}

	tplId := 0 // default value

	// add template (没有继承fb或admob就不用写模板)
	if (len(slot.Third.FbId) > 0 || len(slot.Third.AdmobId) > 0) && len(slot.Templates) > 0 {
		sign := slot.Templates[0].Sign
		tplId = manager.tplMap[sign]
		if tplId == 0 {
			// tplId为0表示该模板不存在
			manager.tplIdx++
			tplId = manager.tplIdx
			tplBase64, _ := util.Base64Encode([]byte(slot.Templates[0].H5))
			manager.Tpls = append(manager.Tpls, &AppTpl{
				Id:  tplId,
				Tpl: string(tplBase64),
			})
			manager.tplMap[sign] = tplId
		}
	}

	// add slot
	appSlot := &AppSlot{
		Id:      slot.IdStr,
		FbId:    slot.Third.FbId,
		AdmobId: slot.Third.AdmobId,
		Active:  slot.SlotSwitch,
		Format:  slot.Format,
		TplId:   tplId,
	}
	if slot.SlotSwitch == 1 && slot.PreClick == 2 {
		appSlot.Active = 3
	}
	manager.Slots = append(manager.Slots, appSlot)

	if slot.Format != 6 && slot.Format != 7 {
		// 下面是处理视频slot的部分
		return
	}

	// add rewarded video config
	if len(slot.RewardedAmount) > 0 {
		if len(slot.RewardedCurrency) == 0 {
			slot.RewardedCurrency = "COIN"
		}
		manager.RewardedVideos = append(manager.RewardedVideos, &RewardedVideoItem{
			VOrient:     slot.VideoScreenType,
			ClickTime:   slot.ClickTime,
			IsMute:      1,
			IsInner:     slot.IsInner,
			LoadTime:    slot.LoadTime,
			ButtonColor: slot.ButtonColor,
			PpUrl:       "http://en.yeahmobi.com/privacy-policy/",
			Name:        slot.RewardedCurrency,
			Value:       slot.RewardedAmount,
			Slot:        slot.IdStr,
		})
	}

	// add video integration config
	if len(slot.VideoIntegration) > 0 || len(manager.RewardedVideos) > 0 {
		if manager.VideoIntegration == nil {
			manager.VideoIntegration = &VideoIntegration{
				Tokens:             make([]*VIToken, 0, 1),
				Slots:              make([]*VISlot, 0, 1),
				appIdHelper:        make(map[string]string, len(slot.VideoIntegration)),
				appPlacementHelper: make(map[string][]string, len(slot.VideoIntegration)),
			}
		}

		viSlot := &VISlot{
			Id:         slot.IdStr,
			Priority:   slot.VideoSort,
			MaxPlay:    make(map[string]int, 1+len(slot.VideoIntegration)),
			Placements: make(map[string]string, 1+len(slot.VideoIntegration)),
		}

		for _, viobj := range slot.VideoIntegration {
			if viobj.Status != 1 {
				// slot closed
				continue
			}
			viSlot.MaxPlay[viobj.Type] = 3 // use default value
			viSlot.Placements[viobj.Type] = viobj.ApiPlacementId

			manager.VideoIntegration.appIdHelper[viobj.Type] = viobj.ApiAppId
			manager.VideoIntegration.appPlacementHelper[viobj.Type] = append(
				manager.VideoIntegration.appPlacementHelper[viobj.Type],
				viobj.ApiPlacementId)
		}

		viSlot.MaxPlay["cloudmobi"] = -1

		manager.VideoIntegration.Slots = append(manager.VideoIntegration.Slots, viSlot)
	}
}

func (manager *AppSlotManager) genToken() {
	if manager.VideoIntegration != nil {
		for appName, appId := range manager.VideoIntegration.appIdHelper {
			manager.VideoIntegration.Tokens = append(
				manager.VideoIntegration.Tokens, &VIToken{
					AppName:    appName,
					AppId:      appId,
					Placements: manager.VideoIntegration.appPlacementHelper[appName],
				})
		}
	}
}

func (app *AppStore) GenAuxiliaryGlobalConf() {
	slotStore := GetGlobalSlotStore()
	if slotStore == nil {
		return
	}
	for _, appInfo := range app.m {
		manager := NewAppSlotManager()
		for _, slotId := range appInfo.Slots {
			if slotInfo := slotStore.Get(slotId); slotInfo != nil {
				manager.AddSlot(slotInfo)
			}
		}
		manager.genToken()
		appInfo.Manager = manager
	}
}
