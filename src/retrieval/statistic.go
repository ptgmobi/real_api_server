package retrieval

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
)

var one int64 = int64(1)

func incr(n *int64) int64 {
	return atomic.AddInt64(n, one)
}

type Statistic struct {
	TotalReq        int64 `json:"total_req"`
	TotalReqLastSec int64 `json:"qps"`

	SdkStat     SubStatistic `json:"sdk"`
	NatStat     SubStatistic `json:"native"`
	RtStat      SubStatistic `json:"retriveal"`
	PmtStat     SubStatistic `json:"promote"`
	WuganStat   SubStatistic `json:"wugan"`
	JstagStat   SubStatistic `json:"jstag"`
	InmobiStat  SubStatistic `json:"inmobi"`
	RltStat     SubStatistic `json:"realtime"`
	JstagH5Stat SubStatistic `json:"jstag_h5"`
	LifeStat    SubStatistic `json:"life"`
}

func (stat *Statistic) QpsStat() string {
	stat.TotalReq = atomic.LoadInt64(&stat.SdkStat.TotReq) +
		atomic.LoadInt64(&stat.NatStat.TotReq) +
		atomic.LoadInt64(&stat.RtStat.TotReq)

	delta := stat.TotalReq - stat.TotalReqLastSec
	stat.TotalReqLastSec = stat.TotalReq

	return fmt.Sprint(">>> total req:", stat.TotalReq, ", qps: ", delta)
}

func (stat *Statistic) IncrTot() int64 {
	return incr(&stat.TotalReq)
}

func (stat *Statistic) GetSdkStat() *SubStatistic {
	return &stat.SdkStat
}

func (stat *Statistic) GetNatStat() *SubStatistic {
	return &stat.NatStat
}

func (stat *Statistic) GetRtStat() *SubStatistic {
	return &stat.RtStat
}

func (stat *Statistic) GetPmtStat() *SubStatistic {
	return &stat.PmtStat
}

func (stat *Statistic) GetWuganStat() *SubStatistic {
	return &stat.WuganStat
}

func (stat *Statistic) GetInmobiStat() *SubStatistic {
	return &stat.InmobiStat
}

func (stat *Statistic) GetJstagStat() *SubStatistic {
	return &stat.JstagStat
}

func (stat *Statistic) GetRltStat() *SubStatistic {
	return &stat.RltStat
}

func (stat *Statistic) GetJstagH5Stat() *SubStatistic {
	return &stat.JstagH5Stat
}

func (stat *Statistic) GetLifeStat() *SubStatistic {
	return &stat.LifeStat
}

type SubStatistic struct {
	TotReq int64 `json:"total_req"`

	CtxErr           int64 `json:"ctx_err"`
	DnfNil           int64 `json:"dnf_nil"`
	TplNoMatch       int64 `json:"tpl_no_match"`
	TplSizeErr       int64 `json:"tpl_size_0"`
	ImgWH0           int64 `json:"img_wh_0"`
	RetrievalFilted  int64 `json:"retrieval_filted"`
	RankFilted       int64 `json:"rank_filted"`
	RawAdReq         int64 `json:"raw_ad_req"`
	Imp              int64 `json:"impression"`
	WuganFilted      int64 `json:"wugan_filted"`
	InsPkgErr        int64 `json:"ins_pkg_err"`
	Untouch          int64 `json:"untouch"`
	ImpRateFilted    int64 `json:"imp_rate_filted"`
	PmtInvalidFilted int64 `json:"pmt_invalid_filted"`
}

// to avoid race warning
func (sub *SubStatistic) Load() *SubStatistic {
	return &SubStatistic{
		TotReq:           atomic.LoadInt64(&sub.TotReq),
		CtxErr:           atomic.LoadInt64(&sub.CtxErr),
		DnfNil:           atomic.LoadInt64(&sub.DnfNil),
		TplNoMatch:       atomic.LoadInt64(&sub.TplNoMatch),
		TplSizeErr:       atomic.LoadInt64(&sub.TplSizeErr),
		ImgWH0:           atomic.LoadInt64(&sub.ImgWH0),
		RetrievalFilted:  atomic.LoadInt64(&sub.RetrievalFilted),
		RankFilted:       atomic.LoadInt64(&sub.RankFilted),
		RawAdReq:         atomic.LoadInt64(&sub.RawAdReq),
		Imp:              atomic.LoadInt64(&sub.Imp),
		WuganFilted:      atomic.LoadInt64(&sub.WuganFilted),
		InsPkgErr:        atomic.LoadInt64(&sub.InsPkgErr),
		Untouch:          atomic.LoadInt64(&sub.Untouch),
		ImpRateFilted:    atomic.LoadInt64(&sub.ImpRateFilted),
		PmtInvalidFilted: atomic.LoadInt64(&sub.PmtInvalidFilted),
	}
}

func (sub *SubStatistic) ToString() string {
	dup := sub.Load()
	b, _ := json.Marshal(dup)
	return string(b)
}

func (sub *SubStatistic) IncrTot() int64 {
	return incr(&sub.TotReq)
}

func (sub *SubStatistic) IncrCtxErr() int64 {
	return incr(&sub.CtxErr)
}

func (sub *SubStatistic) IncrDnfNil() int64 {
	return incr(&sub.DnfNil)
}

func (sub *SubStatistic) IncrTplNoMatch() int64 {
	return incr(&sub.TplNoMatch)
}

func (sub *SubStatistic) IncrImgSizeErr() int64 {
	return incr(&sub.ImgWH0)
}

func (sub *SubStatistic) IncrTplSizeErr() int64 {
	return incr(&sub.TplSizeErr)
}

func (sub *SubStatistic) IncrRetrievalFilted() int64 {
	return incr(&sub.RetrievalFilted)
}

func (sub *SubStatistic) IncrRankFilted() int64 {
	return incr(&sub.RankFilted)
}

func (sub *SubStatistic) IncrRawAdReq() int64 {
	return incr(&sub.RawAdReq)
}

func (sub *SubStatistic) IncrImp() int64 {
	return incr(&sub.Imp)
}

func (sub *SubStatistic) IncrWuganFilted() int64 {
	return incr(&sub.WuganFilted)
}

func (sub *SubStatistic) IncrUntouch() int64 {
	return incr(&sub.Untouch)
}

func (sub *SubStatistic) IncrInsPkgErr() int64 {
	return incr(&sub.InsPkgErr)
}

func (sub *SubStatistic) IncrImpRateFilted() int64 {
	return incr(&sub.ImpRateFilted)
}

func (sub *SubStatistic) IncrPmtInvalidFilted() int64 {
	return incr(&sub.PmtInvalidFilted)
}
