package click_counter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

var defaultCounter *ClickCounter

type ClickCounter struct {
	sync.RWMutex
	apiAddr string

	clickCount map[string]int // 当前的offer点击频次
}

type Conf struct {
	ApiAddr string `json:"api_addr"`
	Speed   int    `json:"speed"`
}

func Init(cf *Conf) {
	defaultCounter = NewClickCounter()
	defaultCounter.apiAddr = cf.ApiAddr

	go defaultCounter.Ticker(cf.Speed)
}

func NewClickCounter() *ClickCounter {
	return &ClickCounter{
		clickCount: make(map[string]int, 1024),
	}
}

func (ck *ClickCounter) Ticker(speed int) {
	if speed < 60 {
		speed = 60
	}
	ticker := time.NewTicker(time.Duration(speed) * time.Second)
	for t := range ticker.C {
		fmt.Printf("%v update click_counter\n", t)
		ck.Update()
	}
}

func (ck *ClickCounter) Update() {
	ck.submitClickCount()
}

func Update() {
	defaultCounter.Update()
}

type ClickCtrlResp struct {
	BlackOffers []string `json:"black_offers"`
	ErrMsg      string   `json:"err_msg"`
}

func (ck *ClickCounter) submitClickCount() {
	ck.Lock()
	m := ck.clickCount
	ck.clickCount = make(map[string]int, 1024)
	ck.Unlock()

	type OfferCount struct {
		ClkNum  int    `json:"clk_num"`
		OfferId string `json:"offer_id"`
	}
	var postBody struct {
		Offers []OfferCount `json:"offers"`
	}

	postBody.Offers = make([]OfferCount, 0, len(m))
	for oid, cnt := range m {
		postBody.Offers = append(postBody.Offers, OfferCount{
			OfferId: oid,
			ClkNum:  cnt,
		})
	}

	b, err := json.Marshal(postBody)
	if err != nil {
		fmt.Println("submitClickCount: marshl post body error:", err)
		return
	}

	// XXX
	resp, err := http.Post(ck.apiAddr+"/push", "application/json", bytes.NewReader(b))
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		fmt.Println("submitClickCount post error:", err)
		return
	}
}

func (ck *ClickCounter) AddClick(oid string, nClick int) {
	ck.Lock()
	defer ck.Unlock()
	if ck.clickCount != nil {
		ck.clickCount[oid] += nClick
	}
}

func AddClick(oid string, nClick int) {
	defaultCounter.AddClick(oid, nClick)
}
