package ct_bloom

import (
	"encoding/base64"
	"hash/crc32"
	"sort"
	"strconv"
)

type Hash func(string) uint

var hashs [7]Hash
var bits [8]uint8 = [8]uint8{
	128, 64, 32, 16, 8, 4, 2, 1,
}

type BloomFilter struct {
	bucket [512 / 8]byte
}

func init() {
	for i := 0; i != 7; i++ {
		func(i int) {
			hashs[i] = func(s string) uint {
				return uint(crc32.Checksum([]byte(strconv.Itoa(i+1)+s), crc32.IEEETable) % 512)
			}
		}(i)
	}
}

func NewBloomFilter(s string) *BloomFilter {
	bucket, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	p := &BloomFilter{}
	copy(p.bucket[:], bucket)
	return p
}

func setBit(b *byte, nbit uint8) {
	*b = byte(uint8(*b) | bits[nbit])
}

func (bf *BloomFilter) addNum(n uint) {
	nByte := n / 8
	nBit := uint8(n % 8)
	setBit(&bf.bucket[nByte], nBit)
}

func (bf *BloomFilter) Add(s string) {
	for i := 0; i != len(hashs); i++ {
		n := hashs[i](s)
		bf.addNum(n)
	}
}

type UIntSlice []uint

func (us UIntSlice) Len() int           { return len(us) }
func (us UIntSlice) Less(i, j int) bool { return us[i] < us[j] }
func (us UIntSlice) Swap(i, j int)      { us[i], us[j] = us[j], us[i] }

func CalcIndexes(s string) (rc []uint) {
	for i := 0; i != len(hashs); i++ {
		rc = append(rc, hashs[i](s))
	}
	sort.Sort(UIntSlice(rc))
	return
}

func (bf *BloomFilter) testNum(n uint) bool {
	nByte := n / 8
	nBit := uint8(n % 8)

	return (uint8(bf.bucket[nByte]) & bits[nBit]) != 0
}

func (bf *BloomFilter) Test(s string) bool {
	for i := 0; i != len(hashs); i++ {
		n := hashs[i](s)
		if !bf.testNum(n) {
			return false
		}
	}
	return true
}

func (bf *BloomFilter) TestAdd(s string) bool {
	rc := bf.Test(s)
	if !rc {
		bf.Add(s)
	}
	return rc
}

func (bf *BloomFilter) GetIndex() (rc []uint) {
	for i, b := range bf.bucket {
		for j := 0; j != 8; j++ {
			if bits[j]&b != 0 {
				rc = append(rc, uint(i*8+j))
			}
		}
	}
	return
}

func (bf *BloomFilter) TestIndex(index []uint) bool {
	for _, n := range index {
		if !bf.testNum(n) {
			return false
		}
	}
	return true
}

func (bf *BloomFilter) Base64EncodeBucket() string {
	return base64.StdEncoding.EncodeToString(bf.bucket[:])
}
