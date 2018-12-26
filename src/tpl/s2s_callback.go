package tpl

import (
	"net/http"
	"net/url"
	"strings"

	"ssp"
	"util"
)

var gS2sHeap *heapWrapper

func init() {
	gS2sHeap = NewHeapWrapper()
	go gS2sHeap.Serve()
}

func (s *Service) S2sCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusNoContent)

	if err := r.ParseForm(); err != nil {
		s.l.Println("[S2S] parse form err: ", err, ", uri: ", r.RequestURI)
		return
	}

	slotId := r.Form.Get("slot")
	idfa := r.Form.Get("idfa")
	gaid := r.Form.Get("gaid")
	aid := r.Form.Get("aid")
	custId := r.Form.Get("cust") // custom id, base64 encoded
	if len(custId) > 0 {
		b64, err := util.Base64Decode([]byte(custId))
		if err == nil {
			custId = string(b64)
		} else {
			s.l.Println("decode custom id err: ", err, ", uri: ", r.RequestURI)
		}
	}

	imp := r.Form.Get("imp")
	if len(imp) == 0 {
		s.l.Println("[S2S] get s2s callback missing imp: ", r.RequestURI)
		imp = util.UUIDStr(slotId)
	}

	slotStore := ssp.GetGlobalSlotStore()
	if slotStore == nil {
		s.l.Println("[S2S] no data", ", uri: ", r.RequestURI)
		return
	}

	slot := slotStore.Get(slotId)
	if slot == nil {
		s.l.Println("[S2S] no slot: ", slotId, ", uri: ", r.RequestURI)
		return
	}

	if slot.SlotSwitch != 1 {
		s.l.Println("[S2S] slot ", slotId, " closed, uri: ", r.RequestURI)
		return
	}

	cbUrl := slot.RewardedCallback
	if cbUrl == "" {
		return
	}

	cbUrl = strings.Replace(cbUrl, "{gaid}", gaid, -1)
	cbUrl = strings.Replace(cbUrl, "{aid}", aid, -1)
	cbUrl = strings.Replace(cbUrl, "{idfa}", idfa, -1)
	cbUrl = strings.Replace(cbUrl, "{slot}", slotId, -1)
	cbUrl = strings.Replace(cbUrl, "{custom_id}", custId, -1)
	cbUrl = strings.Replace(cbUrl, "{currency}",
		url.QueryEscape(slot.RewardedCurrency), -1)
	cbUrl = strings.Replace(cbUrl, "{amount}",
		url.QueryEscape(slot.RewardedAmount), -1)

	gS2sHeap.Push(NewCbItem(cbUrl, imp, slot.RewardedCbKey, s.l))

	return
}
