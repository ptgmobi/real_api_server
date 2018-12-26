package vast_module

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"log"

	"github.com/cloudadrd/vast"

	"ad"
)

var trackEvents = map[string]int{
	"creativeView":  0,
	"start":         1,
	"firstQuartile": 2,
	"midpoint":      3,
	"thirdQuartile": 4,
	"complete":      5,
	"closeLinear":   16,
}

type Video struct {
	ID   string    `xml:"ID"`
	VAST vast.VAST `xml:"VAST"`
}

// native video v4
func NativeV4(ad *ad.NativeVideoV4Ad) string {
	var video Video
	// 自己的广告，都是inline
	var inline vast.InLine

	nativeAdObj := &ad.Core

	// 广告ID
	uniqId := ad.Channel + "_" + ad.Id
	video.ID = uniqId

	inline.AdTitle = nativeAdObj.Title
	inline.Description = nativeAdObj.Desc

	// 选中icon
	icons := make([]vast.Icon, 0, 1)
	icons = append(icons, vast.Icon{
		StaticResource: &vast.StaticResource{
			URI:          nativeAdObj.Icon,
			CreativeType: "image",
		},
	})

	// 选中video，目前存在BackCreativeObj中
	var creative vast.Creative
	var media vast.MediaFile
	var videoClicks vast.VideoClicks

	media.URI = ad.Video.Url
	creative.AdID = uniqId

	// ClickThrought
	clkUrl := ""
	if ad.FinalUrl != "" {
		clkUrl = ad.FinalUrl
	} else {
		clkUrl = ad.ClkUrl
	}
	clickTroughs := make([]vast.VideoClick, 0)
	clickTroughs = append(clickTroughs,
		vast.VideoClick{URI: clkUrl})

	// ClickTrackings
	clickTrackings := make([]vast.VideoClick, 0, len(ad.ClkTks))
	for _, clk := range ad.ClkTks {
		clickTrackings = append(clickTrackings,
			vast.VideoClick{URI: clk})
	}

	impUrls := ad.ImpTks
	imps := make([]vast.Impression, 0, len(ad.ImpTks))
	for _, imp := range ad.ImpTks {
		imps = append(imps, vast.Impression{URI: imp})
	}

	// TrackEvent
	tracks := make([]vast.Tracking, 0, len(trackEvents))
	if len(impUrls) > 0 {
		for event, id := range trackEvents {
			tracks = append(tracks, vast.Tracking{
				Event: event,
				URI:   fmt.Sprintf("%s&track_event=%d", impUrls[0], id),
			})
		}
	}

	videoClicks.ClickThroughs = clickTroughs
	videoClicks.ClickTrackings = clickTrackings
	creative.Linear = &vast.Linear{
		Icons:          icons,
		VideoClicks:    &videoClicks,
		MediaFiles:     []vast.MediaFile{media},
		TrackingEvents: tracks,
	}

	inline.Creatives = append(inline.Creatives, creative)
	inline.Impressions = imps
	video.VAST.Ads = append(video.VAST.Ads, vast.Ad{
		ID:     uniqId,
		InLine: &inline,
	})
	video.VAST.Version = "3.0"

	return video.XML()
}

// video v4
func FillV4Vast(ad *ad.VideoAdObj) {
	var video Video
	// 自己的广告，都是inline
	var inline vast.InLine

	// 广告ID
	uniqId := ad.Channel + "_" + ad.Id
	video.ID = uniqId

	inline.AdTitle = ad.Title
	inline.Description = ad.Desc

	// 选中icon
	icons := make([]vast.Icon, 0, 1)
	icons = append(icons, vast.Icon{
		StaticResource: &vast.StaticResource{
			URI:          ad.Icon,
			CreativeType: "image",
		},
	})

	// 选中video，目前存在BackCreativeObj中
	var creative vast.Creative
	var media vast.MediaFile
	var videoClicks vast.VideoClicks

	media.URI = ad.RewardedVideo.Video.Url
	creative.ID = uniqId
	creative.AdID = uniqId

	// ClickThrought
	clkUrl := ""
	if ad.FinalUrl != "" {
		clkUrl = ad.FinalUrl
	} else {
		clkUrl = ad.ClkUrl
	}
	clickTroughs := make([]vast.VideoClick, 0)
	clickTroughs = append(clickTroughs,
		vast.VideoClick{URI: clkUrl})

	// ClickTrackings
	clkTracks := ad.ClkTks
	clickTrackings := make([]vast.VideoClick, 0, len(clkTracks))
	for _, url := range clkTracks {
		clickTrackings = append(clickTrackings,
			vast.VideoClick{URI: url})
	}

	impUrls := ad.ImpTks
	imps := make([]vast.Impression, 0, len(impUrls))
	for _, imp := range impUrls {
		imps = append(imps, vast.Impression{URI: imp})
	}

	// TrackEvent
	tracks := make([]vast.Tracking, 0, len(trackEvents))
	if len(impUrls) > 0 {
		for event, id := range trackEvents {
			tracks = append(tracks, vast.Tracking{
				Event: event,
				URI:   fmt.Sprintf("%s&track_event=%d", impUrls[0], id),
			})
		}
	}

	// 对于头条的激励视频api，impression事件直接上报show_url
	// 放到vast.InLine的start事件中，是因为SDK没有正确的处理impression和creativeView事件
	if ad.Channel == "ttapi" {
		if showUrls, ok := ad.Ext.([]string); ok {
			for _, showUrl := range showUrls {
				tracks = append(tracks, vast.Tracking{
					Event: "start",
					URI:   showUrl,
				})
			}
		}
	}

	videoClicks.ClickThroughs = clickTroughs
	videoClicks.ClickTrackings = clickTrackings
	creative.Linear = &vast.Linear{
		Icons:          icons,
		VideoClicks:    &videoClicks,
		MediaFiles:     []vast.MediaFile{media},
		TrackingEvents: tracks,
	}

	inline.Creatives = append(inline.Creatives, creative)
	inline.Impressions = imps
	video.VAST.Ads = append(video.VAST.Ads, vast.Ad{
		ID:     uniqId,
		InLine: &inline,
	})
	video.VAST.Version = "3.0"

	ad.VastXmlData = video.XML()
}

func FillVast(ad *ad.AdObj) {
	var video Video
	// 自己的广告，都是inline
	var inline vast.InLine

	// 广告ID
	uniqId := ad.Channel + "_" + ad.Id
	video.ID = uniqId

	inline.AdTitle = ad.AppDownload.Title
	inline.Description = ad.AppDownload.Desc

	// 选中icon
	icons := make([]vast.Icon, 0, 1)
	icons = append(icons, vast.Icon{
		StaticResource: &vast.StaticResource{
			URI:          ad.IconChosen,
			CreativeType: "image",
		},
	})

	// 选中video，目前存在BackCreativeObj中
	var creative vast.Creative
	var media vast.MediaFile
	var videoClicks vast.VideoClicks

	media.URI = ad.BakCreative.Video.Url
	creative.ID = uniqId
	creative.AdID = uniqId

	// ClickThrought
	clkUrl := ""
	if ad.FinalUrl != "" {
		clkUrl = ad.FinalUrl
	} else {
		clkUrl = ad.ClkUrl
	}
	clickTroughs := make([]vast.VideoClick, 0)
	clickTroughs = append(clickTroughs,
		vast.VideoClick{URI: clkUrl})

	// ClickTrackings
	clkTracks := ad.BakCreative.BakClkTkUrl
	clickTrackings := make([]vast.VideoClick, 0, len(clkTracks))
	for _, url := range clkTracks {
		clickTrackings = append(clickTrackings,
			vast.VideoClick{URI: url})
	}

	impUrls := ad.BakCreative.BakImpTkUrl
	imps := make([]vast.Impression, 0, len(impUrls))
	for _, imp := range impUrls {
		imps = append(imps, vast.Impression{URI: imp})
	}
	// TrackEvent
	tracks := make([]vast.Tracking, 0, len(trackEvents))
	if len(impUrls) > 0 {
		for event, id := range trackEvents {
			tracks = append(tracks, vast.Tracking{
				Event: event,
				URI:   fmt.Sprintf("%s&track_event=%d", impUrls[0], id),
			})
		}
	}

	videoClicks.ClickThroughs = clickTroughs
	videoClicks.ClickTrackings = clickTrackings

	creative.Linear = &vast.Linear{
		Icons:          icons,
		VideoClicks:    &videoClicks,
		MediaFiles:     []vast.MediaFile{media},
		TrackingEvents: tracks,
	}

	inline.Creatives = append(inline.Creatives, creative)
	inline.Impressions = imps
	video.VAST.Ads = append(video.VAST.Ads, vast.Ad{
		ID:     uniqId,
		InLine: &inline,
	})
	video.VAST.Version = "3.0"

	ad.VastXmlData = video.XML()
}

// 返回XML base64数据
func (this *Video) XML() string {
	data, err := xml.Marshal(this.VAST)
	if err != nil {
		log.Println("[VAST] xml marshal err: ", err, ", id: ", this.ID)
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}
