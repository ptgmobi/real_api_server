package rank

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"sync/atomic"

	"http_context"
	"rank/video"
	"raw_ad"
)

type Conf struct {
	RankApis     []string `json:"rank_apis"`
	ImpRankApi   string   `json:"imp_rank_api"`
	VideoRankApi string   `json:"video_rank_api"`
}

var (
	conf  *Conf
	iHost int64 = 0
)

var spaceReg *regexp.Regexp

func Init(cf *Conf) {
	spaceReg = regexp.MustCompile("^\\s+$")
	conf = cf
	if len(conf.RankApis) == 0 {
		panic("missing rank_apis[] in conf")
	}
	if len(conf.ImpRankApi) == 0 {
		panic("missing imp_rank_api in conf")
	}
	if len(conf.VideoRankApi) == 0 {
		panic("missing video_rank_api in conf")
	}
	video.Init(cf.VideoRankApi)
}

type Offer struct {
	Id      string  `json:"id"`
	Price   float32 `json:"price"`
	Freq    int     `json:"freq"`
	PkgName string  `json:"pkg"`
	Channel string  `json:"channel"`
	ChType  []int   `json:"c_type"`
	CanPre  int     `json:"can_pre,omitempty"` // 1: on, 2: off
	Settle  string  `json:"settle,omitempty"`  // cpm, cpc, cpi
}

type PostReq struct {
	UserId   string   `json:"user_id"`
	Slot     string   `json:"slot"`
	Country  string   `json:"country"`
	Platform string   `json:"platform"`
	AdNum    int      `json:"ad_num"`
	AdType   string   `json:"adtype"`
	Offers   []*Offer `json:"offers"`
}

type RespData struct {
	Method      string `json:"method"`
	PreClick    int    `json:"pre_clk"`
	LastUrl     string `json:"last_url"`
	LandingType int    `json:"type"` // resty-lua返回值：2表示二代协议，1表示app download
	Id          string `json:"id"`
	Comment     string `json:"comment"`

	Ecpm    float64 `json:"ecpm"`
	Channel string  `json:"channel"`
}

type PostResp struct {
	Tot    int        `json:"tot"`
	ErrMsg string     `json:"errmsg"`
	Method string     `json:"method"`
	Data   []RespData `json:"data"`
}

// setPre returns 1 when preclick is allowed, otherwise 2 returned
func setPre(raw *raw_ad.RawAdObj, ctx *http_context.Context) int {
	if ctx.IsWugan() {
		return 1
	}
	return 2
}

func createPostData(raws []*raw_ad.RawAdObj, ctx *http_context.Context) *PostReq {
	pData := &PostReq{
		Slot:     ctx.SlotId,
		AdNum:    ctx.AdNum,
		UserId:   ctx.UserId,
		Country:  ctx.Country,
		Platform: ctx.Platform,
		AdType:   ctx.AdType,
		Offers:   make([]*Offer, 0, len(raws)),
	}

	for i := 0; i != len(raws); i++ {
		freqCnt := 0
		if ctx.FreqInfo.FreqMap != nil {
			freqCnt = ctx.FreqInfo.FreqMap[raws[i].Id]
		}

		chType := raws[i].GetChannelType()
		if chType == nil {
			chType = []int{}
		}

		tmpOffer := &Offer{
			Id:      raws[i].Id,
			Price:   raws[i].Payout,
			Freq:    freqCnt,
			PkgName: raws[i].AppDownload.PkgName,
			Channel: raws[i].Channel,
			ChType:  chType,
		}

		if ctx.AdType != "0" && ctx.AdType != "1" && ctx.AdType != "2" { // 正常的offer排序不要这两个参数
			tmpOffer.CanPre = setPre(raws[i], ctx)
			tmpOffer.Settle = "cpi" // default value
		}

		pData.Offers = append(pData.Offers, tmpOffer)
	}
	return pData
}

func createPostDataWg(raws []*raw_ad.RawAdObj, ctx *http_context.Context) (*PostReq, []*raw_ad.RawAdObj) {
	pData := &PostReq{
		Slot:     ctx.SlotId,
		UserId:   ctx.UserId,
		Country:  ctx.Country,
		Platform: ctx.Platform,
		AdType:   ctx.AdType,
		Offers:   make([]*Offer, 0, len(raws)),
	}

	whiteOffers := make([]*raw_ad.RawAdObj, 0, 8)
	for i := 0; i != len(raws); i++ {
		if raws[i].IsT {
			whiteOffers = append(whiteOffers, raws[i])
			continue
		}

		freqCnt := 0
		if ctx.FreqInfo.FreqMap != nil {
			freqCnt = ctx.FreqInfo.FreqMap[raws[i].Id]
		}

		chType := raws[i].GetChannelType()
		if chType == nil {
			chType = []int{}
		}

		tmpOffer := &Offer{
			Id:      raws[i].Id,
			Price:   raws[i].Payout,
			Freq:    freqCnt,
			PkgName: raws[i].AppDownload.PkgName,
			Channel: raws[i].Channel,
			ChType:  chType,
		}

		if ctx.AdType != "0" && ctx.AdType != "1" && ctx.AdType != "2" { // 正常的offer排序不要这两个参数
			tmpOffer.CanPre = setPre(raws[i], ctx)
			tmpOffer.Settle = "cpi" // default value
		}

		pData.Offers = append(pData.Offers, tmpOffer)
	}
	pData.AdNum = len(pData.Offers)
	return pData, whiteOffers
}

func RandCopy(raws []*raw_ad.RawAdObj, ctx *http_context.Context, num int) []*raw_ad.RawAdObj {
	var plist []*raw_ad.RawAdObj
	ctx.Method = "randcp"

	if num > len(raws) {
		plist = raws
	} else {
		l := len(raws)
		for i := 0; i != num; i++ {
			n := rand.Intn(l)
			plist = append(plist, raws[n])
			raws[n], raws[l-1] = raws[l-1], raws[n]
			l--
		}
	}

	// copy
	rc := make([]*raw_ad.RawAdObj, 0, len(plist))
	for i := 0; i != len(plist); i++ {
		raw := *plist[i] // value copy
		rc = append(rc, &raw)
	}
	return rc
}

func randomCopy(raws []*raw_ad.RawAdObj, ctx *http_context.Context) []*raw_ad.RawAdObj {
	var plist []*raw_ad.RawAdObj

	ctx.Method = "randcp"

	if ctx.AdNum >= len(raws) {
		plist = raws
	} else {
		l := len(raws)

		for i := 0; i < l; i++ {
			if raws[i].IsT {
				plist = append(plist, raws[i])
				if len(plist) >= ctx.AdNum {
					break
				}
			}
		}

		for i := 0; i < l; i++ {
			if raws[i].IsT {
				continue
			}
			plist = append(plist, raws[i])
			if len(plist) >= ctx.AdNum {
				break
			}
		}
	}

	// copy
	rc := make([]*raw_ad.RawAdObj, 0, len(plist))
	for i := 0; i != len(plist); i++ {
		raw := *plist[i] // value copy
		rc = append(rc, &raw)
	}
	return rc
}

func priorityCopy(pChannel string, raws []*raw_ad.RawAdObj, ctx *http_context.Context) []*raw_ad.RawAdObj {
	var list []*raw_ad.RawAdObj
	var pList []*raw_ad.RawAdObj
	var otherList []*raw_ad.RawAdObj

	ctx.Method = "prioritycp"

	if ctx.AdNum >= len(raws) {
		list = raws
	} else {
		for i := 0; i < len(raws); i++ {
			if raws[i].Channel == pChannel {
				pList = append(pList, raws[i])
			} else {
				otherList = append(otherList, raws[i])
			}
		}
		pListSize := len(pList)
		if ctx.AdNum > pListSize { // 优先出的offer不够
			l := len(otherList)
			for i := 0; i != ctx.AdNum-pListSize; i++ {
				n := rand.Intn(l)
				pList = append(pList, otherList[n])
				otherList[n], otherList[l-1] = otherList[l-1], otherList[n]
				l--
			}
		}
		list = pList[:ctx.AdNum]
	}

	// copy
	rc := make([]*raw_ad.RawAdObj, 0, len(list))
	for i := 0; i != len(list); i++ {
		raw := *list[i] // value copy
		rc = append(rc, &raw)
	}
	return rc
}

func SelectWg(raws []*raw_ad.RawAdObj, ctx *http_context.Context) []*raw_ad.RawAdObj {
	if len(raws) == 0 {
		return nil
	}

	if ctx.AdType == "5" || ctx.AdType == "7" ||
		ctx.AdType == "9" || ctx.AdType == "10" {
		return randomCopy(raws, ctx) // display ad use round robin strategy
	}

	cplOffers := make([]*raw_ad.RawAdObj, 0, 8)
	if !ctx.IsWugan() && ctx.SlotId == "1244" { // 专门给测试cpl的slot
		for i := 0; i < len(raws); i++ {
			if raws[i].PayoutType == "CPL" && len(cplOffers) < ctx.AdNum {
				rawCopy := *raws[i]
				cplOffers = append(cplOffers, &rawCopy)
			}
		}
	}
	if len(cplOffers) > 0 { // 优先出cpl广告
		ctx.Method = "cpl"
		return cplOffers
	}

	pData, whiteOffers := createPostDataWg(raws, ctx)
	if pData.AdNum == 0 {
		rcRaws := make([]*raw_ad.RawAdObj, 0, len(whiteOffers))
		for _, offer := range whiteOffers {
			rawObj := *(offer)
			rcRaws = append(rcRaws, &rawObj)
		}
		return rcRaws
	}

	reqb, _ := json.Marshal(pData)

	n := int64(len(conf.RankApis))
	i := int(atomic.AddInt64(&iHost, int64(1)) % n) // round robin

	var buf bytes.Buffer
	if ctx.RankUseGzip {
		enc := gzip.NewWriter(&buf)
		enc.Write(reqb)
		enc.Close()
	} else {
		buf.Write(reqb)
	}

	var reqUrl string
	if ctx.ImpRank() {
		reqUrl = conf.ImpRankApi + "?user_hash=" + strconv.Itoa(ctx.UserHash)
	} else {
		reqUrl = conf.RankApis[i] + "?user_hash=" + strconv.Itoa(ctx.UserHash)
	}

	req, err := http.NewRequest("POST", reqUrl, &buf)
	if err != nil {
		ctx.L.Println("CTR Request err: ", err)
		return randomCopy(raws, ctx)
	}

	req.Header.Set("Content-Type", "application/json")
	if ctx.RankUseGzip {
		req.Header.Set("Content-Encoding", "gzip")
	}

	client := &http.Client{}
	rep, repErr := client.Do(req)
	if rep != nil {
		defer rep.Body.Close()
	}
	if repErr != nil {
		ctx.L.Println("CTR Response err: ", repErr)
		return randomCopy(raws, ctx)
	}

	b, err := ioutil.ReadAll(rep.Body)
	if err != nil {
		ctx.L.Println("Read CTR Response err: ", err)
		return randomCopy(raws, ctx)
	}

	var repData PostResp
	if err := json.Unmarshal(b, &repData); err != nil {
		ctx.L.Println("Decode Json Response err: ", err,
			", req body:", string(reqb), ", resp body: ", string(b))
		return randomCopy(raws, ctx)
	}

	if repData.ErrMsg != "ok" {
		ctx.L.Println("rank err: ", repData.ErrMsg)
		return randomCopy(raws, ctx)
	}

	if repData.Tot <= 0 {
		return nil
	}

	nTot := repData.Tot
	rcRaws := make([]*raw_ad.RawAdObj, 0, repData.Tot+len(whiteOffers))

	for _, offer := range whiteOffers {
		// raw copy
		rawObj := *(offer)
		rcRaws = append(rcRaws, &rawObj)
	}

	for _, rcData := range repData.Data {
		ctx.Estimate(rcData.Channel + "_" + rcData.Id + " ecpm: " + fmt.Sprintf("%.6f", rcData.Ecpm))
		for i := 0; i != len(raws); i++ {
			if nTot <= 0 {
				return rcRaws
			}

			if raws[i].Id == rcData.Id && raws[i].Channel == rcData.Channel {
				nTot--

				rawObj := *(raws[i])

				rawObj.AttachArgs = append(rawObj.AttachArgs, fmt.Sprintf("e=%f", 1000*rcData.Ecpm)) // anli没有乘1000，我帮你乘！
				rawObj.AttachArgs = append(rawObj.AttachArgs, fmt.Sprintf("po=%f", rawObj.Payout))

				if len(repData.Method) > 0 {
					ctx.Method = repData.Method
				} else {
					ctx.Method = rcData.Method
				}
				rcRaws = append(rcRaws, &rawObj)
				break
			}
		}
	}

	return rcRaws
}

func Select(raws []*raw_ad.RawAdObj, ctx *http_context.Context) []*raw_ad.RawAdObj {
	if len(raws) == 0 {
		return nil
	}

	if ctx.AdType == "5" || ctx.AdType == "7" ||
		ctx.AdType == "9" || ctx.AdType == "10" {
		return randomCopy(raws, ctx) // display ad use round robin strategy
	}

	cplOffers := make([]*raw_ad.RawAdObj, 0, 8)
	if !ctx.IsWugan() && ctx.SlotId == "1244" { // 专门给测试cpl的slot
		for i := 0; i < len(raws); i++ {
			if raws[i].PayoutType == "CPL" && len(cplOffers) < ctx.AdNum {
				rawCopy := *raws[i]
				cplOffers = append(cplOffers, &rawCopy)
			}
		}
	}
	if len(cplOffers) > 0 { // 优先出cpl广告
		ctx.Method = "cpl"
		return cplOffers
	}

	pData := createPostData(raws, ctx)
	reqb, _ := json.Marshal(pData)

	n := int64(len(conf.RankApis))
	i := int(atomic.AddInt64(&iHost, int64(1)) % n) // round robin

	var buf bytes.Buffer
	if ctx.RankUseGzip {
		enc := gzip.NewWriter(&buf)
		enc.Write(reqb)
		enc.Close()
	} else {
		buf.Write(reqb)
	}

	var reqUrl string
	if ctx.ImpRank() {
		reqUrl = conf.ImpRankApi + "?user_hash=" + strconv.Itoa(ctx.UserHash)
	} else {
		reqUrl = conf.RankApis[i] + "?user_hash=" + strconv.Itoa(ctx.UserHash)
	}

	req, err := http.NewRequest("POST", reqUrl, &buf)
	if err != nil {
		ctx.L.Println("CTR Request err: ", err)
		return randomCopy(raws, ctx)
	}

	req.Header.Set("Content-Type", "application/json")
	if ctx.RankUseGzip {
		req.Header.Set("Content-Encoding", "gzip")
	}

	client := &http.Client{}
	rep, repErr := client.Do(req)
	if rep != nil {
		defer rep.Body.Close()
	}
	if repErr != nil {
		ctx.L.Println("CTR Response err: ", repErr)
		return randomCopy(raws, ctx)
	}

	b, err := ioutil.ReadAll(rep.Body)
	if err != nil {
		ctx.L.Println("Read CTR Response err: ", err)
		return randomCopy(raws, ctx)
	}

	var repData PostResp
	if err := json.Unmarshal(b, &repData); err != nil {
		ctx.L.Println("Decode Json Response err: ", err,
			", req body:", string(reqb), ", resp body: ", string(b))
		return randomCopy(raws, ctx)
	}

	if repData.ErrMsg != "ok" {
		ctx.L.Println("rank err: ", repData.ErrMsg)
		return randomCopy(raws, ctx)
	}

	if repData.Tot <= 0 {
		return nil
	}

	nTot := repData.Tot
	rcRaws := make([]*raw_ad.RawAdObj, 0, repData.Tot)

	for _, rcData := range repData.Data {
		ctx.Estimate(rcData.Channel + "_" + rcData.Id + " ecpm: " + fmt.Sprintf("%.6f", rcData.Ecpm))
		for i := 0; i != len(raws); i++ {
			if nTot <= 0 {
				return rcRaws
			}

			if raws[i].Id == rcData.Id && raws[i].Channel == rcData.Channel {
				nTot--

				rawObj := *(raws[i])

				rawObj.AttachArgs = append(rawObj.AttachArgs, fmt.Sprintf("e=%f", 1000*rcData.Ecpm)) // anli没有乘1000，我帮你乘！
				rawObj.AttachArgs = append(rawObj.AttachArgs, fmt.Sprintf("po=%f", rawObj.Payout))

				if len(repData.Method) > 0 {
					ctx.Method = repData.Method
				} else {
					ctx.Method = rcData.Method
				}
				rcRaws = append(rcRaws, &rawObj)
				break
			}
		}
	}

	return rcRaws
}
