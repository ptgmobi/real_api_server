package retrieval

import (
	"encoding/json"
	"net/http"
	"strings"

	"aes"
)

type FunnyResp struct {
	ErrNo int         `json:"err_no"`
	Funny FunnyResult `json:"result"`
}

type FunnyResult struct {
	Gaid string `json:"bast"`
	Aid  string `json:"hodor"`
}

func revolve(str string) string {
	if len(str) > 1 {
		return str[len(str)-1:] + str[1:len(str)-1] + str[0:1]
	}
	return str
}

func mix(id string) string {
	if len(id) == 0 {
		return id
	}
	// gaid b67514b2-20b7-4f5e-84a1-4b5fb9224a44
	// aid b17c49f8886a9e0
	var res []string
	if strings.Contains(id, "-") { // gaid
		for _, str := range strings.Split(id, "-") {
			res = append(res, revolve(str))
		}
	} else { // aid
		res = append(res, revolve(id))
	}
	return strings.Join(res, "-")
}

func (s *Service) funnyHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	gaid := r.Form.Get("gaid")
	aid := r.Form.Get("aid")
	apiVersion := r.Form.Get("api_version")

	var result = FunnyResult{
		Gaid: mix(gaid),
		Aid:  mix(aid),
	}
	var resp = FunnyResp{
		ErrNo: 1,
		Funny: result,
	}

	b, _ := json.Marshal(resp)
	if apiVersion != "v1" {
		b = aes.EncryptBytes(b)
	}

	w.Write(b)
	return
}
