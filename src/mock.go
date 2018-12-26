package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	dnf "github.com/brg-liuwei/godnf"
	"github.com/brg-liuwei/gotools"

	"affiliate"
	"dump"
	yeahmobi_product "offer/yeahmobi"
	"raw_ad"
	"retrieval"
)

type Resp struct {
	Msg string `json:"error"`
}

func NewResp(msg string) *Resp {
	return &Resp{
		Msg: msg,
	}
}

func (resp *Resp) ToJson() (b []byte) {
	b, _ = json.Marshal(*resp)
	return
}

func httpHandlerHelper(location, filePrefix string) {
	var idx int64 = 0
	http.HandleFunc(location, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		if r.Method != "POST" {
			resp := NewResp("http method require POST")
			w.Write(resp.ToJson())
			return
		}
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			resp := NewResp("http header Content-Type error: " + ct)
			w.Write(resp.ToJson())
			return
		}

		atomic.AddInt64(&idx, 1)
		f, err := os.OpenFile("mock_data/"+filePrefix+"-"+strconv.Itoa(int(idx)),
			os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			resp := NewResp("create file error: " + err.Error())
			w.Write(resp.ToJson())
			return
		}
		defer f.Close()

		b, rerr := ioutil.ReadAll(r.Body)
		if rerr != nil {
			resp := NewResp("read body error: " + err.Error())
			w.Write(resp.ToJson())
			return
		}

		if _, werr := f.Write(b); werr != nil {
			resp := NewResp("write file error: " + err.Error())
			w.Write(resp.ToJson())
			return
		}

		resp := NewResp("ok")
		w.Write(resp.ToJson())
		return
	})
}

func mockError(a ...interface{}) {
	fmt.Fprintln(os.Stderr, a...)
}

func addRawAdsToDnf(h *dnf.Handler, raws []raw_ad.RawAdObj, docIdBase int) {
	for i := 0; i != len(raws); i++ {
		raw := &raws[i]
		docId := strconv.Itoa(docIdBase + i)
		if err := h.AddDoc("", docId, raw.Dnf, raw); err != nil {
			mockError("[DNF] Add Doc ", docId, "err: ", err, ", dnf: ", raw.Dnf)
		}
	}
}

func mockUpdateYeahmobi(file string) {
	if len(file) == 0 {
		mockError("please check your conf, mock yeahmobi data file[", file, "] empty")
		return
	}
	f, err := os.Open(file)
	if err != nil {
		mockError("open file ", file, " err: ", err)
		return
	}
	defer f.Close()

	dnfHandler := dnf.GetHandler()
	rawAdCnt := 0

	dec := json.NewDecoder(f)
	for {
		t, err := dec.Token()
		if err != nil {
			if err != io.EOF {
				mockError("dec Token error: ", err)
			}
			return
		}
		if _, ok := t.(json.Delim); ok {
			if dec.More() {
				continue
			}
			return
		}

		if key, ok := t.(string); ok {
			switch key {
			case "flag":
				t, err = dec.Token()
				if err != nil {
					if err != io.EOF {
						mockError("unexpected error when parsing flag: ", err)
					}
					return
				}
				v, _ := t.(string)
				if v != "success" {
					mockError("call yeahmobi api flag not success: ", v)
					return
				}

			case "msg":
				t, err = dec.Token()
				if err != nil {
					if err != io.EOF {
						mockError("unexpected error when parsing msg: ", err)
					}
					return
				}
				v, _ := t.(string)
				if v != "success." {
					mockError("call yeahmobi api msg not success: ", v)
					return
				}

			case "data":
				t, err = dec.Token() // read open bracket
				if err != nil {
					mockError("unexpected error when reading data open bracket: ", err)
					return
				}

				for dec.More() {
					var item yeahmobi_product.Product
					err = dec.Decode(&item)
					if err != nil {
						mockError("[JSON] unexpected error when reading data item: ", err)
						continue
					}

					rawAds := item.ToRawAdObjs("ym")
					addRawAdsToDnf(dnfHandler, rawAds, rawAdCnt)
					rawAdCnt += len(rawAds)
				}

				t, err = dec.Token() // read close bracket
				if err != nil {
					mockError("unexpected error when reading data open bracket: ", err)
					return
				}
			}
		}
	}
}

func mockUpdate(dataSrc map[string]string) {
	mockUpdateYeahmobi(dataSrc["yeahmobi"])
}

type MockConf struct {
	DataSrc       map[string]string `json:"data"`
	RetrievalConf retrieval.Conf    `json:"retrieval_config"`
	AffiliateConf affiliate.Conf    `json:"affiliate_config"`
	DumpAddr      string            `json:"dump_addr"`
}

func main() {
	var conf MockConf
	if err := gotools.DecodeJsonFile("conf/mock.conf.json", &conf); err != nil {
		panic(err)
	}

	h := dnf.NewHandler()
	dnf.SetHandler(h)

	go mockUpdate(conf.DataSrc)
	go dump.Serve(conf.DumpAddr)

	retrievalMock, err := retrieval.NewService(&conf.RetrievalConf)
	if err != nil {
		panic(err.Error())
	}
	go retrievalMock.Serve()

	affiliateMock := affiliate.NewAffiliate(&conf.AffiliateConf)
	go affiliateMock.Serve()

	httpHandlerHelper("/appinfo", "app")
	httpHandlerHelper("/slotinfo", "slot")
	panic(http.ListenAndServe(":22222", nil))
}
