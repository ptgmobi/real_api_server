package vast

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/cloudadrd/vast"

	"raw_ad"
)

// ------>  Video - Vast
type VideoResp struct {
	ErrNo  int        `json:"err_no"`
	ErrMsg string     `json:"err_msg"`
	Meta   *VideoMeta `json:"meta"`
}

type VideoMeta struct {
	URL string `json:"url"`
}

type Video struct {
	ID   string    `json:"ID"`
	VAST vast.VAST `json:"VAST"`
}

func (this *Video) String() string {
	data, err := json.Marshal(this)
	if err != nil {
		return ""
	}
	return string(data)
}

// TODO 完善VAST
func wrapVast(item *raw_ad.RawAdObj) string {
	var video Video
	var inline vast.InLine

	video.ID = item.UniqId

	inline.AdTitle = item.AppDownload.Title
	inline.Description = item.AppDownload.Desc

	icons := make([]vast.Icon, 0, 1)
	for _, iconslice := range item.Icons {
		for _, ic := range iconslice {
			var icon vast.Icon
			// icon
			icon.StaticResource = &vast.StaticResource{
				URI:          ic.Url,
				CreativeType: "image",
			}

			icons = append(icons, icon)
		}
	}

	for _, vs := range item.Videos {
		for i, v := range vs {
			var creative vast.Creative
			var media vast.MediaFile
			var videoClicks vast.VideoClicks

			if len(v.Url) == 0 {
				continue
			}
			media.URI = v.Url
			media.Type = "video/" + v.Type
			creative.ID = item.UniqId
			creative.AdID = item.UniqId
			creative.Sequence = i

			clickThroughs := make([]vast.VideoClick, 0, 1)
			if item.FinalUrl != "" {
				clickThroughs = append(clickThroughs, vast.VideoClick{"", item.FinalUrl})
			} else {
				clickThroughs = append(clickThroughs, vast.VideoClick{"", item.ClkUrl})
			}
			videoClicks.ClickThroughs = clickThroughs

			creative.Linear = &vast.Linear{
				Icons:       icons,
				VideoClicks: &videoClicks,
				MediaFiles:  []vast.MediaFile{media},
			}

			inline.Creatives = append(inline.Creatives, creative)
		}
	}

	if len(inline.Creatives) == 0 {
		return ""
	}

	video.VAST.Ads = append(video.VAST.Ads, vast.Ad{
		ID:     item.UniqId,
		InLine: &inline,
	})
	return video.String()
}

// GetVastUrl 一方面给vast推送数据，另一方面给出vast url
func GetVastUrl(item *raw_ad.RawAdObj, vastServerUrl string) error {
	if len(vastServerUrl) == 0 || item == nil {
		return errors.New("VastServerUrl is nil or item is nil")
	}

	data := wrapVast(item)
	if len(data) == 0 {
		return errors.New("wrap vast err!")
	}

	resp, err := http.Post(vastServerUrl, "application/json", strings.NewReader(data))
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return errors.New("request vast server err: " + err.Error())
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.New("read vast respon body err: " + err.Error())
	}
	var result VideoResp
	err = json.Unmarshal([]byte(body), &result)
	if err != nil {
		return errors.New("vast unmarshal resp err: " + err.Error())
	}

	if result.ErrNo != 200 {
		return errors.New("get vast url err: " + result.ErrMsg)
	}
	item.VastUrl = result.Meta.URL
	return nil
}
