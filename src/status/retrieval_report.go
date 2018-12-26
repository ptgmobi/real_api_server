package status

import (
	"container/heap"
	"fmt"
	"sync"
)

type elem struct {
	Req      int    `json:"request"`
	Filled   int    `json:"filled"`
	FillRate string `json:"fill_rate"`
}

func NewElem(r *Request) *elem {
	e := &elem{
		Req:    r.numReq,
		Filled: r.numFilled,
	}
	if e.Req <= 0 {
		e.FillRate = "0%"
	} else {
		e.FillRate = fmt.Sprintf("%.2f%%",
			100*float32(e.Filled)/float32(e.Req))
	}
	return e
}

func (e *elem) AddReq(r *Request) *elem {
	e.Req += r.numReq
	e.Filled += r.numFilled
	if e.Req <= 0 {
		e.FillRate = "0%"
	} else {
		e.FillRate = fmt.Sprintf("%.2f%%",
			100*float32(e.Filled)/float32(e.Req))
	}
	return e
}

type retrievalReport struct {
	sync.Mutex

	iosCountryMap     map[string]*elem
	androidCountryMap map[string]*elem

	iosAdTypeMap     map[string]*elem
	androidAdTypeMap map[string]*elem

	slotMap  map[string]*elem
	offerMap map[string]*elem

	PlatformAdtypeMap       map[string]map[string]*elem `json:"dim_adtype"`
	PlatformTop20CountryMap map[string]map[string]*elem `json:"dim_top20_country"`

	Top20SlotMap  map[string]*elem `json:"dim_top20_slot"`
	Top30OfferMap map[string]*elem `json:"dim_top30_offer"`

	topOk bool
}

func NewRetrievalReport() *retrievalReport {
	return &retrievalReport{
		iosCountryMap:     make(map[string]*elem),
		androidCountryMap: make(map[string]*elem),
		iosAdTypeMap:      make(map[string]*elem),
		androidAdTypeMap:  make(map[string]*elem),

		slotMap:  make(map[string]*elem),
		offerMap: make(map[string]*elem),

		topOk: false,
	}
}

func (report *retrievalReport) recordRequest(r *Request) {
	report.Lock()
	defer report.Unlock()

	if r.platform == "iOS" {
		if e := report.iosAdTypeMap[r.adtype]; e != nil {
			e.AddReq(r)
		} else {
			report.iosAdTypeMap[r.adtype] = NewElem(r)
		}

		if e := report.iosCountryMap[r.country]; e != nil {
			e.AddReq(r)
		} else {
			report.iosCountryMap[r.country] = NewElem(r)
		}
	} else {
		if e := report.androidAdTypeMap[r.adtype]; e != nil {
			e.AddReq(r)
		} else {
			report.androidAdTypeMap[r.adtype] = NewElem(r)
		}

		if e := report.androidCountryMap[r.country]; e != nil {
			e.AddReq(r)
		} else {
			report.androidCountryMap[r.country] = NewElem(r)
		}
	}

	if r.adtype == "8" {
		// 不统计promote
		return
	}

	if e := report.slotMap[r.slotid]; e != nil {
		e.AddReq(r)
	} else {
		report.slotMap[r.slotid] = NewElem(r)
	}

	for _, oid := range r.offers {
		if e := report.offerMap[oid]; e != nil {
			e.Req++
			e.Filled++
		} else {
			report.offerMap[oid] = &elem{
				Req:    1,
				Filled: 1,
			}
		}
	}
}

type Item struct {
	Key  string
	Elem *elem
}

type priorityQueue []*Item

func (pq priorityQueue) Len() int           { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool { return pq[i].Elem.Req > pq[j].Elem.Req }
func (pq priorityQueue) Swap(i, j int) {
	pq[i].Key, pq[j].Key = pq[j].Key, pq[i].Key
	pq[i].Elem, pq[j].Elem = pq[j].Elem, pq[i].Elem
}
func (pq *priorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*Item))
}
func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := old.Len()
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

func setTop(dst, src map[string]*elem, top int) {
	h := &priorityQueue{}
	heap.Init(h)
	for key, e := range src {
		heap.Push(h, &Item{
			Key:  key,
			Elem: e,
		})
	}
	for h.Len() > 0 && top > 0 {
		top--
		item := heap.Pop(h).(*Item)
		dst[item.Key] = item.Elem
	}
}

func (report *retrievalReport) preReport() {
	// for quick return
	if report.topOk {
		return
	}

	report.Lock()
	defer report.Unlock()

	// double check
	if report.topOk {
		return
	}

	iosTopCountry := make(map[string]*elem, 32)
	setTop(iosTopCountry, report.iosCountryMap, 20)
	iosTopCountry["CN"] = report.iosCountryMap["CN"]
	iosTopCountry["TW"] = report.iosCountryMap["TW"]
	iosTopCountry["HK"] = report.iosCountryMap["HK"]
	report.iosCountryMap = nil

	androidTopCountry := make(map[string]*elem, 32)
	setTop(androidTopCountry, report.androidCountryMap, 20)
	androidTopCountry["CN"] = report.androidCountryMap["CN"]
	androidTopCountry["TW"] = report.androidCountryMap["TW"]
	androidTopCountry["HK"] = report.androidCountryMap["HK"]
	report.androidCountryMap = nil

	report.PlatformTop20CountryMap = map[string]map[string]*elem{
		"iOS":     iosTopCountry,
		"Android": androidTopCountry,
	}

	report.PlatformAdtypeMap = map[string]map[string]*elem{
		"iOS":     report.iosAdTypeMap,
		"Android": report.androidAdTypeMap,
	}

	report.Top20SlotMap = make(map[string]*elem, 20)
	setTop(report.Top20SlotMap, report.slotMap, 20)
	report.slotMap = nil // release memory

	report.Top30OfferMap = make(map[string]*elem, 30)
	setTop(report.Top30OfferMap, report.offerMap, 30)
	report.offerMap = nil // release memory

	report.topOk = true
}
