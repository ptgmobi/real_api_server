package util

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var crc32Table *crc32.Table
var restfulReg *regexp.Regexp
var httpsReplaceReg *regexp.Regexp
var unicodeReg *regexp.Regexp
var finalReg *regexp.Regexp
var chineseReg *regexp.Regexp
var base64Reg *regexp.Regexp
var sizeUnit []string = []string{"B", "KB", "MB", "GB", "TB", "PB"}

// 移位密码表，加密 0~9._ 共12个字符
var alphaTable []rune = []rune{
	's', 't', 'u', 'p', 'i', 'd', 'l', 'w', 'm', 'a', 'k', 'e',
}

var alphaTableDecodeMap map[rune]string

func AlphaTableEncode(text string) string {
	s := make([]string, 0, len(text))
	for _, c := range text {
		var index int
		switch c {
		case '.':
			index = 10
		case '_':
			index = 11
		default:
			index, _ = strconv.Atoi(string(c))
		}
		s = append(s, string(alphaTable[index]))
	}
	return strings.Join(s, "")
}

func AlphaTableDecode(cipher string) string {
	s := make([]string, 0, len(cipher))
	for _, c := range cipher {
		s = append(s, alphaTableDecodeMap[c])
	}
	return strings.Join(s, "")
}

type Conf struct {
	ReplacingSlotId []string `json:"replacing_slot_id"`
	DeviceApiAddr   string `json:"device_api_addr"`
    WuganChannelMap map[string]bool `json:"wugan_channel_map"`
}

type ChannelOffereFilter struct {
	Channel string   `json:"channel"`
	Offers  []string `json:"offers"`
}

type SlotWhite struct {
	Id      string   `json:"id"`
	SlotIds []string `json:"slot_ids"`
}

type RWLockMap struct {
	sync.RWMutex
	Data map[string]interface{}
}

type RWLockMapInt struct {
	sync.RWMutex
	Data map[string]int
}

var replacingSlotId []string
var deviceApiAddr string
var wuganChannel map[string]bool

func NewRWLockMap(num int) *RWLockMap {
	return &RWLockMap{
		Data: make(map[string]interface{}, num),
	}
}

func NewRWLockMapInt(num int) *RWLockMapInt {
	return &RWLockMapInt{
		Data: make(map[string]int, num),
	}
}

func init() {
	// 0xD5828281 present the following polynomial:
	// x³²+ x³¹+ x²⁴+ x²²+ x¹⁶+ x¹⁴+ x⁸+ x⁷+ x⁵+ x³+ x¹+ x⁰
	crc32Table = crc32.MakeTable(0xD5828281)

	restfulReg = regexp.MustCompile("^https?://.+")
	httpsReplaceReg = regexp.MustCompile("^http://")
	finalReg = regexp.MustCompile("(^https://play\\.google\\.com|^https://itunes\\.apple\\.com|^itms-apps://itunes\\.apple\\.com|^market://)")
	unicodeReg = regexp.MustCompile("(\u000A|\u000B|\u000C|\u000D|\u000E|\u000F)")
	chineseReg = regexp.MustCompile("[\u4e00-\u9fa5]+")
	base64Reg = regexp.MustCompile("^[0-9a-zA-Z]+(={0,2})$")

	alphaTableDecodeMap = make(map[rune]string)
	for i, a := range alphaTable {
		if i == 10 {
			alphaTableDecodeMap[a] = "."
		} else if i == 11 {
			alphaTableDecodeMap[a] = "_"
		} else {
			alphaTableDecodeMap[a] = strconv.Itoa(i)
		}
	}
}

func Init(cf *Conf) {
	replacingSlotId = cf.ReplacingSlotId
	deviceApiAddr = cf.DeviceApiAddr
    wuganChannel = cf.WuganChannelMap
}

func WuganHitChannel(channel string) bool {
	return wuganChannel[channel]
}

func GetFuyuDevices(platform, country string, num int) []string {
	api := deviceApiAddr + "/" + strings.ToLower(platform) +
		"/get_user?country=" + strings.ToUpper(country) +
		"&num=" + strconv.Itoa(num)
	resp, err := http.Get(api)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		fmt.Println("GetFuyuDevices http get error: ", err)
		return nil
	}
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("GetFuyuDevices read body error: ", err)
		return nil
	}
	fuyuIds := strings.Split(string(bytes), ",")
	for i := 0; i < len(fuyuIds); i++ {
		fuyuIds[i] = strings.TrimSpace(fuyuIds[i])
	}
	return fuyuIds
}

func ContainChinese(s string) bool {
	return chineseReg.MatchString(s)
}

func IsBase64(s string) bool {
	return base64Reg.MatchString(s)
}

func IsRestfulUri(uri string) bool {
	return restfulReg.MatchString(uri)
}

func HttpPrefix2Https(url string) string {
	return httpsReplaceReg.ReplaceAllString(url, "https://")
}

func UnicodeReplace(s string) string {
	return unicodeReg.ReplaceAllString(s, " ")
}

func UUID(key string) int {
	r := rand.Int63()
	var buff bytes.Buffer
	fmt.Fprintf(&buff, "%s-%d", key, r)
	rc := int(crc32.Checksum(buff.Bytes(), crc32Table))
	if rc < 0 {
		return -rc
	}
	return rc
}

func UUIDStr(key string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(strconv.Itoa(int(time.Now().Unix())+UUID(key)))))
}

func ToMd5(s string) string {
	b := md5.Sum([]byte(s))
	return hex.EncodeToString(b[:])
}

func Rand8Bytes() string {
	r := rand.Intn(100000000)
	if r < 0 {
		r = -r
	}
	return fmt.Sprintf("%08d", r)
}

var unknown string = "Unknown"

func FileSize2Str(size int) string {
	if size < 0 {
		return unknown
	}
	for i := 0; i < len(sizeUnit); i++ {
		tmp := size / 1024
		if tmp == 0 {
			return fmt.Sprintf("%d %s", size, sizeUnit[i])
		}
		size = tmp
	}
	return unknown
}

func FileSizeStr2Int(size string) (int, error) {
	var n int
	var unit string
	_, err := fmt.Sscanf(size, "%d %s", &n, &unit)
	if err != nil {
		return -1, errors.New("fmt error: " + size)
	}
	if n < 0 {
		return -1, errors.New("size error: " + size)
	}
	x := 1
	for _, s := range sizeUnit {
		if s == unit {
			return n * x, nil
		}
		x = x * 1024
	}
	return -1, errors.New("unit error: " + size)
}

func Base64Encode(raw []byte) ([]byte, error) {
	var buff bytes.Buffer
	enc := base64.NewEncoder(base64.StdEncoding, &buff)
	_, err := enc.Write(raw)
	enc.Close()
	return buff.Bytes(), err
}

func Base64Decode(raw []byte) ([]byte, error) {
	dec := base64.NewDecoder(base64.StdEncoding, bytes.NewBuffer(raw))
	return ioutil.ReadAll(dec)
}

func SplitHelper(raw string, sep string, strDealer func(string) string) []string {
	list := strings.Split(raw, sep)
	for i := 0; i != len(list); i++ {
		list[i] = strDealer(list[i])
	}
	return list
}

type JsonDecodeDealer func(*json.Decoder, chan<- interface{}) error

func DecodeJsonArrayStream(r io.Reader,
	dealer JsonDecodeDealer) (<-chan interface{}, error) {

	dec := json.NewDecoder(r)
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}

	if start, ok := t.(json.Delim); !ok {
		return nil, errors.New("json array required")
	} else if start.String() != "[" {
		return nil, errors.New("unexpected start token of: " + start.String())
	}

	ch := make(chan interface{}, 1024)
	go func(ch chan interface{}) {
		defer close(ch)

		for dec.More() {
			if err := dealer(dec, ch); err != nil {
				fmt.Println("decode dealer error: ", err)
				break
			}
		}

		if t, err := dec.Token(); err != nil {
			fmt.Println("unexpected error when read close bracket: ", err)
		} else if delim, ok := t.(json.Delim); !ok {
			fmt.Println("unexpected token: ", t)
		} else if delim.String() != "]" {
			fmt.Println("unexpected end json delim: ", delim.String())
		}
	}(ch)
	return ch, nil
}

func MillisecondTs(t time.Time) int {
	return int(t.UnixNano()) / 1000000
}

func NowMillisecondTs() int {
	return MillisecondTs(time.Now())
}

func NowString() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}

func Ts2String(ms int64) string {
	return time.Unix(ms/1000, ms%1000).UTC().Format("2006-01-02 15:04:05")
}

func GzipEncode(raw []byte) ([]byte, error) {
	var buff bytes.Buffer
	enc := gzip.NewWriter(&buff)

	// 注意，这里不能使用defer enc.Close(),
	// 因为enc.Close()会写入EOF,
	// 如果使用defer，buff.Bytes()是在Close之前被调用
	if _, err := enc.Write(raw); err != nil {
		enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

func GzipDecode(encoded []byte) ([]byte, error) {
	dec, err := gzip.NewReader(bytes.NewBuffer(encoded))
	if err != nil {
		return nil, err
	}
	defer dec.Close()

	return ioutil.ReadAll(dec)
}

var sizeArr [][]int = [][]int{
	// {width, height},
	{500, 500},
	{950, 500},
	{720, 1280},
}

func GetRenderImages(offerId, platform string) map[string]string {
	m := make(map[string]string)
	urlFmt := "http://image.zcoup.com/creative/%s/%d/%d/%s.jpg"
	cdnUrlFmt := "https://cdn.image.zcoup.com/creative/%s/%d/%d/%s.jpg"

	for i := 0; i != len(sizeArr); i++ {
		imgUrl := fmt.Sprintf(urlFmt, offerId,
			sizeArr[i][0], sizeArr[i][1], platform)

		if checkRenderImages(imgUrl) {
			key := strconv.Itoa(sizeArr[i][0]*10000 + sizeArr[i][1])
			m[key] = fmt.Sprintf(cdnUrlFmt, offerId,
				sizeArr[i][0], sizeArr[i][1], platform)
		}
	}
	return m
}

func checkRenderImages(url string) bool {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("checkRenderImages http.Get(", url, ") error: ", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// fmt.Println("checkRenderImages http.Get(", url,
		// 	") status not 200: ", resp.StatusCode)
		return false
	}
	return true
}

func LaplaceSmooth(w, h int) float64 {
	return float64(w+1) / float64(h+1)
}

type printer interface {
	Println(...interface{})
}

func HttpError(w http.ResponseWriter, code int) {
	http.Error(w, http.StatusText(code), code)
}

func GenClickId(prefix string) string {
	rand.Seed(int64(time.Now().Nanosecond()))
	r := int(rand.Float32() * 10000000)
	if r < 1000000 {
		r = r + 1000000
	}
	return prefix + strconv.Itoa(r)
}

func FinalUrlFilter(url string) string {
	if finalReg.MatchString(url) {
		return url
	}
	return ""
}

func NormalizeRegion(region string) string {
	return strings.ToLower(strings.Replace(region, " ", "", -1))
}

func Shuffle(arr []int) {
	size := len(arr)
	if size == 0 {
		return
	}
	for i := size - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		arr[i], arr[j] = arr[j], arr[i]
	}
}
func GetReplacingSlotIds() []string {
	return replacingSlotId
}

func UpdateByPage(fmtUri string, updateFun func(string) (bool, error)) {

	size := 50
	for page := 1; ; page++ {
		url := fmt.Sprintf(fmtUri, page, size)
		finished, err := updateFun(url)
		if err != nil {
			fmt.Println("UpdateByPage err: ", err)
			return
		}
		if finished {
			fmt.Println("UpdateByPage finished, url: ", url)
			return
		}
	}
}

func StrJoinHyphen(ss ...string) string {
	return strings.Join(ss, "-")
}

func StrJoinUnderline(ss ...string) string {
	return strings.Join(ss, "_")
}

func GetKeyHash(key string) int {
	if len(key) == 0 {
		return 0
	}

	md5Sum := fmt.Sprintf("%x", md5.Sum([]byte(key)))
	return int(crc32.ChecksumIEEE([]byte(md5Sum)) % 1000)
}

func AppendArgsToMonitors(tks []string, argsStr string) []string {
	monitors := make([]string, len(tks))
	for i := 0; i != len(tks); i++ {
		if strings.Contains(tks[i], "?") {
			monitors[i] = tks[i] + "&" + argsStr
		} else {
			monitors[i] = tks[i] + "?" + argsStr
		}
	}
	return monitors
}

func GetOsVersion(osv string) (major, minor, micro int) {
	osvArr := strings.Split(osv, ".")
	major, _ = strconv.Atoi(osvArr[0])
	if len(osvArr) > 1 {
		minor, _ = strconv.Atoi(osvArr[1])
	}
	if len(osvArr) > 2 {
		micro, _ = strconv.Atoi(osvArr[2])
	}
	return
}
