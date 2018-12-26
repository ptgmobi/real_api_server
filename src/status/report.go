package status

import (
	"bytes"
	"encoding/json"
	"sync"
	"time"
)

type reportInfo struct {
	sync.Mutex
	inited bool

	Utc       string           `json:"utc"`
	Retrieval *retrievalReport `json:"retrieval"`
	Phase     *phaseReport     `json:"phase"`
}

func (info *reportInfo) visit() {
	if info.inited {
		return
	}

	info.Lock()
	defer info.Unlock()

	if info.inited {
		return
	}

	info.inited = true
	info.Retrieval = NewRetrievalReport()
	info.Phase = NewPhaseReport()
}

func (info *reportInfo) clear() {
	info.Lock()
	defer info.Unlock()

	info.inited = false
	info.Retrieval = nil
	info.Phase = nil
}

func (info *reportInfo) getReports(t time.Time) (buff *bytes.Buffer) {
	info.Utc = t.Format("2006-01-02 15:04:05")
	info.visit()
	info.Retrieval.preReport()

	buff = bytes.NewBuffer(nil)
	enc := json.NewEncoder(buff)
	if err := enc.Encode(info); err != nil {
		panic(err)
	}
	return
}

type report struct {
	Info [60]reportInfo
}

func (r *report) getReports() *bytes.Buffer {
	now := time.Now().UTC()

	return r.Info[now.Add(-1*time.Minute).Minute()].getReports(now)
}
