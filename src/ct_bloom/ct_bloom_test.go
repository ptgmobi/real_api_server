package ct_bloom

import (
	"math/rand"
	"testing"
)

func uintArrayEqual(a, b []uint) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i != len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestNewBloomFilterErrorInput(t *testing.T) {
	if NewBloomFilter("123456") != nil {
		t.Error("NewBloomFilter(`error param`) expected nil return")
	}
}

/*
Android BloomFilter:
Test data and result:
测试字符数组:
[Life begins at the end of your comfort zone, AAA.com, aaa.com, test.com, a.b.c.d.e.f.g]

bit分布: {8, 9, 12, 15, 32, 68, 75, 98, 109, 110, 142, 151, 162, 170, 207, 218, 243, 260, 290, 313, 320, 327, 349, 372, 377, 386, 388, 398, 423, 441, 449, 451, 468, 486, 490}

Bucket BASE64: AMkAAIAAAAAIEAAAIAYAAAACAQAgIAAAAAEAIAAAEAAIAAAAIAAAQIEAAAQAAAhAKAIAAAEAAEBQAAgAAiAAAA==
*/
func TestCtBloomFilter(t *testing.T) {
	var testArray []string = []string{
		"Life begins at the end of your comfort zone",
		"AAA.com",
		"aaa.com",
		"test.com",
		"a.b.c.d.e.f.g",
	}
	var expected []uint = []uint{
		8, 9, 12, 15, 32, 68, 75, 98,
		109, 110, 142, 151, 162, 170,
		207, 218, 243, 260, 290, 313,
		320, 327, 349, 372, 377, 386,
		388, 398, 423, 441, 449, 451,
		468, 486, 490}

	var bf BloomFilter
	for _, s := range testArray {
		bf.TestAdd(s)
	}

	result := bf.GetIndex()

	if !uintArrayEqual(result, expected) {
		t.Error("index result: ", result, ", expected: ", expected)
	}

	base64Expected := "AMkAAIAAAAAIEAAAIAYAAAACAQAgIAAAAAEAIAAAEAAIAAAAIAAAQIEAAAQAAAhAKAIAAAEAAEBQAAgAAiAAAA=="
	base64Result := bf.Base64EncodeBucket()

	if base64Expected != base64Result {
		t.Error("base64 result: ", base64Result, ", expected: ", base64Expected)
	}

	nbf := NewBloomFilter(base64Expected)

	for _, s := range testArray {
		if !nbf.Test(s) {
			t.Error("test string in testArray error, s: ", s)
		}
	}
}

func TestIndex(t *testing.T) {
	skyWalker := "skywalker.liu"
	var bf BloomFilter

	index := CalcIndexes(skyWalker)

	bf.Add(skyWalker)

	if !bf.TestIndex(index) {
		t.Error("test index fail, index: ", index)
	}

	ctIndex := bf.GetIndex()
	if !uintArrayEqual(index, ctIndex) {
		t.Error("index un-equal: ", index, ctIndex)
	}
}

func TestIndexFail(t *testing.T) {
	skyWalker := "skywalker.liu"
	var bf BloomFilter

	index := CalcIndexes(skyWalker)
	succ := false
	for !succ {
		n := rand.Intn(len(index))
		val := uint(rand.Intn(512))
		if val != index[n] {
			index[n] = val
			succ = true
		}
	}

	bf.Add(skyWalker)

	if bf.TestIndex(index) {
		t.Error("test index fail expected")
	}
}
