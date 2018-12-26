package aes

import (
	"crypto/aes"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

type Conf struct {
	Key string `json:"key"`
}

var key []byte

func Init(cf *Conf) {
	rand.Seed(time.Now().Unix())
	var err error
	key, err = hex.DecodeString(cf.Key)
	if err != nil {
		panic(err)
	}
	if len(key) != 32 {
		panic("key size error, only 32 bytes key supported (64 bytes hex string)")
	}
}

func Encrypt(text string) string {
	return string(EncryptBytes([]byte(text)))
}

func EncryptBytes(textBytes []byte) []byte {
	keyBlock, _ := aes.NewCipher(key)
	size := fmt.Sprintf("%016d", len(textBytes))

	randPadding := (16 - (len(textBytes) % 16)) % 16
	for i := 0; i != randPadding; i++ {
		textBytes = append(textBytes, byte(rand.Intn(256)))
	}

	textBytes = append(textBytes, []byte(size)...)

	cipherBytes := make([]byte, 0, len(textBytes))
	blockSize := keyBlock.BlockSize()
	enc := make([]byte, blockSize)

	for len(textBytes) > 0 {
		toEnc := textBytes[:blockSize]
		textBytes = textBytes[blockSize:]
		keyBlock.Encrypt(enc, toEnc)
		cipherBytes = append(cipherBytes, enc...)
	}

	rt := make([]byte, 2*len(cipherBytes))
	n := hex.Encode(rt, cipherBytes)
	return rt[:n]
}

func Decrypt(cipher string) string {
	return string(DecryptBytes([]byte(cipher)))
}

func DecryptBytes(cipher []byte) []byte {
	keyBlock, _ := aes.NewCipher(key)

	textBytes := make([]byte, 0, len(cipher))
	cipherBytes := make([]byte, len(cipher)/2)

	if _, err := hex.Decode(cipherBytes, cipher); err != nil {
		return []byte("")
	}

	var lastBlock []byte
	blockSize := keyBlock.BlockSize()
	dec := make([]byte, blockSize)
	for len(cipherBytes) > 0 {
		if len(cipherBytes) < blockSize {
			return []byte("")
		}
		toDec := cipherBytes[:blockSize]
		cipherBytes = cipherBytes[blockSize:]
		keyBlock.Decrypt(dec, toDec)
		textBytes = append(textBytes, dec...)
		lastBlock = dec
	}

	size, _ := strconv.Atoi(string(lastBlock))
	return textBytes[:size]
}
