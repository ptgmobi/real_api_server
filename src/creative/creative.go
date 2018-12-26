package creative

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Conf struct {
	Url string `json:"creative_info_manager_url"`
}

type CreativeResp struct {
	ErrMsg      string `json:"err_msg"`
	Id          string `json:"creative_id"`
	OldId       string `json:"creative_old_id"`
	DomesticCDN string `json:"domestic_url"`
	OverseasCDN string `json:"overseas_url"`
	Size        int64  `json:"size"`
}

var CreativeInfoManagerUrl string

func Init(cf *Conf) {
	CreativeInfoManagerUrl = cf.Url
}

func GetCreativeInfo(cUrl, region, cType string) *CreativeResp {
	if len(cUrl) == 0 || len(cType) == 0 {
		fmt.Println("empty cUrl or invalid creativeType, creativeUrl: ", cUrl, ", creativeType: ", cType)
		return nil
	}
	uri := CreativeInfoManagerUrl + "?ctype=" + cType + "&curl=" + base64.StdEncoding.EncodeToString([]byte(cUrl)) + "&region=" + region
	c := &http.Client{
		Timeout: time.Second * 1,
	}
	resp, err := c.Get(uri)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		fmt.Println("GetCreativeId http get error, creative_url: ", cUrl, ", error: ", err)
		return nil
	}

	var res CreativeResp
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		fmt.Println("GetCreativeId Decode error, creative_url :", cUrl, ", error: ", err)
		return nil
	}
	if len(res.ErrMsg) > 0 || len(res.Id) == 0 {
		fmt.Println("GetCreativeId error: ", res.ErrMsg, ", creative_url: ", cUrl,
			", creative_id: ", res.Id, ", creative size: ", res.Size)
		return nil
	}
	return &res
}
