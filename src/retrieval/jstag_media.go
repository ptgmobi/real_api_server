package retrieval

import (
	"encoding/json"
	"io"
	"net/http"

	dnf "github.com/brg-liuwei/godnf"

	"http_context"
	"raw_ad"
	"util"
)

type jsMediaResp struct {
	ErrNo  int           `json:"err_no"`
	ErrMsg string        `json:"err_msg"`
	Cookie string        `json:"ck"`
	AdList []interface{} `json:"ad_list"`
}

func NewJmResp(errMsg string, errNo int, ctx *http_context.Context) *jsMediaResp {
	return &jsMediaResp{
		ErrNo:  errNo,
		ErrMsg: errMsg,
		Cookie: ctx.Ck,
		AdList: make([]interface{}, 0, 5),
	}
}

func (resp *jsMediaResp) WriteTo(w io.Writer) (int, error) {
	enc := json.NewEncoder(w)
	enc.Encode(resp)
	return 0, nil
}

func (s *Service) jstagMediaHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		NewJmResp("init ad error", 3, nil).WriteTo(w)
		return
	}

	tpl := s.getTpl(ctx.SlotId)
	if tpl == nil || tpl.SlotSwitch == 2 {
		NewJmResp("tpl empty", 5, nil).WriteTo(w)
		return
	}

	handler := dnf.GetHandler()
	if handler == nil {
		NewJmResp("dnf_handler nil", 6, ctx).WriteTo(w)
		return
	}

	conds := s.makeRetrievalConditions(ctx)
	docs, _ := handler.Search(conds, func(a dnf.DocAttr) bool {
		raw := a.(*raw_ad.RawAdObj)
		// 印尼电商弹窗单子都放到n3ym渠道
		return raw.Channel == "n3ym" && raw.Creatives != nil && len(raw.Creatives["ALL"]) > 0
	})

	util.Shuffle(docs)
	if len(docs) == 0 {
		NewJmResp("No ads", 1, ctx).WriteTo(w)
		return
	}

	adNum := len(docs)
	if adNum > 5 {
		adNum = 5
	}

	resp := NewJmResp("ok", 0, ctx)
	for _, docid := range docs[:adNum] {
		rawAdInter, _ := handler.DocId2Attr(docid)
		rawObj := *rawAdInter.(*raw_ad.RawAdObj) // val copy
		raw := &rawObj
		if adv := raw.ToJsMediaAd(ctx); adv != nil {
			resp.AdList = append(resp.AdList, adv)
		}
	}

	resp.WriteTo(w)
}
