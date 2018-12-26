package video

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"http_context"
	"raw_ad"
)

var (
	rankApi string
)

func Init(api string) {
	rankApi = api
}

type Offer struct {
	Id        string   `json:"id"`
	Price     float32  `json:"price"`
	Freq      int      `json:"freq"`
	PkgName   string   `json:"pkg"`
	Creatives []string `json:"creatives"`
	Channel   string   `json:"channel"`
	ChType    int      `json:"c_type"`
}

type PostReq struct {
	Slot      string         `json:"slot"`
	Country   string         `json:"country"`
	UserId    string         `json:"user_id"`
	Platform  string         `json:"platform"`
	Cached    []string       `json:"cached"`
	Offers    []*Offer       `json:"offers"`
	Creatives map[string]int `json:"creatives"`
	ReNum     int            `json:"re_num,omitempty"`
	CapNum    int            `json:"cap_num,omitempty"`
}

type Creative struct {
	Id  string `json:"id"`
	Num string `json:"num"`
}

type PostResp struct {
	Tot    int    `json:"tot"`
	ErrMsg string `json:"errmsg"`
	Method string `json:"method"`
	Data   []struct {
		CreativeId string  `json:"creative_id"`
		UniqId     string  `json:"uniq_id"`
		Ecpm       float32 `json:"ecpm"`
	} `json:"data"`
}

type CreativeResp struct {
	Tot    int      `json:"tot"`
	ErrMsg string   `json:"errmsg"`
	Method string   `json:"method"`
	Data   []string `json:"data"`
}

func createPostData(raws []*raw_ad.RawAdObj, ctx *http_context.Context) *PostReq {
	nRaws := len(raws)
	pData := &PostReq{
		Slot:      ctx.SlotId,
		Country:   ctx.Country,
		UserId:    ctx.UserId,
		Platform:  ctx.Platform,
		Cached:    make([]string, 0, 8),
		Offers:    make([]*Offer, 0, 8),
		Creatives: make(map[string]int, 8),
		CapNum:    ctx.VideoCacheNum,
		ReNum:     1,
	}

	if len(ctx.CidMap) == 0 {
		pData.ReNum = 2
	}

	for cid, num := range ctx.CidMap {
		if strings.HasPrefix(cid, "mp4") {
			// 排除掉头条API的素材(第三方实时api给的素材，不参与排序)
			if strings.HasPrefix(cid, "mp4.tt") {
				continue
			}
			pData.Cached = append(pData.Cached, cid)
			pData.Creatives[cid] = num
		}
	}

	if ctx.ServerCidMap != nil {
		for cid, num := range ctx.ServerCidMap {
			if strings.HasPrefix(cid, "mp4") {
				// 如果服务端计数存在存在，用服务端的计数覆盖客户端计数
				pData.Creatives[cid] = num
			}
		}
	}

	for i := 0; i != nRaws; i++ {
		pData.Offers = append(pData.Offers, &Offer{
			Id:        raws[i].Id,
			Price:     raws[i].Payout,
			PkgName:   raws[i].AppDownload.PkgName,
			Channel:   raws[i].Channel,
			Creatives: raws[i].VideoIds(ctx),
		})
	}

	return pData
}

func randomCopy(raws []*raw_ad.RawAdObj, ctx *http_context.Context) []*raw_ad.RawAdObj {
	var plist []*raw_ad.RawAdObj

	ctx.Method = "randcp"

	if ctx.AdNum >= len(raws) {
		plist = raws
	} else {
		l := len(raws)
		for i := 0; i != ctx.AdNum; i++ {
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

func randomCreative(raws []*raw_ad.RawAdObj, ctx *http_context.Context) []string {
	raws = randomCopy(raws, ctx)
	ids := make([]string, 0, len(raws))
	for _, raw := range raws {
		videoIds := raw.VideoIds(ctx)
		if l := len(videoIds); l > 0 {
			ids = append(ids, videoIds[rand.Intn(l)])
		}
	}
	return ids
}

func SelectCreative(raws []*raw_ad.RawAdObj, ctx *http_context.Context) []string {
	pData := createPostData(raws, ctx)
	reqb, err := json.Marshal(pData)
	if err != nil {
		return randomCreative(raws, ctx)
	}

	var buf bytes.Buffer
	if ctx.RankUseGzip {
		enc := gzip.NewWriter(&buf)
		enc.Write(reqb)
		enc.Close()
	} else {
		buf.Write(reqb)
	}

	reqUrl := rankApi + "/get_creatives?user_hash=" + strconv.Itoa(ctx.UserHash)

	req, err := http.NewRequest("POST", reqUrl, &buf)
	if err != nil {
		ctx.L.Println("Video Creatives Rank request err: ", err)
		return randomCreative(raws, ctx)
	}

	req.Header.Set("Content-Type", "application/json")
	if ctx.RankUseGzip {
		req.Header.Set("Content-Encoding", "gzip")
	}

	client := &http.Client{
		Timeout: 50 * time.Millisecond,
	}
	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		ctx.L.Println("Video Creatives Rank response err: ", err)
		return randomCreative(raws, ctx)
	}

	var respData CreativeResp
	if err = json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return randomCreative(raws, ctx)
	}

	if respData.ErrMsg != "ok" {
		return randomCreative(raws, ctx)
	}

	if respData.Tot <= 0 {
		return randomCreative(raws, ctx)
	}

	return respData.Data
}

func Select(raws []*raw_ad.RawAdObj, ctx *http_context.Context) []*raw_ad.RawAdObj {
	pData := createPostData(raws, ctx)
	reqb, err := json.Marshal(pData)
	if err != nil {
		ctx.ServerId = "rcp1"
		return randomCopy(raws, ctx)
	}

	var buf bytes.Buffer
	if ctx.RankUseGzip {
		enc := gzip.NewWriter(&buf)
		enc.Write(reqb)
		enc.Close()
	} else {
		buf.Write(reqb)
	}

	reqUrl := rankApi + "/rank?user_hash=" + strconv.Itoa(ctx.UserHash)

	req, err := http.NewRequest("POST", reqUrl, &buf)
	if err != nil {
		ctx.L.Println("Video Rank request err: ", err)
		ctx.ServerId = "rcp2"
		return randomCopy(raws, ctx)
	}

	req.Header.Set("Content-Type", "application/json")
	if ctx.RankUseGzip {
		req.Header.Set("Content-Encoding", "gzip")
	}

	client := &http.Client{
		Timeout: 50 * time.Millisecond,
	}
	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		ctx.L.Println("Video Rank response err: ", err)
		ctx.ServerId = "rcp3"
		return randomCopy(raws, ctx)
	}

	var respData PostResp
	if err = json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		ctx.ServerId = "rcp4"
		return randomCopy(raws, ctx)
	}

	if respData.ErrMsg != "ok" {
		ctx.ServerId = "rcp5"
		return randomCopy(raws, ctx)
	}

	if respData.Tot <= 0 {
		ctx.ServerId = "rcp6"
		return randomCopy(raws, ctx)
	}

	nTot := respData.Tot
	rcRaws := make([]*raw_ad.RawAdObj, 0, nTot)

	for _, rcData := range respData.Data {
		ctx.Estimate(rcData.UniqId + " ecpm: " + fmt.Sprintf("%.6f", rcData.Ecpm))
		for i := 0; i != len(raws); i++ {
			if nTot <= 0 {
				return rcRaws
			}

			if raws[i].UniqId == rcData.UniqId {
				nTot--

				// raw copy
				rawObj := *(raws[i])

				rawObj.AttachArgs = append(rawObj.AttachArgs, fmt.Sprintf("e=%f", 1000*rcData.Ecpm))
				rawObj.AttachArgs = append(rawObj.AttachArgs, fmt.Sprintf("po=%f", rawObj.Payout))

				// choose video
				if _, ok := ctx.CidMap[rcData.CreativeId]; ok {
					// 确保返回的素材ID是客户端带过来的，否则保持rawObj.VideoChosen为nil
					rawObj.ChoseVideoById(rcData.CreativeId)
				}

				if len(respData.Method) > 0 {
					ctx.Method = respData.Method
				}
				rcRaws = append(rcRaws, &rawObj)
				break
			}
		}
	}

	return rcRaws
}
