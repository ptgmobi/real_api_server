package tpl

import (
	"container/heap"
	"crypto/md5"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type printer interface {
	Println(...interface{})
}

type cbItem struct {
	UniqId  string
	SignKey string
	CbUrl   string

	FailCount  int
	LaunchTime time.Time

	pr printer
}

func NewCbItem(cb, uniq, signKey string, pr printer) *cbItem {
	return &cbItem{
		UniqId:  uniq,
		SignKey: signKey,
		CbUrl:   cb,

		FailCount:  0,
		LaunchTime: time.Now(),

		pr: pr,
	}
}

func (item *cbItem) Callback() bool {
	tsStr := fmt.Sprintf("%d", item.LaunchTime.Unix())
	signStr := fmt.Sprintf("%x", md5.Sum([]byte(item.SignKey+"#"+item.UniqId+"#"+tsStr)))
	c := "?"
	if strings.Contains(item.CbUrl, "?") {
		c = "&"
	}
	url := item.CbUrl + c + "ts=" + tsStr + "&uniq_id=" + item.UniqId

	if item.SignKey != "" {
		url += "&sign=" + signStr
	}

	item.FailCount++
	item.LaunchTime = item.LaunchTime.Add(15 * time.Second) // next retry time

	cli := http.Client{
		Timeout: time.Second * 5,
	}
	resp, err := cli.Get(url)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		item.pr.Println("[S2S] GET cb url: ", url, " error: ", err, ", FailCount: ", item.FailCount)
		return false
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true
	}
	item.pr.Println("[S2S] GET cb url: ", url, " status code error: ", resp.StatusCode, ", FailCount: ", item.FailCount)
	return false
}

type cbHeap []*cbItem

func (h cbHeap) Len() int           { return len(h) }
func (h cbHeap) Less(i, j int) bool { return h[i].LaunchTime.Before(h[j].LaunchTime) }
func (h cbHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *cbHeap) Push(x interface{}) {
	item := x.(*cbItem)
	if item.FailCount > 3 {
		return
	}
	*h = append(*h, item)
}

func (h *cbHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type heapWrapper struct {
	sync.RWMutex
	h cbHeap
}

func NewHeapWrapper() *heapWrapper {
	w := &heapWrapper{}
	heap.Init(&w.h)
	return w
}

func (w *heapWrapper) Push(item *cbItem) {
	w.Lock()
	defer w.Unlock()
	heap.Push(&w.h, item)
}

func (w *heapWrapper) Pop() *cbItem {
	w.Lock()
	defer w.Unlock()
	item := heap.Pop(&w.h)
	if item != nil {
		return item.(*cbItem)
	}
	return nil
}

func (w *heapWrapper) Top() *cbItem {
	w.RLock()
	defer w.RUnlock()
	n := w.h.Len()
	if n <= 0 {
		return nil
	}
	return w.h[n-1]
}

func (w *heapWrapper) Serve() {
	for {
		item := w.Top()
		if item == nil {
			time.Sleep(time.Second)
			continue
		}

		now := time.Now()
		if item.LaunchTime.After(now) {
			time.Sleep(time.Second)
			continue
		}

		for item != nil && item.LaunchTime.Before(now) {
			item = w.Pop()
			if !item.Callback() {
				w.Push(item)
			}
			item = w.Top()
		}
	}
}
