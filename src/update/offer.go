package update

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"raw_ad"
	"store"
)

var s3pathReg *regexp.Regexp = regexp.MustCompile("^.*/([^/]+)/\\d+\\.gz$")

func getChannel(key string) string {
	rc := s3pathReg.FindAllStringSubmatch(key, -1)
	if len(rc) > 0 && len(rc[0]) == 2 {
		return rc[0][1]
	}
	return ""
}

func (s *Service) doUpdate(ch, url string, callback func()) (offerCnt int) {
	defer callback()

	if url == "" {
		s.l.Println("[", ch, "] disabled")
		return
	}

	resp, err := http.Get(url)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		s.l.Printf("[%s] request url: %s, err: %v", ch, url, err)
		return
	}

	errCnt := 0
	dec := json.NewDecoder(resp.Body)
	for {
		t, err := dec.Token()
		if err != nil {
			s.l.Printf("[%s] dec token err: %v", ch, err)
			break
		}

		if _, ok := t.(json.Delim); ok {
			if dec.More() {
				continue
			}
			break
		}

		key, ok := t.(string)
		if !ok {
			continue
		}

		switch key {
		case "err_msg":
			t, err = dec.Token()
			if err != nil {
				s.l.Printf("[%s] dec err_msg err: %v", ch, err)
				break
			}
			if v, ok := t.(string); ok {
				if strings.ToLower(v) != "ok" {
					s.l.Printf("[%s] got err_msg: %v", ch, v)
				}
			} else {
				s.l.Printf("[%s] got err_msg wrong type", ch)
			}

		case "ads":
			if _, err = dec.Token(); err != nil {
				s.l.Printf("[%s] begin dec ads err: %v", ch, err)
				break
			}
			for dec.More() {
				var raw raw_ad.RawAdObj
				if err := dec.Decode(&raw); err != nil {
					s.l.Printf("[%s] json decode err: %v", ch, err)
					if errCnt > 50 {
						s.l.Printf("[%s] too many errors", ch)
						break
					}
					errCnt++
					continue
				}

				if rawAd := s.PreProcess(&raw); rawAd != nil {
					if err := rawAd.AddToDnf(s.dnfHandler); err != nil {
						s.l.Println("[DNF] add docId:", rawAd.UniqId, ", err: ", err)
					} else {
						offerCnt++
						s.l.Printf("[%s] add raw: %s", ch, rawAd.Id)
					}
				}
			}
			if _, err = dec.Token(); err != nil {
				s.l.Printf("[%s] end dec ads err: %v", ch, err)
				break
			}

		default:
			s.l.Printf("[%s] un-handle key: %s", ch, key)
		}
	}

	return
}

func (s *Service) doUpdateS3TmpChannels() {
	if s.conf.OfferBucket == "" || s.conf.OfferKey == "" {
		s.l.Println("[S3] invald conf bucket: ",
			s.conf.OfferBucket, ", key: ", s.conf.OfferKey)
		return
	}

	prefix := "tmp_test/" + s.conf.OfferKey
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	keys, err := store.ListFolder(s.conf.S3Region,
		s.conf.DownPath, s.conf.OfferBucket, prefix)
	if err != nil {
		s.l.Println("[S3] list ", prefix, " folder error: ", err)
		return
	}

	// prefix是tmp_test/offer_data/这样的格式
	// keys的格式是tmp_test/offer_data/(channel)/\\d.gz的格式
	for _, key := range keys {
		ch := getChannel(key)
		if len(ch) == 0 {
			s.l.Println("[S3] list key found no channel: ", key)
			continue
		}
		var cnt int
		s.doUpdateS3WithPrefix(ch, &cnt, prefix, true)
		s.l.Println("update tmp test channel: ", ch, " ok, offer cnt: ", cnt)
	}
}

func (s *Service) doUpdateS3(ch string, offerCnt *int) {
	if s.conf.OfferBucket == "" || s.conf.OfferKey == "" {
		s.l.Println("[S3] invald conf bucket: ",
			s.conf.OfferBucket, ", key: ", s.conf.OfferKey)
		return
	}
	prefix := s.conf.OfferKey
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	s.doUpdateS3WithPrefix(ch, offerCnt, prefix, false)
}

func (s *Service) doUpdateS3WithPrefix(ch string, offerCnt *int, prefix string, isTemp bool) {
	// 1. 获取s3上该渠道的所有文件
	prefix += ch + "/"
	keys, err := store.ListFolder(s.conf.S3Region,
		s.conf.DownPath, s.conf.OfferBucket, prefix)
	if err != nil {
		s.l.Println("[S3] list ", ch, " folder error: ", err)
		return
	}
	// 2. 下载数据文件存储到内存中
	for _, k := range keys {
		data, err := store.DownloadFile(s.conf.S3Region, s.conf.DownPath,
			s.conf.OfferBucket, k)
		if err != nil {
			s.l.Println("[S3] download ", k, " error: ", err)
			continue
		}

		ads := make([]*raw_ad.RawAdObj, 0, 100)
		if err := json.NewDecoder(bytes.NewReader(data)).Decode(&ads); err != nil {
			s.l.Println("[S3] json decode ", k, " error: ", err)
			continue
		}
		for _, ad := range ads {
			if rawAd := s.PreProcess(ad); rawAd != nil {
				rawAd.IsT = isTemp
				if err := rawAd.AddToDnf(s.dnfHandler); err != nil {
					s.l.Println("[DNF] add docId:", rawAd.UniqId, ", err: ", err)
				} else {
					*offerCnt++
					s.l.Printf("[%s] add raw: %s, total: %d", ch, rawAd.Id, *offerCnt)
				}
			}
		}
	}
}

// 对每个offer进行预处理
func (s *Service) PreProcess(raw *raw_ad.RawAdObj) *raw_ad.RawAdObj {
	// 不同channel的特殊处理
	switch raw.Channel {
	case "ym":
		raw.AttachArgs = append(raw.AttachArgs, "aff_id="+s.conf.YeahmobiAffId)
	case "nym":
		raw.AttachArgs = append(raw.AttachArgs, "aff_id="+s.conf.YeahmobiNonPreloadAffId)
	case "tym":
		raw.AttachArgs = append(raw.AttachArgs, "aff_id="+s.conf.YeahmobiThirdPartAffId)
	case "iym":
		raw.AttachArgs = append(raw.AttachArgs, "aff_id="+s.conf.YeahmobiIrsAffId)
	case "inym":
		raw.AttachArgs = append(raw.AttachArgs, "aff_id="+s.conf.YeahmobiInmobiAffId)
	case "stpym":
		raw.AttachArgs = append(raw.AttachArgs, "aff_id="+s.conf.YeahmobiStartAppAffId)
	case "jpym":
		raw.AttachArgs = append(raw.AttachArgs, "aff_id="+s.conf.YeahmobiJapanAffId)
	case "jpym2":
		raw.AttachArgs = append(raw.AttachArgs, "aff_id="+s.conf.YeahmobiJapan2AffId)
	case "vym":
		raw.AttachArgs = append(raw.AttachArgs, "aff_id="+s.conf.YeahmobiVideoAffId)
	case "v1ym":
		raw.AttachArgs = append(raw.AttachArgs, "aff_id="+s.conf.YeahmobiVideo1AffId)
	case "n1ym":
		raw.AttachArgs = append(raw.AttachArgs, "aff_id="+s.conf.YeahmobiN1AffId)
	case "n2ym":
		raw.AttachArgs = append(raw.AttachArgs, "aff_id="+s.conf.YeahmobiN2AffId)
	case "n3ym":
		raw.AttachArgs = append(raw.AttachArgs, "aff_id="+s.conf.YeahmobiN3AffId)
	case "sym":
		raw.AttachArgs = append(raw.AttachArgs, "aff_id="+s.conf.YeahmobiSAffId)
	}

	// 单价太低的不入库
	if raw.Payout <= 0.1 {
		s.l.Println("offer price less 0.1$, id:  ", raw.UniqId, " payout: ", raw.Payout)
		return nil
	}

	if raw.Channel == "wrapChannel" { // 离线offer
		if ids := strings.Split(raw.Id, "_"); len(ids) == 2 {
			raw.Channel = ids[0]
			raw.UniqId = raw.Id
			raw.Id = ids[1]
		} else {
			s.l.Println("can't parse offline offer, id:", raw.Id, " UniqId: ", raw.UniqId)
			return nil
		}
	}

	// 设置CPL流量比率
	if raw.PayoutType == "CPL" {
		raw.PreRate = 0.0
	}

	return raw
}
