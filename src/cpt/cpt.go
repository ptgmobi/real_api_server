package cpt

import (
	"time"

	dnf "github.com/brg-liuwei/godnf"

	"http_context"
	"raw_ad"
)

type Conf []CptConfItem

type CptConfItem struct {
	Offers    []string      `json:"offers"`
	Slots     []CptSlotInfo `json:"slots"`
	StartTime string        `json:"start_time"`
	EndTime   string        `json:"end_time"`
}

type CptSlotInfo struct {
	SlotId string `json:"slot_id"`
	ImgH   int    `json:"img_h"`
	ImgW   int    `json:"img_w"`
}

type Media struct {
	Offers    map[string]bool
	Slots     map[string]MediaSlot
	StartTime time.Time
	EndTime   time.Time
}

type MediaSlot struct {
	ImgH int
	ImgW int
}

var gMediaMap map[string]*Media

var startTimeMinStr string = "1970-01-02 15:00:00"
var startTimeMin time.Time

var endTimeMaxStr string = "2099-01-02 15:00:00"
var endTimeMax time.Time

func initMedia(ci *CptConfItem) error {
	me := &Media{
		Offers: make(map[string]bool, len(ci.Offers)),
		Slots:  make(map[string]MediaSlot, len(ci.Slots)),
	}

	var err error
	if len(ci.StartTime) > 0 {
		if me.StartTime, err = time.Parse("2006-01-02 15:00:00", ci.StartTime); err != nil {
			return err
		}
	} else {
		me.StartTime = startTimeMin
	}

	if len(ci.EndTime) > 0 {
		if me.EndTime, err = time.Parse("2006-01-02 15:00:00", ci.EndTime); err != nil {
			return err
		}
	} else {
		me.EndTime = endTimeMax
	}

	for _, oid := range ci.Offers {
		me.Offers[oid] = true
	}

	for _, slot := range ci.Slots {
		me.Slots[slot.SlotId] = MediaSlot{
			ImgH: slot.ImgH,
			ImgW: slot.ImgW,
		}
		gMediaMap[slot.SlotId] = me
	}

	return nil
}

func Init(cf *Conf) {
	gMediaMap = make(map[string]*Media)

	startTimeMin, _ = time.Parse("2006-01-02 15:00:00", startTimeMinStr)
	endTimeMax, _ = time.Parse("2006-01-02 15:00:00", endTimeMaxStr)

	for i := 0; i != len(*cf); i++ {
		item := (*cf)[i]
		if err := initMedia(&item); err != nil {
			panic("cpt init media error: " + err.Error())
		}
	}
}

func (media *Media) getCptDocs(conds []dnf.Cond, ctx *http_context.Context) (docs []int) {
	if ctx.Now.Before(media.StartTime) || ctx.Now.After(media.EndTime) {
		return nil
	}

	h := dnf.GetHandler()
	if h == nil {
		return nil
	}

	imgwBak, imghBak := ctx.ImgW, ctx.ImgH
	mediaSlot := media.Slots[ctx.SlotId]
	ctx.ImgW, ctx.ImgH = mediaSlot.ImgW, mediaSlot.ImgH

	docs, _ = h.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)
		if !media.Offers[raw.UniqId] {
			return false
		}
		if !raw.HasMatchedCreative(ctx) {
			return false
		}
		return true
	})

	if len(docs) == 0 {
		ctx.ImgW, ctx.ImgH = imgwBak, imghBak
	}

	return
}

func GetCptDocs(conds []dnf.Cond, ctx *http_context.Context) (docs []int) {
	if ctx.IsWugan() {
		return nil
	}

	// appendPmt: 处理点击达到上限后，dnf中加入了pmt in {true}的情况
	appendPmt := true
	for _, cond := range conds {
		if cond.Key == "pmt" {
			appendPmt = false
			break
		}
	}

  if appendPmt {
		conds = append(conds, dnf.Cond{"pmt", "true"})
	}

	if media, ok := gMediaMap[ctx.SlotId]; ok {
		return media.getCptDocs(conds, ctx)
	}

	return nil
}
