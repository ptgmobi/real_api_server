package main

import (
	_ "net/http/pprof"
	"os"

	"github.com/brg-liuwei/gotools"

	"aes"
	"affiliate"
	"cache"
	"cpt"
	"creative"
	"dump"
	"ios_pmt"
	"rank"
	"real_api"
	"retrieval"
	"status"
	"subscription"
	"tpl"
	"update"
	"util"
)

type Conf struct {
	DumpAddr      string            `json:"dump_addr"`
	TplConf       tpl.Conf          `json:"template_config"`
	UpdateConf    update.Conf       `json:"update_config"`
	RetrievalConf retrieval.Conf    `json:"retrieval_config"`
	AffiliateConf affiliate.Conf    `json:"affiliate_config"`
	RankConf      rank.Conf         `json:"rank_config"`
	RedisConf     cache.Conf        `json:"cache_config"`
	UtilConf      util.Conf         `json:"util_config"`
	AesConf       aes.Conf          `json:"aes_config"`
	IOSPmtConf    ios_pmt.Conf      `json:"ios_pmt_config"`
	CptConf       cpt.Conf          `json:"cpt_config"`
	CreativeConf  creative.Conf     `json:"creative_config"`
	Subscription  subscription.Conf `json:"subscription"`
	RealApi       real_api.Conf     `json:"real_api_conf"`
}

var conf Conf

func startUpdateService(cf *update.Conf) {
	updateService, err := update.NewService(cf)
	if err != nil {
		panic(err)
	}
	updateService.Serve()
}

func startRetrievalService(cf *retrieval.Conf) {
	retrievalService, err := retrieval.NewService(cf)
	if err != nil {
		panic(err)
	}
	retrievalService.Serve()
}

func checkFileExist(path string) {
	if _, err := os.Stat(path); err != nil {
		panic(err)
	}
}

func checkTpl() {
	checkFileExist("tpl_files/TemplateL001.txt")
	checkFileExist("tpl_files/TemplateM002.txt")
	checkFileExist("tpl_files/TemplateS003.txt")
	checkFileExist("tpl_files/TemplateLHFJ009.txt")
	checkFileExist("tpl_files/TemplateLVF010.txt")
	checkFileExist("tpl_files/TemplateLHV011.txt")
	checkFileExist("tpl_files/TemplateLVV012.txt")
	checkFileExist("tpl_files/HtmlTpl.html")
	checkFileExist("tpl_files/MraidTpl.html")
}

func main() {
	if err := gotools.DecodeJsonFile("conf/offer.conf", &conf); err != nil {
		panic(err)
	}

	checkTpl()

	aes.Init(&conf.AesConf)
	util.Init(&conf.UtilConf)
	rank.Init(&conf.RankConf)
	cache.Init(&conf.RedisConf)
	cpt.Init(&conf.CptConf)
	ios_pmt.Init(&conf.IOSPmtConf)
	creative.Init(&conf.CreativeConf)
	subscription.Init(&conf.Subscription)
	real_api.Init(&conf.RealApi)

	aff := affiliate.NewAffiliate(&conf.AffiliateConf)
	if aff == nil {
		panic("aff nil")
	}
	go aff.Serve()

	go startRetrievalService(&conf.RetrievalConf)
	go startUpdateService(&conf.UpdateConf)
	go dump.Serve(conf.DumpAddr)

	go tpl.StartTlServer(&conf.TplConf)

	status.Serve()
}
