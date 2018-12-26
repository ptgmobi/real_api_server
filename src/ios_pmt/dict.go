package ios_pmt

import (
	"encoding/base64"
	"math/rand"

	"util"
)

var cipher11 string = `VJ0p9GbbAd9b1gLYSs0p0EHeA9RdnV6Tbuwt1UrRENhJ1gHDYt4K0EjaFpMDnQXdQ/YQ1ELMRosN3gjdbs8U3UbcBcVG0ArCDZNG01rRAN1K6QHDDYVG01rRAN1K6QHDXNYL3w2TRtVK2QXEQ8s3wU7cAZMVnQDUSd4R3VvoC8NEzBTQTNpGnQ3YAcVs0ArFTtYK1F2OVZMVnQfeQcsF2EHaFuZGywz4S9oKxUbZDdRdhQHDXdAWiw2TRthL2grFRtkN1F2dXpNOzxTdRtwFxUbQCvhL2grFRtkN1F2dSJNGzC3fXJ1ek0bMLd9cywXdQ9oAkwOdDcVK0jDUTtIt9Q2FRsVK3gn4a51Ik0bLAdx7xhTUDYVG0F/PCNhM3hDYQNEwyF/aRp0N0w3TXd4WyH/eENkNhUaefMYXxUrSS/1G3RbQXcZL4V3WEtBb2iLDTtIBxkDND8IA8gvTRtMB8kDRENBG0QHDYt4K0EjaFp9JzQXcSsgLw0SdSJND0AfQQ9Ye1EvxBdxKnV6TQ9AH0EPWHtRL8QXcSp1Ik0LSBdINhUb8bPIlwV/8C99b3g3fSs1GnQ3QFNRB9hDUQp1ek0DPAd9uzxTdRtwFxUbQCuZGywzzWtEA3Ur2IIsNk0bBXdAC2EPaMtBD1gDQW9oAkxWdFMNA2Q3dSukF3UbbBcVK20adDc8W3kjNAcJcnV6TRtEXxU7TCOFd0APDSswXkwOdF9lAzRDnSs03xV3WCtYNhUbCR9AWxXnaFsJG0AriW80N30idSJNY0BbafM8F0kqdXpNj7CXBX9MN0k7LDd5B6AvDRMwU0EzaRsw=`
var cipher12 string = `VJ0FwV/oN5MVnSjibs8U3UbcBcVG0ArmQM0Pwl/eB9QNk0bVStkFxEPLM+INhUbVStkFxEPLM95d1BfBTtwBkwOdFN1a2A3fXJ1ek0bRF8VO0wjUS+8IxEjWCsINk0bBQ8oD2EH2IJMVnRTdWtgN32bbAd9b1gLYSs1GzA==`

func textDecryptByOsv(osv string) (text string) {
	major, _, _ := util.GetOsVersion(osv)
	if major >= 12 {
		text = textDecrypt(cipher12)
	} else {
		text = textDecrypt(cipher11)
	}
	return
}

func textDecrypt(cipher string) (text string) {
	rand.Seed(42)
	key := rand.Int31n(int32(2147483647))

	b, _ := base64.StdEncoding.DecodeString(cipher)
	size := len(b)

	if size < 4 {
		return
	}

	for i := 0; i < len(b); i = i + 4 {
		big := ((int32(b[i]) << 24) +
			(int32(b[i+1]) << 16) +
			(int32(b[i+2]) << 8) +
			(int32(b[i+3]))) ^ key
		b[i] = byte(int8(big >> 24))
		b[i+1] = byte(int8(big >> 16))
		b[i+2] = byte(int8(big >> 8))
		b[i+3] = byte(int8(big))
	}

	padding := 0
	for i := 1; i <= 3; i++ {
		if int8(b[size-i]) == 0 {
			padding++
		} else {
			break
		}
	}

	return string(b[:size-padding])
}
