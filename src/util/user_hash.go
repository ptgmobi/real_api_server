package util

import (
	"crypto/md5"
	"fmt"
	"hash/crc32"
)

func GetUserHash(uid string) string {
	if uid == "" {
		return "0"
	}
	md5Bytes := md5.Sum([]byte("bus-" + uid))
	b := fmt.Sprintf("%x", md5Bytes)
	hash := crc32.ChecksumIEEE([]byte(b)) % 1000
	return fmt.Sprint(hash)
}
