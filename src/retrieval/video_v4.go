package retrieval

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"net/http"

	dnf "github.com/brg-liuwei/godnf"

	"aes"
	"http_context"
	"rank"
	rank_video "rank/video"
	"raw_ad"
	"util"
)

const Limit150M int64 = 250 * 1024 * 1024

type Creative struct {
	Url    string `json:"url"`
	CrId   string `json:"cid"`
	Type   string `json:"type"` // jpg or mp4
	Width  int    `json:"width"`
	Height int    `json:"height"`

	uniqId string
}

type CreativeResp struct {
	Error     string      `json:"error"`
	Creatives []*Creative `json:"creatives"`

	Country string `json:"country"`
	IsWifi  bool   `json:"is_wifi"`
	SlotId  string `json:"slot_id"`

	ctx *http_context.Context
}

func NewCreativeResp(errMsg string, ctx *http_context.Context) *CreativeResp {
	return &CreativeResp{
		Error:     errMsg,
		Creatives: make([]*Creative, 0),
		Country:   ctx.Country,
		SlotId:    ctx.SlotId,
		IsWifi:    ctx.IsWifi(),
		ctx:       ctx,
	}
}

func (resp *CreativeResp) WriteTo(w http.ResponseWriter) (int, error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Err-Msg", resp.Error)
	b, _ := json.Marshal(resp)

	if resp.ctx != nil && resp.ctx.UseGzipCT {
		var buf bytes.Buffer
		enc := gzip.NewWriter(&buf)
		if n, err := enc.Write(b); err != nil {
			enc.Close()
			return n, err
		}
		enc.Close()
		w.Header().Set("CT-Content-Encoding", "gzip")

		if resp.ctx.UseAes { // 加密
			b = aes.EncryptBytes(buf.Bytes())
			if resp.ctx.TestEncode != "hex" {
				w.Header().Set("CT-Encrypt", "binary")
				decodeBytes := make([]byte, hex.DecodedLen(len(b)))
				hex.Decode(decodeBytes, b)
				return w.Write(decodeBytes)
			} else {
				w.Header().Set("CT-Encrypt", "hex")
			}
		}
		return w.Write(b)
	}

	if resp.ctx != nil && resp.ctx.UseAes {
		b = aes.EncryptBytes(b)
	}

	if resp.ctx != nil && resp.ctx.UseGzip && len(b) > 1000 {
		enc := gzip.NewWriter(w)
		defer enc.Close()
		w.Header().Set("Content-Encoding", "gzip")
		return enc.Write(b)
	}
	return w.Write(b)
}

func ShuffleCreatives(arr []Creative) {
	size := len(arr)
	if size == 0 {
		return
	}
	for i := size - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		arr[i], arr[j] = arr[j], arr[i]
	}
}

type videoResp struct {
	ErrMsg string        `json:"err_msg"`
	ErrNo  int           `json:"err_no"`
	AdList []interface{} `json:"ad_list"`

	Headers map[string]string `json:"-"`

	offers []string
	ctx    *http_context.Context
}

func NewVideoResp(errMsg string, errNo int, ctx *http_context.Context) *videoResp {
	return &videoResp{
		ErrMsg:  errMsg,
		ErrNo:   errNo,
		AdList:  make([]interface{}, 0),
		Headers: make(map[string]string),
		ctx:     ctx,
		offers:  make([]string, 0),
	}
}

func (s *Service) videoCreativeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		s.l.Println("video creative handler new context error: ", err)
		http.Error(w, `{"err_msg": "url params error", "err_no":42}`, http.StatusOK)
		return
	}

	handler := dnf.GetHandler()
	if handler == nil {
		if n, err := NewCreativeResp("dnf handler nil", ctx).WriteTo(w); err != nil {
			s.l.Println("[videov4] dnf nil rewp write: ", n, ", error: ", err)
		}
		return
	}

	tpl := s.getVideoTplAndUpdateSlot(ctx)
	if tpl == nil {
		if n, err := NewCreativeResp("no active video slot matched", ctx).WriteTo(w); err != nil {
			s.l.Println("[videov4] no active video resp write: ", n, ", error: ", err)
		}
		return
	}

	if rand.Float64() > tpl.ImpressionRate {
		if n, err := NewCreativeResp("no match ad creative", ctx).WriteTo(w); err != nil {
			s.l.Println("[videov4] slot improssion rate control resp write: ", n, ", error:", err)
		}
		return
	}

	if err := ctx.GetFreq(); err != nil {
		s.l.Println("[videov4] GetFreq error: ", err)
	}

	if ctx.IsRewardedVideo() {
		if !ctx.VideoInRequestCap() || !ctx.VideoInCompleteCap(tpl.VideoFreqCap) || !tpl.SlotImpInCap(tpl.MaxComplete) {
			if n, err := NewCreativeResp("user reach freq cap of slot or slot reach complete cap", ctx).WriteTo(w); err != nil {
				s.l.Println("[videov4] user reach freq cap of slot or slot reach complete cap resp write: ", n, ", error:", err)
			}
			return
		}
	} else {
		if !tpl.SlotImpInCap(tpl.MaxImpression) {
			if n, err := NewCreativeResp("slot reach impression cap", ctx).WriteTo(w); err != nil {
				s.l.Println("[videov4] slot reach impression cap resp write: ", n, ", error:", err)
			}
			return
		}
	}

	// Video Screen Type 1: 横屏 2: 竖屏
	if tpl.VideoScreenType == 1 {
		ctx.VideoScreenType = 1
	} else if tpl.VideoScreenType == 2 {
		ctx.VideoScreenType = 2
	} else {
		ctx.VideoScreenType = 0
	}

	ctx.AdNum, ctx.VideoCacheNum = tpl.AdCacheNum, tpl.AdCacheNum
	ctx.LoadTime, ctx.ClickTime = tpl.LoadTime, tpl.ClickTime
	ctx.AdType = "7"
	ctx.CreativeType = "video"
	ctx.PkgName = ctx.PkgName + "(vdo" + ctx.AdType + ")"

	conds := s.makeRetrievalConditions(ctx)

	rawList := make([]*raw_ad.RawAdObj, 0, 16)
	selCreatives := make(map[string][]*Creative, 16) // key: videoId, val: [videoCreative, imgCreative]

	handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)

		if raw.IsT {
			return false
		}

		// 通用广告搜索
		if _, ok := s.adSearch(raw, ctx, tpl); !ok {
			return false
		}

		if tpl.AppCategoryInBlackList(raw.AppCategory) {
			return false
		}

		// 过滤iOS非wifi环境下超过150M
		if ctx.Platform == "iOS" && !ctx.IsWifi() && raw.AppDownload.FileSize >= Limit150M {
			return false
		}

		if ctx.AdType == "7" { // 激励视频只出特定渠道
			if !raw.IsHitChannelType(raw_ad.CT_REWARDED_VIDEO) {
				return false
			}
		}

		if ctx.IsIosInstalledPkg(raw.AppDownload.PkgName) {
			return false
		}

		if len(raw.Creatives) == 0 {
			return false
		}

		images := raw.GetImgs(ctx)
		if len(images) == 0 {
			return false
		}

		if len(raw.Videos) == 0 {
			return false
		}

		videos := raw.GetVideos(ctx)
		if len(videos) == 0 {
			return false
		}

		var selImage *Creative
		if img := raw.VideoGetMacthedImg(ctx); img != nil {
			selImage = &Creative{
				Url:    ctx.CreativeCdnConv(img.Url, img.DomesticCDN),
				CrId:   img.Id,
				Type:   "jpg",
				Width:  img.Width,
				Height: img.Height,
				uniqId: raw.UniqId,
			}
		}

		if selImage == nil {
			return false
		}

		for _, video := range videos {
			if len(video.Id) <= 0 {
				continue
			}
			if c, ok := selCreatives[video.Id]; !ok {
				selCreatives[video.Id] = make([]*Creative, 0, 16)
			} else if len(c) > 0 {
				continue
			}
			selCreatives[video.Id] = append(selCreatives[video.Id], &Creative{
				CrId:   video.Id,
				Url:    ctx.CreativeCdnConv(video.Url, video.DomesticCDN),
				Type:   "mp4",
				Width:  video.W,
				Height: video.H,
				uniqId: raw.UniqId,
			})
			if selImage != nil {
				selCreatives[video.Id] = append(selCreatives[video.Id], selImage)
			}
		}
		rawList = append(rawList, raw)

		return true
	})

	ctx.RankUseGzip = len(rawList) > rankUseGzipCap

	// 视频素材排序返回素材id
	ids := rank_video.SelectCreative(rawList, ctx)

	if len(ids) == 0 {
		if n, err := NewCreativeResp("no rank creatives", ctx).WriteTo(w); err != nil {
			s.l.Println("[videov4] no rank creatives resp write: ", n, ", error: ", err)
		}
		return
	}

	resp := NewCreativeResp("ok", ctx)

	for _, id := range ids {
		if sel, ok := selCreatives[id]; ok {
			resp.Creatives = append(resp.Creatives, sel...)
		}
	}

	if len(resp.Creatives) > 0 {
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[videov4] resp write: ", n, ", error: ", err)
		}
		return
	}

	if n, err := NewCreativeResp("no creatives suggestted", ctx).WriteTo(w); err != nil {
		s.l.Println("[videov4] no creatives resp write: ", n, ", error: ", err)
	}

	return
}

func (s *Service) videoAdHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		s.l.Println("videoAdHandler new context err: ", err)
		if n, err := NewRtvResp("url params error", 42, ctx).WriteTo(w); err != nil {
			s.l.Println("[videov4] context err resp write: ", n, ", error: ", err)
		}
		return
	}

	if ctx.Country == "CN" {
		ctx.ButtonText = "立即下载"
	} else {
		ctx.ButtonText = "Install Now"
	}

	handler := dnf.GetHandler()
	if handler == nil {
		if n, err := NewRtvResp("dnf handler nil", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[videov4] dnf nil resp write: ", n, ", error: ", err)
		}
		return
	}

	tpl := s.getVideoTplAndUpdateSlot(ctx)
	if tpl == nil {
		if n, err := NewRtvResp("no active video slot matched", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[videov4] no match tpl resp write: ", n, ", error: ", err)
		}
		return
	}

	if rand.Float64() > tpl.ImpressionRate {
		if n, err := NewRtvResp("no match ad", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[videov4] slot improssion rate control resp write: ", n, ", error:", err)
		}
		return
	}

	if err := ctx.GetFreq(); err != nil {
		s.l.Println("[videov4] GetFreq error: ", err)
	}

	if ctx.IsRewardedVideo() {
		if !ctx.VideoInRequestCap() || !ctx.VideoInCompleteCap(tpl.VideoFreqCap) || !tpl.SlotImpInCap(tpl.MaxComplete) {
			if n, err := NewRtvResp("user reach freq cap of slot or slot reach complete cap", 1, ctx).WriteTo(w); err != nil {
				s.l.Println("[videov4] user reach freq cap of slot or slot reach complete cap resp write: ", n, ", error:", err)
			}
			return
		}
	} else {
		if !tpl.SlotImpInCap(tpl.MaxImpression) {
			if n, err := NewRtvResp("slot reach impression cap", 1, ctx).WriteTo(w); err != nil {
				s.l.Println("[videov4] slot reach impression cap resp write: ", n, ", error:", err)
			}
			return
		}
	}

	s.SetVideoCtxTks(ctx)

	// Video Screen Type 1: 横屏 2: 竖屏
	if tpl.VideoScreenType == 1 {
		ctx.VideoScreenType = 1
	} else if tpl.VideoScreenType == 2 {
		ctx.VideoScreenType = 2
	} else {
		ctx.VideoScreenType = 0
	}

	// SlotInfo.Format: 6 video; 7 rewarded video
	if tpl.Format == 6 {
		ctx.AdType = "6"
	} else if tpl.Format == 7 {
		ctx.AdType = "7"
	} else {
		// use default adtype(7)
		s.l.Println("unexpected video format: ", tpl.Format, ", slot id: ", ctx.SlotId)
	}

	ctx.PkgName = ctx.PkgName + "(vdo" + ctx.AdType + ")"

	if ctx.ImgH*ctx.ImgW == 0 {
		if n, err := NewRtvResp("missing required parameter imgw or imgh",
			6, ctx).WriteTo(w); err != nil {
			s.l.Println("[videov4] missing imgw or imgh resp write: ", n, ", error:", err)
		}
		return
	}

	conds := s.makeRetrievalConditions(ctx)

	docs, _ := handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)

		// 通用广告搜索
		if msg, ok := s.adSearch(raw, ctx, tpl); !ok {
			ctx.Debug(raw.UniqId, msg)
			return false
		}

		if tpl.AppCategoryInBlackList(raw.AppCategory) {
			ctx.Debug(raw.UniqId, "slot black cat")
			return false
		}

		if ctx.AdType == "7" { // 激励视频只出特定渠道
			if !raw.IsHitChannelType(raw_ad.CT_REWARDED_VIDEO) {
				ctx.Debug(raw.UniqId, "channel type")
				return false
			}
		}

		if ctx.IsIosInstalledPkg(raw.AppDownload.PkgName) {
			ctx.Debug(raw.UniqId, "installed")
			return false
		}

		if !raw.HasVideo(ctx) {
			ctx.Debug(raw.UniqId, "no matched video")
			return false
		}

		return true
	})

	util.Shuffle(docs)
	ndocs := len(docs)

	if ndocs == 0 {
		if n, err := NewRtvResp("No ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[videov4] no ads resp write: ", n, ", error: ", err)
		}
		return
	}

	docCnt := 0

	listCap := ndocs
	if listCap > rankLimitCap {
		listCap = rankLimitCap
	}
	if listCap > rankUseGzipCap {
		ctx.RankUseGzip = true
	} else {
		ctx.RankUseGzip = false
	}
	rawList := make([]*raw_ad.RawAdObj, 0, listCap)

	for i := 0; i != ndocs && docCnt < listCap; i++ {
		docCnt++
		rawAdInter, _ := handler.DocId2Attr(docs[i])
		raw := rawAdInter.(*raw_ad.RawAdObj)

		rawList = append(rawList, raw)
	}

	raws := rank_video.Select(rawList, ctx)

	if len(raws) == 0 {
		if n, err := NewRtvResp("No rank ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[videov4] rank no ads resp write: ", n, ", error:", err)
		}
		return
	}

	resp := NewRtvResp("ok", 0, ctx)

	for _, raw := range raws {
		if adv := raw.ToVideoV4Ad(ctx); adv != nil {
			resp.AdList = append(resp.AdList, adv)
			resp.offers = append(resp.offers, raw.UniqId)
		}
	}

	if len(resp.AdList) > 0 {
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[videov4] resp write: ", n, ", error:", err)
		}
		return
	}

	if n, err := NewRtvResp("no video ad suggestted", 6, ctx).WriteTo(w); err != nil {
		s.l.Println("[videov4] no video ad resp write: ", n, ", error:", err)
	}
	return
}

// v4视频原生广告
func (s *Service) videov4NativeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		s.l.Println("native video new context error: ", err)
		if n, err := NewRtvResp("url params error", 42, ctx).WriteTo(w); err != nil {
			s.l.Println("[native-videov4] context err resp write: ", n, ", error: ", err)
		}
		return
	}

	handler := dnf.GetHandler()
	if handler == nil {
		if n, err := NewRtvResp("dnf handler nil", 2, ctx).WriteTo(w); err != nil {
			s.l.Println("[native-videov4] dnf nil rewp write: ", n, ", error: ", err)
		}
		return
	}

	tpl := s.getVideoTplAndUpdateSlot(ctx)
	if tpl == nil {
		if n, err := NewRtvResp("no active video slot matched", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[native-videov4] no match tpl resp write: ", n, ", error: ", err)
		}
		return
	}

	ctx.LoadTime, ctx.ClickTime = tpl.LoadTime, tpl.ClickTime

	if !tpl.MatchSspFormat(8) {
		if n, err := NewRtvResp("not native video slot", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[native-videov4] not native video slot resp write: ", n, ", error: ", err)
		}
		return
	}

	if rand.Float64() > tpl.ImpressionRate {
		if n, err := NewRtvResp("no match ad", 5, ctx).WriteTo(w); err != nil {
			s.l.Println("[native-videov4] slot improssion rate control resp write: ", n, ", error:", err)
		}
		return
	}

	if err := ctx.GetFreq(); err != nil {
		s.l.Println("[native-videov4] GetFreq error: ", err)
	}

	// 原生视频adtype 为6
	ctx.AdType = "6"
	// 全部横屏 XXX 益民需求
	ctx.VideoScreenType = 1
	ctx.PkgName = ctx.PkgName + "(vdo" + ctx.AdType + ")"

	if ctx.ImgH*ctx.ImgW == 0 {
		if n, err := NewRtvResp("missing required parameter imgw or imgh",
			6, ctx).WriteTo(w); err != nil {
			s.l.Println("[native-video] missing imgw or imgh resp write: ", n, ", error:", err)
		}
		return
	}

	conds := s.makeRetrievalConditions(ctx)
	docs, _ := handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)

		if msg, ok := s.adSearch(raw, ctx, tpl); !ok {
			ctx.Debug(raw.UniqId, msg)
			return false
		}

		if tpl.AppCategoryInBlackList(raw.AppCategory) {
			ctx.Debug(raw.UniqId, "slot black cat")
			return false
		}

		// 曝光频次
		if inCap, _ := ctx.InFreqCap(raw.AppDownload.PkgName, 5); !inCap {
			ctx.Debug(raw.UniqId, "imp frequency")
			return false
		}

		// 原生视频出ChannelType为3的offer
		if !raw.IsHitChannelType(raw_ad.CT_VIDEO) {
			ctx.Debug(raw.UniqId, "channel type")
			return false
		}

		if ctx.IsIosInstalledPkg(raw.AppDownload.PkgName) {
			ctx.Debug(raw.UniqId, "installed")
			return false
		}

		if !raw.HasMatchedNativeVideo(ctx) {
			ctx.Debug(raw.UniqId, "no matched native video")
			return false
		}

		return true
	})

	util.Shuffle(docs)
	ndocs := len(docs)

	if ndocs == 0 {
		if n, err := NewRtvResp("No ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[video] no ads resp write: ", n, ", error: ", err)
		}
		return
	}

	docCnt := 0
	listCap := ndocs
	if listCap > rankLimitCap {
		listCap = rankLimitCap
	}
	if listCap > rankUseGzipCap {
		ctx.RankUseGzip = true
	} else {
		ctx.RankUseGzip = false
	}
	rawList := make([]*raw_ad.RawAdObj, 0, listCap)

	for i := 0; i != ndocs && docCnt < listCap; i++ {
		docCnt++
		rawAdInter, _ := handler.DocId2Attr(docs[i])
		raw := rawAdInter.(*raw_ad.RawAdObj)

		rawList = append(rawList, raw)
	}

	raws := rank.Select(rawList, ctx)

	if len(raws) == 0 {
		if n, err := NewRtvResp("No ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[native-videov4] rank get no ads, resp write: ", n, ", error:", err)
		}
		return
	}

	s.SetVideoCtxTks(ctx)

	resp := NewRtvResp("ok", 0, ctx)
	for _, raw := range raws {
		if adv := raw.ToNativeVideoV4Ad(ctx); adv != nil {
			resp.AdList = append(resp.AdList, adv)
			resp.offers = append(resp.offers, raw.UniqId)
		}
	}

	if len(resp.AdList) > 0 {
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[native-videov4] resp write: ", n, ", error:", err)
		}
		return
	}

	if n, err := NewRtvResp("no video ad suggestted", 6, ctx).WriteTo(w); err != nil {
		s.l.Println("[native-videov4] no video ad resp write: ", n, ", error:", err)
	}
	return
}
