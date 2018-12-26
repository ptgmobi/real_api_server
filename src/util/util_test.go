package util

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestAlphaTable(t *testing.T) {
	var texts []string = []string{
		"1987_10.0_0.24",
		"2016_0.06_0.01",
	}

	for _, text := range texts {
		if text != AlphaTableDecode(AlphaTableEncode(text)) {
			t.Error("alphaTable error")
		}
	}
}

func TestRestfulReg(t *testing.T) {
	// sb just means String-Boolean, do NOT think about other things, lol
	type sb struct {
		s string
		b bool
	}

	sbs := []sb{
		{"http://api.yeahmobi.com", true},
		{"https://api.yeahmobi.com", true},
		{"httpss://api.yeahmobi.com", false},
		{"httpt://api.yeahmobi.com", false},
		{" https://api.yeahmobi.com", false},
		{"htps://api.yeahmobi.com", false},
		{"api.yeahmobi.com", false},
		{"https://github.com", true},
		{" https://github.com", false},
		{"", false},
	}

	for _, sb := range sbs {
		if sb.b != IsRestfulUri(sb.s) {
			t.Error("unexpected result (", !sb.b, ") of string", sb.s)
		}
	}
}

func BenchmarkUUID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = UUID(strconv.Itoa(i))
	}
}

func TestFileSizeStr2Int(t *testing.T) {
	if n, err := FileSizeStr2Int("103 B"); err == nil {
		if n != 103 {
			t.Error("unexpected return of: ", n)
		}
	} else {
		t.Error("unexpected error: ", err)
	}

	if n, err := FileSizeStr2Int("1023 KB"); err == nil {
		if n != 1023*1024 {
			t.Error("unexpected return of: ", n)
		}
	} else {
		t.Error("unexpected error: ", err)
	}

	if n, err := FileSizeStr2Int("1023 MB"); err == nil {
		if n != 1023*1024*1024 {
			t.Error("unexpected return of: ", n)
		}
	} else {
		t.Error("unexpected error: ", err)
	}

	if n, err := FileSizeStr2Int("1023 GB"); err == nil {
		if n != 1023*1024*1024*1024 {
			t.Error("unexpected return of: ", n)
		}
	} else {
		t.Error("unexpected error: ", err)
	}

	if n, err := FileSizeStr2Int("1023 TB"); err == nil {
		if n != 1023*1024*1024*1024*1024 {
			t.Error("unexpected return of: ", n)
		}
	} else {
		t.Error("unexpected error: ", err)
	}

	if n, err := FileSizeStr2Int("1023 PB"); err == nil {
		if n != 1023*1024*1024*1024*1024*1024 {
			t.Error("unexpected return of: ", n)
		}
	} else {
		t.Error("unexpected error: ", err)
	}

	if _, err := FileSizeStr2Int("1023KB"); err == nil {
		t.Error("should return error")
	} else if err.Error() != "fmt error: 1023KB" {
		t.Error("unexpected error message: ", err)
	}

	if _, err := FileSizeStr2Int("-1 KB"); err == nil {
		t.Error("should return error")
	} else if err.Error() != "size error: -1 KB" {
		t.Error("unexpected error message: ", err)
	}

	if _, err := FileSizeStr2Int("12 SB"); err == nil {
		t.Error("should return error")
	} else if err.Error() != "unit error: 12 SB" {
		t.Error("unexpected error message: ", err)
	}
}

func TestFileSize2Str(t *testing.T) {
	type is struct {
		i int
		s string
	}

	K := 1024
	M := K * 1024
	G := M * 1024
	T := G * 1024
	P := T * 1024

	iss := []is{
		{-100, "Unknown"},
		{-1, "Unknown"},
		{0, "0 B"},
		{100, "100 B"},
		{K, "1 KB"},
		{K + 1, "1 KB"},
		{2 * K, "2 KB"},
		{2*K + 1, "2 KB"},
		{M - 1, "1023 KB"},
		{M, "1 MB"},
		{M + 1, "1 MB"},
		{M + K, "1 MB"},
		{M + K + 1, "1 MB"},
		{M + M - 1, "1 MB"},
		{M + M + 1, "2 MB"},
		{G + K, "1 GB"},
		{G + M + K, "1 GB"},
		{2*G + M + K, "2 GB"},
		{42*G + M + K, "42 GB"},
		{1024*G + M + K, "1 TB"},
		{42 * T, "42 TB"},
		{1023 * T, "1023 TB"},
		{1024 * T, "1 PB"},
		{42 * P, "42 PB"},
		{1023 * P, "1023 PB"},
		{1024 * P, "Unknown"},
	}

	for _, is := range iss {
		s := FileSize2Str(is.i)
		if is.s != s {
			t.Error("unexpected result (", s, ") of ", is.i, "==>", is.s)
		}
	}
}

func TestSplitHelper(t *testing.T) {
	strListCmp := func(l1, l2 []string) bool {
		if len(l1) != len(l2) {
			return false
		}
		for i := 0; i != len(l1); i++ {
			if l1[i] != l2[i] {
				return false
			}
		}
		return true
	}
	m := map[string][]string{
		"Entertainment": []string{"entertainment"},
		"Finance":       []string{"finance"},
		"Food & Drink":  []string{"food", "drink"},
		"Games":         []string{"games"},
		"Photo & Video": []string{"photo", "video"},
		"Adult":         []string{"adult"},
	}
	for k, v := range m {
		if !strListCmp(SplitHelper(k, " & ", strings.ToLower), v) {
			t.Error("test case error: ", k, v)
		}
	}
}
func TestDecodeJsonArrayStreamApp(t *testing.T) {
	type AppInfo struct {
		Id          int      `json:"id"`
		AppName     string   `json:"app_name"`
		Slots       []int    `json:"slots"`
		MainTagId   int      `json:"main_tag_id"`
		SecondTagId int      `json:"second_tag_id"`
		Platform    int      `json:"platform"`
		Keywords    []string `json:"string"`
		Version     string   `json:"version"`
		Url         string   `json:"url"`
		PkgName     string   `json:"package_name"`
		Dau         int      `json:"dau"`
		BlackList   []int    `json:"black_list"`
		Gps         int      `json:"gps"`
		Charge      int      `json:"charge"`

		Male   int `json:"male"`
		Female int `json:"female"`

		AgeId     []int `json:"age_id"`
		AppSwitch int   `json:"app_switch"`

		Conf map[string]int `json:"conf"`

		CreateTime int `json:"create_time"`
		UpdateTime int `json:"update_time"`
	}

	f, err1 := os.Open("appinfo.dat")
	if err1 != nil {
		t.Error("open file appinfo.dat error:", err1)
	}
	defer f.Close()

	ch, err2 := DecodeJsonArrayStream(f,
		func(dec *json.Decoder, ch chan<- interface{}) error {
			var app AppInfo
			if err := dec.Decode(&app); err != nil {
				t.Error("decode app error: ", err)
				return err
			}
			ch <- app
			return nil
		})
	if err2 != nil {
		t.Error("decode json array stream error: ", err2)
	}

	cnt := 0
	for app := range ch {
		cnt++
		if _, ok := app.(AppInfo); !ok {
			t.Error("app type assert error")
		}
	}
	if cnt != 3 {
		t.Error("unexpected cnt value of ", cnt, ", 3 expected")
	}
}

func TestDecodeJsonArrayStreamSlot(t *testing.T) {
	type TemplateObj struct {
		Type int    `json:"type"`
		H5   string `json:"h5"`
		Id   int    `json:"id"`
	}

	type SlotInfo struct {
		Id       int    `json:"id"`
		AppId    int    `json:"app_id"`
		SlotName string `json:"slot_name"`
		Format   int    `json:"format"`

		Templates []TemplateObj `json:"templates"`

		FrequencySwitch int `json:"frequency_switch"`
		SlotSwitch      int `json:"slot_switch"`

		CreateTime int `json:"create_time"`
		UpdateTime int `json:"update_time"`
	}

	f, err1 := os.Open("slotinfo.dat")
	if err1 != nil {
		t.Error("open file slotinfo.dat error:", err1)
	}
	defer f.Close()

	ch, err2 := DecodeJsonArrayStream(f,
		func(dec *json.Decoder, ch chan<- interface{}) error {
			var slot SlotInfo
			if err := dec.Decode(&slot); err != nil {
				t.Error("decode sloterror: ", err)
				return err
			}
			ch <- slot
			return nil
		})
	if err2 != nil {
		t.Error("decode json array stream error: ", err2)
	}

	cnt := 0
	for slot := range ch {
		cnt++
		if _, ok := slot.(SlotInfo); !ok {
			t.Error("slot type assert error")
		}
	}
	if cnt != 1 {
		t.Error("unexpected cnt value of ", cnt, ", 1 expected")
	}
}

func genBytesBuffer(nBytes uint) *bytes.Buffer {
	var buff bytes.Buffer
	for i := uint(0); i != nBytes; i++ {
		buff.WriteByte(byte(rand.Int() % 256))
	}
	return &buff
}

type encFunc func([]byte) ([]byte, error)

func encodingHelper(n uint, enc, dec encFunc, t *testing.T) {
	var buff *bytes.Buffer
	buff = genBytesBuffer(n)
	b, err := enc(buff.Bytes())
	if err != nil {
		t.Error("Encode err: ", err, "(", n, "bytes )")
	}
	if b, err = dec(b); err != nil {
		t.Error("Decode err: ", err, "(", n, "bytes )")
	}
	if bytes.Compare(b, buff.Bytes()) != 0 {
		t.Error("encode/decode un-equal (", n, "bytes )")
	}
}
func TestBase64(t *testing.T) {
	base64TestWrapper := func(n uint) {
		encodingHelper(n, Base64Encode, Base64Decode, t)
	}
	base64TestWrapper(0)
	base64TestWrapper(4)
	base64TestWrapper(10)
	base64TestWrapper(1000)
	base64TestWrapper(1000 * 1000)
}

func TestGzip(t *testing.T) {
	gzipTestWrapper := func(n uint) {
		encodingHelper(n, GzipEncode, GzipDecode, t)
	}
	gzipTestWrapper(0)
	gzipTestWrapper(4)
	gzipTestWrapper(10)
	gzipTestWrapper(1000)
	gzipTestWrapper(1000 * 1000)
}

func TestFinalUrlFilter(t *testing.T) {
	type TestCase struct {
		url   string
		legal bool
	}
	testCases := []TestCase{
		{"https://play.google.com/store/apps/details?id=en.co.atm.unison", true},
		{"market://details?id=air.com.buffalo_studios.newflashbingo", true},
		{"https://itunes.apple.com/app/id302584613", true},
		{"itms-apps://itunes.apple.com/app/id333903271", true},
		{"http://api.oceanbys.com/tracking/index/58513613a35e5?gaid={google_adv_id}", false},
	}
	for _, c := range testCases {
		if (c.url == FinalUrlFilter(c.url)) != c.legal {
			t.Error("schema error:", c.url)
		}
	}
}

func TestShuffle(t *testing.T) {
	rand.Seed(0)

	a := []int{1, 2, 3, 4}
	b := []int{2, 3, 5, 6}
	c := []int{}
	d := []int{1}
	Shuffle(a) // [4, 2, 1, 3]
	Shuffle(b) // [3, 2, 6, 5]
	Shuffle(c) // []
	Shuffle(d) // [1]
	check := func(a, b []int) bool {
		if len(a) != len(b) {
			return false
		}
		for i := 0; i < len(a); i++ {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	if !check(a, []int{4, 2, 1, 3}) {
		t.Error("TestShuffle err a: ", a)
	}
	if !check(b, []int{3, 2, 6, 5}) {
		t.Error("TestShuffle err b: ", b)
	}
	if !check(c, []int{}) {
		t.Error("TestShuffle err c: ", c)
	}
	if !check(d, []int{1}) {
		t.Error("TestShuffle err d: ", d)
	}
}

func TestUserHash(t *testing.T) {
	uids := map[string]string{
		"25FBC0EC-BEC7-41D8-A7E8-C2966A598E2A": "303",
		"668981BA-1B7A-4AC4-9955-193BAB572558": "362",
		"01A3C689-1444-492F-BBEC-D689D953F66A": "807",
	}
	for id, hash := range uids {
		if h := GetUserHash(id); h != hash {
			t.Error("GetUserHash: ", id, " expected: ", hash, " got: ", h)
		}
	}
}
