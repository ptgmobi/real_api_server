package main

import (
	_ "net/http/pprof"

	"github.com/brg-liuwei/gotools"

	"aes"
	"real_api"
	"retrieval"
	"status"
	"util"
)

type Conf struct {
	RetrievalConf retrieval.Conf `json:"retrieval_config"`
	UtilConf      util.Conf      `json:"util_config"`
	AesConf       aes.Conf       `json:"aes_config"`
	RealApi       real_api.Conf  `json:"real_api_conf"`
}

var conf Conf

func startRetrievalService(cf *retrieval.Conf) {
	retrievalService, err := retrieval.NewService(cf)
	if err != nil {
		panic(err)
	}
	retrievalService.Serve()
}
func main() {
	if err := gotools.DecodeJsonFile("conf/offer.conf", &conf); err != nil {
		panic(err)
	}

	aes.Init(&conf.AesConf)
	util.Init(&conf.UtilConf)
	real_api.Init(&conf.RealApi)

	go startRetrievalService(&conf.RetrievalConf)

	status.Serve()
}
