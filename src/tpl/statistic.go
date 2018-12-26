package tpl

import (
	"fmt"
	"math/rand"
	"sync/atomic"
)

type Statistic struct {
	TotalReq        int64
	TotalReqLastSec int64

	TotalSlotErrReq           int64
	TotalSlotErrReqLast       int64
	TotalUpdateTimeErrReq     int64
	TotalUpdateTimeErrReqLast int64
	TotalNoDataReq            int64
	TotalNoDataReqLast        int64
	TotalGetSlotErrReq        int64
	TotalGetSlotErrReqLast    int64
	TotalNoUpdateReq          int64
	TotalNoUpdateReqLast      int64
}

func (stat *Statistic) QpsStat() string {
	qps := stat.TotalReq - stat.TotalReqLastSec
	stat.TotalReqLastSec = stat.TotalReq

	return fmt.Sprint(" >>> totle req: ", stat.TotalReq, " totalQps: ", qps)
}

func (stat *Statistic) MoreQpsStat() string {
	slotErrQps := stat.TotalSlotErrReq - stat.TotalSlotErrReqLast
	stat.TotalSlotErrReqLast = stat.TotalSlotErrReq

	updateTimeErrQps := stat.TotalUpdateTimeErrReq - stat.TotalUpdateTimeErrReqLast
	stat.TotalUpdateTimeErrReqLast = stat.TotalUpdateTimeErrReq

	noDataQps := stat.TotalNoDataReq - stat.TotalNoDataReqLast
	stat.TotalNoDataReqLast = stat.TotalNoDataReq

	getSlotErrQps := stat.TotalGetSlotErrReq - stat.TotalGetSlotErrReqLast
	stat.TotalGetSlotErrReqLast = stat.TotalGetSlotErrReq

	noUpdateQps := stat.TotalNoUpdateReq - stat.TotalNoUpdateReqLast
	stat.TotalNoUpdateReqLast = stat.TotalNoUpdateReq

	if rand.Intn(10) == 0 {
		return fmt.Sprint(" >>> slotErrQps: ", slotErrQps, " updateTimeErrQps: ", updateTimeErrQps,
			" noDataQps: ", noDataQps, " getSlotErrQps: ", getSlotErrQps, " noUpdateQps: ", noUpdateQps)
	}
	return ""
}

var one = int64(1)

func incr(n *int64) int64 {
	return atomic.AddInt64(n, one)
}

func (stat *Statistic) IncrTot() int64 {
	return incr(&stat.TotalReq)
}

func (stat *Statistic) IncrSlotErrTot() int64 {
	return incr(&stat.TotalSlotErrReq)
}

func (stat *Statistic) IncrUpdateTimeErrTot() int64 {
	return incr(&stat.TotalUpdateTimeErrReq)
}

func (stat *Statistic) IncrNoDataTot() int64 {
	return incr(&stat.TotalNoDataReq)
}

func (stat *Statistic) IncrGetSlotErrTot() int64 {
	return incr(&stat.TotalGetSlotErrReq)
}

func (stat *Statistic) IncrNoUpdateTot() int64 {
	return incr(&stat.TotalNoUpdateReq)
}
