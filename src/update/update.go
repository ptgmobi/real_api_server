package update

import (
	"encoding/json"
	"errors"
	"log"
	"store"
	"strings"
	"sync"
	"time"

	dnf "github.com/brg-liuwei/godnf"
	"github.com/brg-liuwei/gotools"

	"raw_ad"
)

type Conf struct {
	OfferUpdateApi string `json:"offer_update_api"`

	VastServerUrl string `json:"vast_server_url"`

	YeahmobiAffId           string `json:"ym_aff_id"`
	YeahmobiNonPreloadAffId string `json:"nym_aff_id"`
	YeahmobiThirdPartAffId  string `json:"tym_aff_id"`
	YeahmobiIrsAffId        string `json:"iym_aff_id"`
	YeahmobiStartAppAffId   string `json:"stpym_aff_id"`
	YeahmobiInmobiAffId     string `json:"inym_aff_id"`
	YeahmobiJapanAffId      string `json:"jpym_aff_id"`
	YeahmobiJapan2AffId     string `json:"jpym2_aff_id"`
	YeahmobiVideoAffId      string `json:"vym_aff_id"`
	YeahmobiN1AffId         string `json:"n1ym_aff_id"`
	YeahmobiN2AffId         string `json:"n2ym_aff_id"`
	YeahmobiN3AffId         string `json:"n3ym_aff_id"`
	YeahmobiSAffId          string `json:"sym_aff_id"`
	YeahmobiVideo1AffId     string `json:"v1ym_aff_id"`

	SspImpApi        string `json:"ssp_imp_api"`
	DashImpApi       string `json:"dash_imp_api"`
	DashCompleteApi  string `json:"dash_complete_api"`
	ChannelStatusApi string `json:"channel_status_api"`

	ChannelOfferFilterApi string `json:"channel_offer_filter_api"`
	ExtraClkCountApi      string `json:"extra_click_count_api"`

	LogPath        string `json:"log_path"`
	LogRotateNum   int    `json:"log_rotate_backup"`
	LogRotateLines int    `json:"log_rotate_lines"`

	OfflineOfferApi string `json:"offline_offer_api"`

	ChannelStatus map[string]bool `json:"channel_status"`

	// s3
	S3Region    string `json:"s3_region"`
	OfferBucket string `json:"offer_bucket"`
	OfferKey    string `json:"offer_key"`
	SspKey      string `json:"ssp_key"`
	ChannelKey  string `json:"channel_key"`
	DownPath    string `json:"down_path"`
}

func (s *Service) clearUpdateHandlers() {
	s.Lock()
	defer s.Unlock()

	s.updateHandlers = make(map[string]bool, 16)
}

func (s *Service) setStatus(ch string, status int) {
	s.Lock()
	defer s.Unlock()

	if status == 1 {
		s.updateHandlers[ch] = true
	} else {
		s.updateHandlers[ch] = false
	}
}

func (s *Service) getStatus() []string {
	s.RLock()
	defer s.RUnlock()

	chs := make([]string, 0, len(s.updateHandlers))
	for ch, ok := range s.updateHandlers {
		if ok {
			chs = append(chs, strings.TrimSpace(ch))
		}
	}

	return chs
}

type Service struct {
	conf            *Conf
	l               *gotools.RotateLogger
	updateHandlers  map[string]bool
	capExceedOffers *gotools.ExpiredMap
	sync.RWMutex

	dnfHandler *dnf.Handler
}

func NewService(conf *Conf) (*Service, error) {
	l, err := gotools.NewRotateLogger(conf.LogPath,
		"[UPDATE] ", log.LstdFlags|log.LUTC, conf.LogRotateNum)
	if err != nil {
		return nil, errors.New("[UPDATE] NewRotateLogger failed: " + err.Error())
	}
	l.SetLineRotate(conf.LogRotateLines)

	srv := &Service{
		conf:            conf,
		l:               l,
		capExceedOffers: gotools.NewExpiredMap(4096),

		dnfHandler: dnf.NewHandler(),
	}

	srv.updateHandlers = conf.ChannelStatus

	return srv, nil
}

func (s *Service) SetRawAdRate(raw *raw_ad.RawAdObj) {
	// disable preclick if CPL
	if raw.PayoutType == "CPL" {
		raw.PreRate = 0.0
	}
}

func (s *Service) downloadChannelStatus() {
	data, err := store.DownloadFile(s.conf.S3Region, s.conf.DownPath,
		s.conf.OfferBucket, s.conf.ChannelKey)
	if err != nil {
		s.l.Println("[s3] download channel status failed.")
		return
	}
	var chsStatus []struct {
		Channel string `json:"channel"`
		Type    []int  `json:"type"`
	}
	if err = json.Unmarshal(data, &chsStatus); err != nil {
		s.l.Println("[s3] json unmarshal channel status error: ", err)
		return
	}

	for i := 0; i != len(chsStatus); i++ {
		s.setStatus(chsStatus[i].Channel, 1)
	}
}

func (s *Service) Serve() {
	// dnf
	dnf.SetHandler(s.dnfHandler)

	// 从s3更新offer
	go func() {
		for {
			// 下载channel状态信息
			s.downloadChannelStatus()

			chs := s.getStatus()
			for i := 0; i < len(chs); i++ {
				offerCnt := 0
				s.doUpdateS3(chs[i], &offerCnt)
				s.l.Printf("[%s] update ok, raw cnt: %d", chs[i], offerCnt)
			}

            s.doUpdateS3TmpChannels()

			// offer更新完成后设置当前dnf
			dnf.SetHandler(s.dnfHandler)

			// 每五分钟更换一次
			time.Sleep(time.Duration(5) * time.Minute)
			s.dnfHandler = dnf.NewHandler()
		}
	}()
}
