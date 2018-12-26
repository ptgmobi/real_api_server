package affiliate

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	dnf "github.com/brg-liuwei/godnf"

	"raw_ad"
	"util"
)

var countryReg *regexp.Regexp
var platformReg *regexp.Regexp
var osvReg *regexp.Regexp

func init() {
	countryReg = regexp.MustCompile("country in \\{\\s*([A-Z,]+)\\s*\\}")
	platformReg = regexp.MustCompile("platform in \\{\\s*(Android|iOS)\\s*\\}")
	osvReg = regexp.MustCompile("osv in \\{\\s*([\\d\\.,]+)\\s*\\}")
}

type Token struct {
	AffToken        string   `json:"aff_token"`
	Slot            string   `json:"slot"`
	Discount        float32  `json:"discount"`
	AllowedChannels []string `json:"allowed_channels"`

	allowedChan map[string]bool
}

type Conf struct {
	ListenAddr string  `json:"listen_addr"`
	Tokens     []Token `json:"tokens"`
}

type target struct {
	Country  []string `json:"country,omitempty"`
	Platform []string `json:"platform,omitempty"`
	Osv      []string `json:"osv,omitempty"`
}

func (t *target) parseTargets(tReg *regexp.Regexp, dnf string) []string {
	slice := tReg.FindStringSubmatch(dnf)
	if len(slice) == 2 {
		return strings.Split(slice[1], ",")
	}
	return nil
}

func (t *target) parse(dnf string) {
	countries := t.parseTargets(countryReg, dnf)
	// filter DEBUG
	for i := 0; i != len(countries); i++ {
		if countries[i] == "DEBUG" {
			countries = append(countries[:i], countries[i+1:]...)
			break
		}
	}
	t.Country = countries
	t.Platform = t.parseTargets(platformReg, dnf)
	t.Osv = t.parseTargets(osvReg, dnf)
}

func (t *target) toBytes() []byte {
	b, _ := json.Marshal(t)
	return b
}

type image struct {
	Height int    `json:"height"`
	Width  int    `json:"width"`
	Url    string `json:"url"`
}

type Offer struct {
	Id           string   `json:"id"`
	Category     []string `json:"category"`
	PkgName      string   `json:"package_name"`
	Desc         string   `json:"desc"`
	Title        string   `json:"title"`
	Star         float32  `json:"star"`
	TrackingLink string   `json:"tracking_link"`
	Payout       float32  `json:"payout"`
	Creatives    []image  `json:"creatives"`
	Icons        []image  `json:"icons"`
	Targets      target   `json:"targets"`
}

func NewOffer() *Offer {
	return &Offer{
		Creatives: make([]image, 0, 4),
		Icons:     make([]image, 0, 4),
	}
}

func slotPayoutDiscountEncode(slot string, payout, discount float32) string {
	s := fmt.Sprintf("%d_%s_%.2f_%.2f", rand.Int()%100, slot, discount+payout, payout)
	return util.AlphaTableEncode(s)
}

func (offer *Offer) parseFromRaw(raw *raw_ad.RawAdObj, t *Token) bool {
	if !t.allowedChan[raw.Channel] {
		return false
	}

	switch raw.Channel {
	case "ym":
		offer.Id = "1" + raw.Id
	case "glp":
		offer.Id = "2" + raw.Id
	case "nglp": // glispa non-preload
		offer.Id = "3" + raw.Id
	default:
		fmt.Println("channel unsupport: ", raw.Channel)
		return false
	}

	offer.Category = raw.AppCategory
	offer.PkgName = raw.AppDownload.PkgName
	offer.Desc = raw.AppDownload.Desc
	offer.Title = raw.AppDownload.Title
	offer.Star = raw.AppDownload.Rate
	offer.Payout = raw.Payout * t.Discount
	offer.Payout = float32(int(offer.Payout*100)) / 100.0

	if offer.Payout <= 0.01 {
		fmt.Println("lower than 0.01 after discount: ", t.Discount, "|", raw.Payout, "|", raw.Id)
		return false
	}

	b, _ := util.Base64Encode([]byte(raw.AppDownload.TrackLink))

	alphaEncode := slotPayoutDiscountEncode(t.Slot, offer.Payout, t.Discount)
	offer.TrackingLink = "http://track.cloudmobi.net/trk/" + alphaEncode + "/" + offer.Id + "/" + string(b) + "/djj"

	for _, imgs := range raw.Creatives {
		for i := 0; i != len(imgs); i++ {
			offer.Creatives = append(offer.Creatives, image{
				Height: imgs[i].Height,
				Width:  imgs[i].Width,
				Url:    imgs[i].Url,
			})
		}
	}

	for _, icons := range raw.Icons {
		for i := 0; i != len(icons); i++ {
			offer.Icons = append(offer.Icons, image{
				Height: icons[i].Height,
				Width:  icons[i].Width,
				Url:    icons[i].Url,
			})
		}
	}

	offer.Targets.parse(raw.Dnf)

	return len(offer.Creatives) > 0 && len(offer.Icons) > 0 &&
		len(offer.Targets.Country) > 0 && len(offer.Targets.Platform) == 1
}

func (offer *Offer) toBytes() []byte {
	b, _ := json.Marshal(offer)
	return b
}

func (offer *Offer) WriteTo(w io.Writer) (int, error) {
	return w.Write(offer.toBytes())
}

type Affiliate struct {
	conf *Conf
}

func NewAffiliate(cf *Conf) *Affiliate {
	for i, t := range cf.Tokens {
		if t.Discount > 1.0 || t.Discount < 0.0 {
			panic("token[" + strconv.Itoa(i) + "] discount illegal: " + fmt.Sprintf("%.3f", t.Discount))
		}

		m := make(map[string]bool, len(t.AllowedChannels))
		for _, ch := range t.AllowedChannels {
			m[ch] = true
		}
		cf.Tokens[i].allowedChan = m
	}
	return &Affiliate{
		conf: cf,
	}
}

func (aff *Affiliate) writeStream(w http.ResponseWriter, t *Token, h *dnf.Handler) {
	if _, err := w.Write([]byte("{\"msg\":\"ok\",\"data\":[")); err != nil {
		fmt.Println("w write error:", err)
		return
	}

	isFirst := true

	for i := 0; ; i++ {
		attr, err := h.DocId2Attr(i)
		if err != nil {
			break
		}

		raw, _ := attr.(*raw_ad.RawAdObj)
		if raw == nil {
			break
		}

		if raw.Channel != "ym" && raw.Channel != "tym" {
			continue
		}

		if raw.ReplaceSlotId {
			continue
		}

		offer := NewOffer()
		if !offer.parseFromRaw(raw, t) {
			continue
		}

		if isFirst {
			isFirst = false
		} else {
			w.Write([]byte(","))
		}

		offer.WriteTo(w)
	}

	w.Write([]byte("]}"))
}

var examResp []byte = []byte(`{"msg":"ok","data":[{"id":"1361591","category":["communication"],"package_name":"com.UCMobile.intl","desc":"Online videos without waiting. Movies and TV shows in Speed mode. Fast and stable downloads,thanks to our powerful servers. Expanding AdBlock,adapted to main websites and blocks most ads.","title":"UC Browser - Fast Download","star":4.5,"payout":4.2,"tracking_link":"http://track.cloudmobi.net/trk/aHR0cDovL3d3dy5jbG91ZG1vYmkubmV0","creatives":[{"height":294,"width":564,"url":"http://ymcdn.ymtrack6.co/ads_file/4c64fd97-de7f-429a-83bf-59dcac689e477791362065719670797.jpg"},{"height":294,"width":564,"url":"http://ymcdn.ymtrack6.co/ads_file/7c6cc7e7-1196-4644-bf1f-c3863c6c1b644457940557044245750.jpg"},{"height":294,"width":564,"url":"http://ymcdn.ymtrack6.co/ads_file/841401f5-5cf4-45a0-9f4e-42fc95adb2f46959403756859660592.jpg"}],"icons":[{"height":300,"width":300,"url":"http://ymcdn.ymtrack6.co/offer_single_files/OgDb125aG76xs16oPtOcmkP8R9nyQVe3.png"}],"targets":{"country":["JP"],"platform":["Android"]}}]}`)

func (aff *Affiliate) example(w http.ResponseWriter) {
	w.Write(examResp)
}

func (aff *Affiliate) offersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	r.ParseForm()
	token := r.Form.Get("token")
	if token == "5c263df85fc084ba15b550ce3887fb6d" {
		aff.example(w)
		return
	}

	h := dnf.GetHandler()
	if h == nil {
		http.Error(w, "{\"msg\":\"please retry after 1 min\",\"data\":[]}", http.StatusOK)
		return
	}

	if r.Method != "GET" {
		util.HttpError(w, http.StatusMethodNotAllowed)
		return
	}

	auth := false
	i := 0
	for i = 0; i != len(aff.conf.Tokens); i++ {
		if token == aff.conf.Tokens[i].AffToken {
			auth = true
			break
		}
	}

	if !auth {
		util.HttpError(w, http.StatusUnauthorized)
		return
	}

	aff.writeStream(w, &aff.conf.Tokens[i], h)
}

func (aff *Affiliate) Serve() {
	http.HandleFunc("/get_offers", aff.offersHandler)
	panic(http.ListenAndServe(aff.conf.ListenAddr, nil))
}
