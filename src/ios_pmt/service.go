package ios_pmt

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/willf/bloom"

	"aes"
	"util"
)

var awsRegion, s3Bucket, s3Key string

type Conf struct {
	BfConf         BfConf `json:"bf_config"`
	BundleFileConf string `json:"bundle_file_conf"`
	RegionConf     string `json:"region_conf"`
	AwsRegion      string `json:"aws_region"`
	S3Bucket       string `json:"s3_bucket"`
	S3Key          string `json:"s3_key"`
}

type BfConf struct {
	BfN    uint    `json:"bloom_filter_n"`
	BfFp   float64 `json:"bloom_filter_fp"`
	BfPath string  `json:"bloom_filter_path"`
}

type Resp struct {
	Err         string            `json:"error"`
	Dict        map[string]string `json:"init_dict,omitempty"`
	WebViewDict map[string]string `json:"web_view_dict,omitempty"`
	BundleLists []string          `json:"bundle_list,omitempty"`

	body       []byte
	cipherBody []byte
}

var webViewDict map[string]string = map[string]string{
	// web view
	"ctInternal":     "_internal",
	"ctBrowser":      "browserView",
	"ctWebView":      "_webView",
	"ctBWebView":     "WebView",
	"ctSetGLEnabled": "_setWebGLEnabled:",
	"ctGLEnabled":    "_webGLEnabled",
}

var disableJsWorkerWebViewDict map[string]string = map[string]string{
	// disable worker (default disable worker)
	"ctJsContext":   "webView:didCreateJavaScriptContext:forFrame:",
	"ctParentFrame": "parentFrame",

	// web view
	"ctInternal":     "_internal",
	"ctBrowser":      "browserView",
	"ctWebView":      "_webView",
	"ctBWebView":     "WebView",
	"ctSetGLEnabled": "_setWebGLEnabled:",
	"ctGLEnabled":    "_webGLEnabled",
}

var musicSlots map[int]bool = map[int]bool{
	185:      true,
	189:      true,
	191:      true,
	192:      true,
	193:      true,
	194:      true,
	298:      true,
	299:      true,
	386:      true,
	387:      true,
	388:      true,
	389:      true,
	390:      true,
	391:      true,
	392:      true,
	393:      true,
	394:      true,
	395:      true,
	441:      true,
	442:      true,
	533:      true,
	534:      true,
	1123:     true,
	1124:     true,
	1125:     true,
	1126:     true,
	1127:     true,
	1128:     true,
	98784251: true,
	706:      true,
	707:      true,
	708:      true,
	709:      true,
	710:      true,
	711:      true,
	712:      true,
	713:      true,
	714:      true,
	715:      true,
	716:      true,
	717:      true,
	718:      true,
	719:      true,
	720:      true,
	721:      true,
	722:      true,
	723:      true,
	724:      true,
	725:      true,
	1129:     true,
	1130:     true,
	1131:     true,
	1132:     true,
	1133:     true,
	1134:     true,
	3351:     true,
	3352:     true,
	3353:     true,
	931:      true,
	953:      true,
	954:      true,
	955:      true,
	956:      true,
	957:      true,
	958:      true,
	959:      true,
	960:      true,
	961:      true,
	962:      true,
	963:      true,
	964:      true,
	965:      true,
	966:      true,
	967:      true,
	968:      true,
	969:      true,
	970:      true,
	1095:     true,
	1096:     true,
	1097:     true,
	1098:     true,
	1099:     true,
	1100:     true,
	1101:     true,
	1567:     true,
	1568:     true,
	1898:     true,
	1899:     true,
	1900:     true,
	1901:     true,
	1903:     true,
	1904:     true,
	1905:     true,
	1906:     true,
	1907:     true,
	1908:     true,
	1909:     true,
	1910:     true,
	1911:     true,
	1912:     true,
	1913:     true,
	1914:     true,
	94065373: true,
	1147:     true,
	1148:     true,
	1149:     true,
	1150:     true,
	1151:     true,
	1152:     true,
	1153:     true,
	1154:     true,
	1155:     true,
	1156:     true,
	1157:     true,
	1158:     true,
	1159:     true,
	1160:     true,
	1161:     true,
	1162:     true,
	1163:     true,
	1164:     true,
	1165:     true,
	1166:     true,
	1167:     true,
	1168:     true,
	1169:     true,
	1170:     true,
	1171:     true,
	1258:     true,
	1259:     true,
	1263:     true,
	1264:     true,
	1265:     true,
	1266:     true,
	1267:     true,
	1268:     true,
	1269:     true,
	1270:     true,
	1271:     true,
	1272:     true,
	1273:     true,
	1274:     true,
	1275:     true,
	1276:     true,
	1277:     true,
	1278:     true,
	1279:     true,
	1280:     true,
	1281:     true,
	1282:     true,
	1283:     true,
	1284:     true,
	1285:     true,
	1312:     true,
	1313:     true,
	1323:     true,
	1324:     true,
	1325:     true,
	1326:     true,
	1327:     true,
	1328:     true,
	1329:     true,
	1330:     true,
	1405:     true,
	1406:     true,
	1407:     true,
	1408:     true,
	1409:     true,
	1410:     true,
	1411:     true,
	1412:     true,
	1413:     true,
	1414:     true,
	1415:     true,
	1416:     true,
	1417:     true,
	1418:     true,
	1419:     true,
	1420:     true,
	1421:     true,
	1422:     true,
	1423:     true,
	1424:     true,
	1425:     true,
	1426:     true,
	1427:     true,
	1428:     true,
	1429:     true,
	1562:     true,
	1563:     true,
	1709:     true,
	1710:     true,
	1849:     true,
	1850:     true,
	1852:     true,
	1853:     true,
	1854:     true,
	1855:     true,
	1856:     true,
	1857:     true,
	1858:     true,
	1859:     true,
	1860:     true,
	1861:     true,
	1862:     true,
	1863:     true,
	1864:     true,
	1865:     true,
	1866:     true,
	1867:     true,
	1868:     true,
	1869:     true,
	1870:     true,
	1871:     true,
	1872:     true,
	1873:     true,
	1874:     true,
	2538:     true,
	2536:     true,
	2537:     true,
	2539:     true,
	2543:     true,
	2544:     true,
	2545:     true,
	2546:     true,
	2547:     true,
	2548:     true,
	2549:     true,
	2550:     true,
	2551:     true,
	2552:     true,
	2553:     true,
	2554:     true,
	2555:     true,
	2556:     true,
	2557:     true,
	2558:     true,
	2559:     true,
	2560:     true,
	2561:     true,
	2562:     true,
	2563:     true,
	2564:     true,
	2565:     true,
	92933979: true,
	10250697: true,
	25204392: true,
	3790:     true,
	3791:     true,
	3801:     true,
	3806:     true,
	3807:     true,
	3808:     true,
	3809:     true,
	3810:     true,
	3811:     true,
	3812:     true,
	3813:     true,
	3814:     true,
	3815:     true,
	3816:     true,
	3817:     true,
	3818:     true,
	3819:     true,
	3826:     true,
	3827:     true,
	3828:     true,
	3829:     true,
	3830:     true,
	3831:     true,
	3832:     true,
	3833:     true,
	3834:     true,
	61534045: true,
	49157732: true,
	70939868: true,
	16186641: true,
	3792:     true,
	3793:     true,
	3802:     true,
	3846:     true,
	3856:     true,
	13318567: true,
	17382730: true,
	79853455: true,
	84662337: true,
	55768890: true,
	89385858: true,
	37927499: true,
	91862393: true,
	69717526: true,
	46463070: true,
	69604347: true,
	17548302: true,
	68298448: true,
	94525770: true,
	55450029: true,
	10000049: true,
	30969842: true,
	28141455: true,
	61280775: true,
	53496549: true,
	36714501: true,
	73188959: true,
	78997949: true,
	56810599: true,
	57367896: true,
	63465196: true,
	24810271: true,
	21228279: true,
	86570754: true,
	27762457: true,
	85702933: true,
	70770824: true,
	19459405: true,
	65965347: true,
	36854681: true,
	99623035: true,
	25419126: true,
	71478055: true,
	26930178: true,
	21287029: true,
	34285316: true,
	61700796: true,
	69531183: true,
	30663467: true,
	21410807: true,
	89106468: true,
	40168054: true,
	31180976: true,
	53590118: true,
	28914722: true,
	21858137: true,
	98904986: true,
	49550516: true,
	29851667: true,
	19004863: true,
	39896386: true,
	80747884: true,
	46084164: true,
	75869159: true,
	87917740: true,
}

var tutuSlots map[int]bool = map[int]bool{
	1246: true,
	2997: true,
	2998: true,
	2999: true,
	3000: true,
	3001: true,
	3002: true,
	3003: true,
	3005: true,
	3006: true,
	3007: true,
	3008: true,
	3009: true,
	3010: true,
	3011: true,
	3012: true,
	3013: true,
	3014: true,
	3015: true,
	3016: true,
	3017: true,
	3018: true,
	3019: true,
	3020: true,
	3021: true,
	3022: true,
	3023: true,
	3024: true,
	3025: true,
	3026: true,
	3027: true,
	2592: true,
	2593: true,
	2594: true,
	2605: true,
	2606: true,
	2607: true,
	2608: true,
	2609: true,
	2610: true,
	2611: true,
	2612: true,
	2613: true,
	2614: true,
	2615: true,
	2616: true,
	2617: true,
	2618: true,
	2619: true,
	2620: true,
	2621: true,
	2622: true,
	1065: true,
	1066: true,
	1067: true,
	2381: true,
	2407: true,
	2706: true,
	1117: true,
	1244: true,
}

func NewResp(errMessage string) *Resp {
	return &Resp{
		Err:         errMessage,
		WebViewDict: webViewDict,
	}
}

func (resp *Resp) WriteTo(w http.ResponseWriter) (int, error) {
	return w.Write(resp.body)
}

func (resp *Resp) WriteCipherTo(w http.ResponseWriter) (int, error) {
	return w.Write(resp.cipherBody)
}

var unusedResp *Resp
var okResp *Resp

var gVerMap map[string]bool
var gVerMapLock sync.RWMutex

func copyGverMap() map[string]bool {
	gVerMapLock.RLock()
	defer gVerMapLock.RUnlock()
	m := make(map[string]bool, len(gVerMap))
	for k, v := range gVerMap {
		m[k] = v
	}
	return m
}

func verMapSet(key string, status bool) {
	gVerMapLock.Lock()
	defer gVerMapLock.Unlock()
	gVerMap[key] = status
}

func verMapGet(key string) bool {
	gVerMapLock.RLock()
	defer gVerMapLock.RUnlock()
	return gVerMap[key]
}

func verMapGetOr(keys ...string) bool {
	gVerMapLock.RLock()
	defer gVerMapLock.RUnlock()

	for _, key := range keys {
		if gVerMap[key] {
			return gVerMap[key]
		}
	}
	return false
}

func StoreVerMap(appVerInfo string, status int) {
	// appVerInfos "slot-sdkversion-appversion" or "slot-sdkversion-clkversion"
	if status == 1 { // 1是开启后截， 2是关闭后截
		verMapSet(appVerInfo, true)
	} else {
		verMapSet(appVerInfo, false)
	}
}

func loadBloomFilterFile(conf *BfConf) error {
	f, err := os.Open(conf.BfPath)
	if err != nil {
		return err
	}
	defer f.Close()

	bf = bloom.NewWithEstimates(conf.BfN, conf.BfFp)
	if err = gob.NewDecoder(f).Decode(bf); err != nil {
		return err
	}

	return nil
}

func osVersionEnableBundleList(osv string) bool {
	// osv=10.2 || osv=9.8.2
	if len(osv) == 0 {
		return false
	}
	osvs := strings.Split(osv, ".")
	osvInt, err := strconv.Atoi(osvs[0])
	if err != nil {
		log.Println("[osVersionEnable] osv params is err: ", err, " osv: ", osv)
		return false
	}
	if osvInt >= 11 {
		return true
	}
	return false
}

func getCountryWithIp(regionUri string) (string, error) {
	type Region struct {
		Meta struct {
			IP      string `json:"ip"`
			Country string `json:"country"`
			Carrier string `json:"-"`
			Region  string `json:"-"`
			City    string `json:"-"`
		} `json:"meta,omitempty"`
		ErrNo  int    `json:"err_no"`
		ErrMsg string `json:"err_msg"`
	}
	resp, err := http.Get(regionUri)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return "", fmt.Errorf("[getCountryWithIp] get region err: %v, uri: %s", err, regionUri)
	}
	var r Region
	body, err := ioutil.ReadAll(resp.Body)
	if err = json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("[getCountryWithIp] decode json err: %v, uri: %s", err, regionUri)
	}
	if len(r.Meta.Country) == 0 {
		return "", fmt.Errorf("[getCountryWithIp] get country err: %v", r.ErrMsg)
	}
	return r.Meta.Country, nil
}

var once sync.Once
var bf *bloom.BloomFilter

func Init(cf *Conf) {
	awsRegion = cf.AwsRegion
	s3Bucket = cf.S3Bucket
	s3Key = cf.S3Key

	if err := InitBundle(cf.BundleFileConf); err != nil {
		panic("InitBundle err: " + err.Error())
	}

	once.Do(func() {
		gVerMap = make(map[string]bool, 1024)

		err := loadBloomFilterFile(&cf.BfConf)
		if err != nil {
			panic("loadBloomFilterFile err: " + err.Error())
		}
	})

	go updateBundleWithLock(cf.BundleFileConf)
	http.HandleFunc("/get_ios_pmt/info", HandleReviewIosPmtInfo)

	unusedResp := NewResp("unused")
	unusedResp.body, _ = json.Marshal(unusedResp)
	unusedResp.cipherBody = aes.EncryptBytes(unusedResp.body)

	http.HandleFunc("/iconf/get", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		slotId := r.Form.Get("slot_id")
		appVer := r.Form.Get("av") // app version
		sdkVer := r.Form.Get("sv") // sdk version
		cliVer := r.Form.Get("cv") // client version
		osVer := r.Form.Get("osv") // os version
		ip := r.Form.Get("ip")

		bundleList := make([]string, 0, 30)

		if r.Header.Get("CT-Accept-Encoding") == "gzip" {
			w.Header().Set("CT-Encrypt", "hex")
		}

		noCipher := r.Form.Get("nocipher") == "1"
		key1 := util.StrJoinHyphen(slotId, sdkVer, appVer)
		key2 := util.StrJoinHyphen(slotId, sdkVer, cliVer)
		key3 := util.StrJoinHyphen(slotId, "*")
		if !(verMapGetOr(key1, key2, key3) && !bf.Test([]byte(ip))) {
			if noCipher {
				unusedResp.WriteTo(w)
				return
			} else {
				unusedResp.WriteCipherTo(w)
				return
			}
		}

		if osVer >= "12" && sdkVer < "3.2.4" {
			// sv < 3.2.4以下的代码，会在下发iOS12的代码中导致崩溃
			if noCipher {
				unusedResp.WriteTo(w)
				return
			} else {
				unusedResp.WriteCipherTo(w)
				return
			}
			return
		}

		// XXX: 临时修改for tcash
		if slotId == "b34df357" {
			tcashResp := &Resp{
				Err: "unused",
			}
			if noCipher {
				tcashResp.WriteTo(w)
				return
			} else {
				tcashResp.WriteCipherTo(w)
				return
			}
		}

		okResp := NewResp("ok")
		if slotNum, err := strconv.Atoi(slotId); err == nil {
			if slotNum == 518 {
				// disable js worker for 小影
				okResp.WebViewDict = disableJsWorkerWebViewDict
			} else if slotNum >= 2147 && slotNum <= 2156 {
				// disable js worker for 漫画人
				okResp.WebViewDict = disableJsWorkerWebViewDict
			} else if slotNum >= 3276 && slotNum <= 3277 {
				// disable js worker for 车轮查违章
				okResp.WebViewDict = disableJsWorkerWebViewDict
			} else if slotNum == 54687570 || slotNum == 45936622 || slotNum == 45345985 {
				// disable js worker for 无他相机
				okResp.WebViewDict = disableJsWorkerWebViewDict
			} else if tutuSlots[slotNum] {
				okResp.WebViewDict = disableJsWorkerWebViewDict
			} else if musicSlots[slotNum] {
				okResp.WebViewDict = disableJsWorkerWebViewDict
			}
		}

		dictStr := textDecryptByOsv(osVer)

		if err := json.Unmarshal([]byte(dictStr), &okResp.Dict); err != nil {
			panic("unmarshal json error")
		}

		if osVersionEnableBundleList(osVer) {
			regionUri := cf.RegionConf + ip
			if country, err := getCountryWithIp(regionUri); err != nil {
				log.Println(err)
			} else {
				if bundleList = GetBundlesWithCountry(country); len(bundleList) == 0 {
					log.Println("GetBundlesWithCountry bundleList is nil, country: ", country)
				}
				okResp.BundleLists = bundleList
			}
		}

		// 修复SDK上报数据时crash的bug
		if sdkVer >= "2.3.7" && sdkVer <= "2.4.6" {
			okResp.Dict = nil
			okResp.BundleLists = nil
		}

		okResp.body, _ = json.Marshal(okResp)
		okResp.cipherBody = aes.EncryptBytes(okResp.body)

		if noCipher {
			okResp.WriteTo(w)
		} else {
			okResp.WriteCipherTo(w)
		}
	})
}

func HandleReviewIosPmtInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	copyMap := copyGverMap()
	body, _ := json.Marshal(copyMap)
	w.Write(body)
	return
}
