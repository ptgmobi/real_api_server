package tpl

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"ssp"
)

type lifeResp struct {
	Status      int         `json:"status"` // 1：更新，2：关闭，3：不用更新，4：其他（参考下面的err_msg）
	Update      int         `json:"update, omitempty"`
	ErrMsg      string      `json:"err_msg"`            // 错误信息
	ServiceType []int       `json:"service, omitempty"` // 0：酒店HOTEL，1：航班FLIGHT，2：交通出行TRAFFIC，3：外卖TAKEOUT，4：美食CATE，5：配送DELIVERY
	Slots       []*lifeSlot `json:"slot, omitempty"`

	lctx *lifeCtx
}

type lifeSlot struct {
	Id     string `json:"id"`     // slot id
	Active int    `json:"active"` // 1：激活， 2：关闭, XXX ssp拉取到的slot都是激活状态，所以这里只会是1
	Format int    `json:"format"` // slot类型
}

type lifeCtx struct { // 初始化请求时会上传的参数，一些参数目前没有用
	AppId      int
	Gaid       string
	Aid        string
	Osv        string // Android: Build.version.sdk
	La         string // 纬度
	Lo         string // 经度
	UpdateTime int    // 上次模板的更新时间，初次上传为0
	Lang       string // 当前系统语言
	Sv         string // sdk版本号
	IsDebug    string // 测试

	useGzip bool
}

func newLifeCtx(r *http.Request) (*lifeCtx, error) {
	r.ParseForm()

	lctx := new(lifeCtx)
	if appIdStr := r.Form.Get("appid"); len(appIdStr) == 0 {
		return nil, fmt.Errorf("[LIFE] appid required")
	} else {
		appId, err := strconv.Atoi(appIdStr)
		if err != nil {
			return nil, fmt.Errorf("[LIFE] appid error")
		}
		lctx.AppId = appId
	}

	gaid := r.Form.Get("gaid")
	aid := r.Form.Get("aid")
	lctx.Gaid = gaid
	lctx.Aid = aid

	updateTimeStr := r.Form.Get("update_time")
	updateTime, err := strconv.Atoi(updateTimeStr)
	if err != nil {
		return nil, fmt.Errorf("[LIFE] update time: interger required")
	}
	lctx.UpdateTime = updateTime

	if isDebug := r.Form.Get("isdebug"); len(isDebug) == 0 {
		lctx.IsDebug = "1"
	} else {
		lctx.IsDebug = isDebug
	}

	return lctx, nil
}

func newLifeResp(status, update int, err string, lctx *lifeCtx) *lifeResp {
	return &lifeResp{
		Status:      status,
		Update:      update,
		ErrMsg:      err,
		ServiceType: []int{0, 1, 2, 3, 4, 5}, // 目前没有给，这里全给上
		lctx:        lctx,
	}
}

func (lp *lifeResp) toWrite(w http.ResponseWriter) (int, error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	b, err := json.Marshal(lp)
	if err != nil {
		return 0, err
	}
	if lp.lctx != nil && lp.lctx.useGzip {
		enc := gzip.NewWriter(w)
		defer enc.Close()
		w.Header().Set("Content-Encoding", "gzip")
		return enc.Write(b)
	}
	return w.Write(b)
}

func getShouldUpdateSlots(slots []*ssp.SlotInfo, ts int) (updateSlots []*lifeSlot, newUpdateTs int, shouldUpdate bool) {
	updateSlots = make([]*lifeSlot, 0, 4)
	for _, slot := range slots {
		updateSlots = append(updateSlots, &lifeSlot{
			Id:     slot.IdStr,
			Active: slot.SlotSwitch, // 1: 表示激活
			Format: slot.Format,
		})

		if slot.UpdateTime > ts {
			shouldUpdate = true
		}

		if slot.UpdateTime > newUpdateTs {
			newUpdateTs = slot.UpdateTime
		}
	}
	return updateSlots, newUpdateTs, shouldUpdate
}

func (s *Service) lifeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		newLifeResp(4, 0, "method: GET required", nil).toWrite(w)
		return
	}

	lctx, err := newLifeCtx(r)
	if err != nil {
		newLifeResp(4, 0, err.Error(), nil).toWrite(w)
		return
	}

	// 根据appid获取所有的slot
	appStore := ssp.GetGlobalAppStore()
	if appStore == nil {
		newLifeResp(4, 0, "no app data", lctx).toWrite(w)
		return
	}
	app := appStore.Get(lctx.AppId)
	if app == nil {
		newLifeResp(4, 0, "app error", lctx).toWrite(w)
		return
	}

	slots, err := app.GetAllSlots()
	if err != nil {
		newLifeResp(4, 0, err.Error(), lctx).toWrite(w)
		return
	}

	shouldUpdateSlots, newTs, shouldUpdate := getShouldUpdateSlots(slots, lctx.UpdateTime)
	if !shouldUpdate { // don't should update
		newLifeResp(3, newTs, "", lctx).toWrite(w)
		return
	}

	resp := newLifeResp(1, newTs, "", lctx)
	resp.Slots = shouldUpdateSlots

	resp.toWrite(w)
	return
}
