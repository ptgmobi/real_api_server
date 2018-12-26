package real_api

import (
	"http_context"
	"raw_ad"
	"real_api/huicheng"
)

type Conf struct {
	HuichengApi string `json:"huicheng_api"`
}

type RealApi struct {
	conf *Conf
}

var global *RealApi

func Init(conf *Conf) {
	global = &RealApi{
		conf: conf,
	}

}

func Request(ctx *http_context.Context) (*raw_ad.RawAdObj, error) {
	return global.request(ctx)
}

func (s *RealApi) request(ctx *http_context.Context) (*raw_ad.RawAdObj, error) {
	return huicheng.Request(s.conf.HuichengApi, 1000, ctx)
}
