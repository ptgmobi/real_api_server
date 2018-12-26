package dump

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	dnf "github.com/brg-liuwei/godnf"

	"raw_ad"
)

func getOfferIds(b []byte) []byte {
	type DataItem struct {
		Attr struct {
			OfferId string `json:"adid"`
		} `json:"attr"`
		// other fields
	}
	type JsonObj struct {
		Data         []DataItem `json:"data"`
		TotalRecords int        `json:"total_records"`
	}

	var obj JsonObj
	if err := json.Unmarshal(b, &obj); err != nil {
		return []byte("[\"unmarshal json error: " + err.Error() + "\"]")
	}
	resp := make([]string, 0, 8)
	for _, d := range obj.Data {
		resp = append(resp, d.Attr.OfferId)
	}
	b, _ = json.Marshal(resp)
	return b
}

func Serve(addr string) {
	// dump func
	http.HandleFunc("/dump", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf8")
		h := dnf.GetHandler()
		if h == nil {
			http.Error(w, "{\"error\":\"dnf handler nil\"}", http.StatusOK)
			return
		}

		r.ParseForm()

		country := r.Form.Get("country")
		platform := r.Form.Get("platform")
		channel := r.Form.Get("channel")
		version := r.Form.Get("version")
		material := r.Form.Get("material")
		showTest := r.Form.Get("show_test") == "1"

		toMap := r.Form.Get("tomap")

		if len(version) == 0 {
			version = "any"
		}

		if len(material) == 0 {
			material = "any"
		}

		if len(country) != 0 || len(platform) != 0 {
			conds := []dnf.Cond{
				dnf.Cond{"country", country},
				dnf.Cond{"region", "DEBUG"},
				dnf.Cond{"city", "DEBUG"},
				dnf.Cond{"platform", platform},
				dnf.Cond{"version", version},
				dnf.Cond{"material", material},
				dnf.Cond{"device", "any"},
			}

			var docs []int
			if len(channel) == 0 {
				docs, _ = h.Search(conds, func(a dnf.DocAttr) bool {
					if !showTest {
						raw := a.(*raw_ad.RawAdObj)
						return !raw.IsT
					}
                    return true
				})
			} else {
				docs, _ = h.Search(conds, func(a dnf.DocAttr) bool {
					raw := a.(*raw_ad.RawAdObj)
					if !showTest && raw.IsT {
						return false
					}
					if raw.Channel != channel {
						return false
					}
					return true
				})
			}

			list := make([]interface{}, 0, len(docs))
			for _, doc := range docs {
				if toMap == "1" {
					raw := h.DocId2Map(doc)
					list = append(list, raw)
				} else {
					raw, _ := h.DocId2Attr(doc)
					list = append(list, raw)
				}
			}
			b, _ := json.Marshal(map[string]interface{}{
				"data":          list,
				"total_records": len(docs),
			})
			w.Write(b)
			return
		}

		offers := r.Form.Get("offers")
		if len(offers) != 0 {
			offerList := strings.Split(offers, ",")
			w.Write(h.DumpByFilter(func(attr dnf.DocAttr) bool {
				raw := attr.(*raw_ad.RawAdObj)
				for _, o := range offerList {
					if o == raw.Id {
						return true
					}
				}
				return false
			}))
			return
		}

		pageNum, _ := strconv.Atoi(r.Form.Get("page_num"))
		pageSize, _ := strconv.Atoi(r.Form.Get("page_size"))
		if pageSize == 0 {
			pageSize = 10 // default
		}

		b := h.DumpByPage(pageNum, pageSize, func(attr dnf.DocAttr) bool {
			if !showTest {
				raw := attr.(*raw_ad.RawAdObj)
				return !raw.IsT
			}
			return true
		})
		if r.Form.Get("show_offer") == "true" {
			b = getOfferIds(b)
		}
		w.Write(b)
	})

	http.HandleFunc("/dump_creative", func(w http.ResponseWriter, r *http.Request) {
		// TODO: dump creative
		http.Error(w, "", http.StatusNoContent)
	})
	panic(http.ListenAndServe(addr, nil))
}
