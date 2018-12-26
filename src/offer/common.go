package offer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"raw_ad"
)

var searchReg *regexp.Regexp
var checkPkgReg *regexp.Regexp

var imageSuffixReg *regexp.Regexp
var videoSuffixReg *regexp.Regexp

func init() {
	searchReg = regexp.MustCompile("[\\s!,\\-&:]")
	checkPkgReg = regexp.MustCompile(`^(\d+)$|^(\w+\.)\w+$`)

	imageSuffixReg = regexp.MustCompile(`\.(jpeg|jpg|gif|png)$`)
	videoSuffixReg = regexp.MustCompile(`\.(mp4)$`)
}

func SearchPreProcess(content string) string {
	return searchReg.ReplaceAllLiteralString(strings.ToLower(content), "")
}

var GpDefIcon string = "https://cdn.image.zcoup.com/default/icon/100/100/google/icon.png"
var AsDefIcon string = "https://cdn.image.zcoup.com/default/icon/100/100/apple/icon.png"

var DefDesc = []string{
	"The best app you have ever used. Let's try it now!",
	"The most popular app among all your friends. Let's join them today!",
}

var GooglePlayIcon = raw_ad.Img{
	Width:  100,
	Height: 100,
	Url:    GpDefIcon,
	Lang:   "ALL",
}

var AppleStoreIcon = raw_ad.Img{
	Width:  100,
	Height: 100,
	Url:    AsDefIcon,
	Lang:   "ALL",
}

var CreativeManagerMetaUrl = "http://image.zcoup.com/cmgr/meta?"
var CreativeManagerUploadImgUrl = "http://image.zcoup.com/cmgr/uploadImg?"

var CreativeManagerCrawl = "http://10.17.5.52:10000/query?" // XXX

var pkgNameReg = regexp.MustCompile("/id(\\d+)|id=([\\w+.]+)") // "/id1235" "id=com.abc"

var iosVer []string = []string{
	"5.0",
	"5.1",
	"5.2",
	"5.3",
	"5.4",
	"6.0",
	"6.1",
	"6.2",
	"6.3",
	"6.4",
	"7.0",
	"7.1",
	"7.2",
	"7.3",
	"7.4",
	"8.0",
	"8.1",
	"8.2",
	"8.3",
	"8.4",
	"9.0",
	"9.1",
	"9.2",
	"9.3",
	"9.4",
	"10.0",
	"10.1",
	"10.2",
	"10.3",
	"10.4",
	"10.5",
	"10.6",
	"10.7",
	"10.8",
	"10.9",
}

var androidVer []string = []string{
	"4.0",
	"4.0.1",
	"4.0.2",
	"4.0.3",
	"4.0.4",
	"4.1",
	"4.1.1",
	"4.1.2",
	"4.1.3",
	"4.1.4",
	"4.2",
	"4.2.1",
	"4.2.2",
	"4.2.3",
	"4.2.4",
	"4.3",
	"4.3.1",
	"4.3.2",
	"4.3.3",
	"4.3.4",
	"4.4",
	"4.4.1",
	"4.4.2",
	"4.4.3",
	"4.4.4",
	"5.0",
	"5.0.1",
	"5.0.2",
	"5.1.1",
	"5.1.2",
	"6.0",
	"6.0.1",
	"6.0.2",
	"6.1.1",
	"6.1.2",
	"6.2.1",
	"6.2.2",
	"7.0",
	"7.0.1",
	"7.0.2",
	"7.1.1",
	"7.1.2",
	"7.2.1",
	"7.2.2",
}

var apiLevelToVer = map[string]string{
	"1":  "1.0",
	"2":  "1.1",
	"3":  "1.5",
	"4":  "1.6",
	"5":  "2.0",
	"6":  "2.0.1",
	"7":  "2.1",
	"8":  "2.2",
	"9":  "2.3",
	"10": "2.3.3",
	"11": "3.0",
	"12": "3.1",
	"13": "3.2",
	"14": "4.0",
	"15": "4.0.3",
	"16": "4.1",
	"17": "4.2",
	"18": "4.3",
	"19": "4.4",
	"20": "4.4",
	"21": "5.0",
	"22": "5.1.1",
	"23": "6.0",
	"24": "7.0",
}

func GetAndroidVerByLevel(level string) string {
	if v, ok := apiLevelToVer[level]; ok {
		return v
	}
	return ""
}

// 根据url判断素材类型
func GetCreativeUrlType(url string) string {
	if imageSuffixReg.MatchString(url) {
		return "image"
	} else if videoSuffixReg.MatchString(url) {
		return "video"
	}
	return "unknow"
}

type CrawlRestfulResult struct {
	ErrNo  int     `json:"err_no"`
	ErrMsg string  `json:"err_msg"`
	Metas  []*Meta `json:"meta"`
}

type Meta struct {
	PkgName       string   `json:"pkg_name"`
	Platform      string   `json:"platform"`
	Country       string   `json:"country"`
	Lang          string   `json:"lang"`
	Title         string   `json:"title"`
	Category      string   `json:"category"`
	Desc          string   `json:"desc"`
	Type          string   `json:"type"`
	Score         string   `json:"score"`
	Developer     string   `json:"developer"`
	Company       string   `json:"company"`
	RatingCount   string   `json:"rating_count"`
	DatePublished string   `json:"date_published"`
	NumDownload   string   `json:"num_download"`
	SoftVersion   string   `json:"soft_version"`
	Os            string   `json:"os"`
	ContentRating string   `json:"content_rating"`
	Permission    string   `json:"permission"`
	Images        []*Image `json:"images"`
	Icons         []*Icon  `json:"icons"`
}

type Icon struct {
	Url    string `json:"url"`
	Width  int    `width`
	Height int    `json:"height"`
	CdnUrl string `json:"cdn_url"`
}

type Image struct {
	ImageId    int    `json:"-"`
	CreativeId string `json:"creative_id"`
	Url        string `json:"url"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	CdnUrl     string `json:"cdn_url"`
	AppInfoId  int    `json:"-"`
}

var h5TagReg = regexp.MustCompile(`</?(p|img|html|a|div|label|br)\s*/?>`)

func ReplaceH5tag(src string) string {
	return h5TagReg.ReplaceAllString(src, "")
}

func GetVerRange(os string, limit ...string) (versions []string) {
	var verList []string
	if os == "iOS" {
		verList = iosVer
	} else if os == "Android" {
		verList = androidVer
	} else {
		return
	}

	var min, max string
	switch len(limit) {
	case 0:
		return
	case 1:
		min = limit[0]
		max = verList[len(verList)-1]
	default:
		min = limit[0]
		max = limit[1]
	}

	var i int
	for i = 0; i != len(verList); i++ {
		if verList[i] == min {
			break
		}
	}

	if i == len(verList) {
		fmt.Println("un-match min os[", os, "] version: ", min)
		return
	}

	for ; i != len(verList); i++ {
		versions = append(versions, verList[i])
		if verList[i] == max {
			return
		}
	}
	return
}

func GetAppInfoFromCrawl(pkg, platform, country string) []*Meta {
	uri := fmt.Sprintf("%spkg=%s&platform=%s&country=%s&all=true",
		CreativeManagerCrawl, pkg, platform, country)
	client := http.Client{
		Timeout: time.Second * 1,
	}
	resp, err := client.Get(uri)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var item CrawlRestfulResult
	if err := json.Unmarshal(body, &item); err != nil {
		return nil
	}
	if item.ErrNo != 200 || item.ErrMsg != "OK" {
		return nil
	}
	if len(item.Metas) == 0 {
		return nil
	}

	return item.Metas
}

func GetCreativesFromCrawlAll(country, pkgName, platform string) ([]raw_ad.Img, string, string, error) {
	if len(country) == 0 || len(pkgName) == 0 || len(platform) == 0 {
		return nil, "", "", errors.New("country or pkgName or platform is nil")
	}
	var title, desc string
	uri := CreativeManagerCrawl + "pkg=" + pkgName + "&platform=" + platform + "&country=" + country + "&all=true"
	resp, err := http.Get(uri)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, "", "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", err
	}

	var item CrawlRestfulResult
	if err := json.Unmarshal([]byte(body), &item); err != nil {
		return nil, "", "", err
	}
	if item.ErrNo != 200 || item.ErrMsg != "OK" {
		return nil, "", "", errors.New("request failed, msg: " + item.ErrMsg + " no: " + strconv.Itoa(item.ErrNo))
	}
	if len(item.Metas) == 0 {
		return nil, "", "", errors.New("request failed, no creative!")
	}

	imgs := make([]raw_ad.Img, 0, 16)
	for _, meta := range item.Metas {
		if len(meta.Title) > 0 && len(meta.Desc) > 0 {
			title = meta.Title
			desc = meta.Desc
		}
		// 如果语言是英语就用英语
		if strings.ToLower(meta.Lang) == "en" {
			title = meta.Title
			desc = meta.Desc
		}
		for _, image := range meta.Images {
			if img, err := GetCrawlCreative(image.Width, image.Height, image.CdnUrl, ""); img != nil {
				imgs = append(imgs, *img)
			} else {
				fmt.Println("GetCrawlCreative err: ", err)
				continue
			}
		}
	}

	if len(imgs) == 0 && len(title) == 0 && len(desc) == 0 {
		return nil, "", "", errors.New("can't get app info from app wral")
	}
	return imgs, title, desc, nil
}

func GetCreativesFromCrawl(country, pkgName, platform string) ([]raw_ad.Img, error) {
	if len(country) == 0 || len(pkgName) == 0 || len(platform) == 0 {
		return nil, errors.New("country or pkgName or platform is nil")
	}
	uri := CreativeManagerCrawl + "pkg=" + pkgName + "&platform=" + platform + "&country=" + country + "&image=true"
	resp, err := http.Get(uri)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var item CrawlRestfulResult
	if err := json.Unmarshal([]byte(body), &item); err != nil {
		return nil, err
	}
	if item.ErrNo != 200 || item.ErrMsg != "OK" {
		return nil, errors.New("request failed, msg: " + item.ErrMsg + " no: " + strconv.Itoa(item.ErrNo))
	}
	if len(item.Metas) == 0 {
		return nil, errors.New("request failed, no creative!")
	}

	imgs := make([]raw_ad.Img, 0, 16)
	for _, meta := range item.Metas {
		for _, image := range meta.Images {
			if img, err := GetCrawlCreative(image.Width, image.Height, image.CdnUrl, ""); img != nil {
				imgs = append(imgs, *img)
			} else {
				fmt.Println("GetCrawlCreative err: ", err)
				continue
			}
		}
	}

	if len(imgs) > 0 {
		return imgs, nil
	} else {
		return nil, errors.New("can't get imgs from meta's url")
	}
}

func GetCrawlCreative(w, h int, url, creativeId string) (*raw_ad.Img, error) {
	if w <= 0 || h <= 0 || len(url) == 0 {
		return nil, errors.New("GetCrawlCreative parameter err, url: " + url +
			" w=" + strconv.Itoa(w) + " h=" + strconv.Itoa(h))
	} else {
		return &raw_ad.Img{
			Id:     creativeId,
			Width:  w,
			Height: h,
			Url:    url,
			Lang:   "ALL",
		}, nil
	}
}

func GetPkgNameFromFinalURL(finalURL string) string {
	// https://itunes.apple.com/us/app/uber/id368677368?mt=8 // id后面就是包名
	// https://play.google.com/store/apps/details?id=com.amazon.mShop.android.shopping&hl=en // id后面就是包名
	var pkgName string
	pkgSlice := pkgNameReg.FindStringSubmatch(finalURL)
	if len(pkgSlice) == 3 {
		if len(pkgSlice[1]) > 0 {
			pkgName = pkgSlice[1]
		} else if len(pkgSlice[2]) > 0 {
			pkgName = pkgSlice[2]
		}
	}
	return pkgName
}

// 随机返回一个描述
func GetDefaultDesc() string {
	r := rand.Intn(len(DefDesc))
	return DefDesc[r]
}

func CountryCodeThreeToTwo(code string) string {
	if c, ok := countryCode[code]; ok {
		return c
	}
	return ""
}

func CheckPkgName(pkg string) bool {
	return checkPkgReg.MatchString(pkg)
}

var countryCode = map[string]string{
	"AND": "AD", // 安道尔
	"ARE": "AE", // 阿联酋
	"AFG": "AF", // 阿富汗
	"ATG": "AG", // 安提瓜和巴布达
	"AIA": "AI", // 安圭拉
	"ALB": "AL", // 阿尔巴尼亚
	"ARM": "AM", // 亚美尼亚
	"AGO": "AO", // 安哥拉
	"ATA": "AQ", // 南极洲
	"ARG": "AR", // 阿根廷
	"ASM": "AS", // 美属萨摩亚
	"AUT": "AT", // 奥地利
	"AUS": "AU", // 澳大利亚
	"ABW": "AW", // 阿鲁巴
	"ALA": "AX", // 奥兰群岛
	"AZE": "AZ", // 阿塞拜疆
	"BIH": "BA", // 波黑
	"BRB": "BB", // 巴巴多斯
	"BGD": "BD", // 孟加拉国
	"BEL": "BE", // 比利时
	"BFA": "BF", // 布基纳法索
	"BGR": "BG", // 保加利亚
	"BHR": "BH", // 巴林
	"BDI": "BI", // 布隆迪
	"BEN": "BJ", // 贝宁
	"BLM": "BL", // 圣巴泰勒米岛
	"BMU": "BM", // 百慕大
	"BRN": "BN", // 文莱
	"BOL": "BO", // 玻利维亚
	"BES": "BQ", // 荷兰加勒比区
	"BRA": "BR", // 巴西
	"BHS": "BS", // 巴哈马
	"BTN": "BT", // 不丹
	"BVT": "BV", // 布韦岛
	"BWA": "BW", // 博茨瓦纳
	"BLR": "BY", // 白俄罗斯
	"BLZ": "BZ", // 伯利兹
	"CAN": "CA", // 加拿大
	"CCK": "CC", // 科科斯群岛
	"COD": "CD", // 刚果（金）
	"CAF": "CF", // 中非
	"COG": "CG", // 刚果（布）
	"CHE": "CH", // 瑞士
	"CIV": "CI", // 科特迪瓦
	"COK": "CK", // 库克群岛
	"CHL": "CL", // 智利
	"CMR": "CM", // 喀麦隆
	"CHN": "CN", // 中国；
	"COL": "CO", // 哥伦比亚
	"CRI": "CR", // 哥斯达黎加
	"CUB": "CU", // 古巴
	"CPV": "CV", // 佛得角
	"CUW": "CW", // 库拉索
	"CXR": "CX", // 圣诞岛
	"CYP": "CY", // 塞浦路斯
	"CZE": "CZ", // 捷克
	"DEU": "DE", // 德国
	"DJI": "DJ", // 吉布提
	"DNK": "DK", // 丹麦
	"DMA": "DM", // 多米尼克
	"DOM": "DO", // 多米尼加
	"DZA": "DZ", // 阿尔及利亚
	"ECU": "EC", // 厄瓜多尔
	"EST": "EE", // 爱沙尼亚
	"EGY": "EG", // 埃及
	"ESH": "EH", // 西撒哈拉
	"ERI": "ER", // 厄立特里亚
	"ESP": "ES", // 西班牙
	"ETH": "ET", // 埃塞俄比亚
	"FIN": "FI", // 芬兰
	"FJI": "FJ", // 斐济群岛
	"FLK": "FK", // 马尔维纳斯群岛（福克兰）
	"FSM": "FM", // 密克罗尼西亚联邦
	"FRO": "FO", // 法罗群岛
	"FRA": "FR", // 法国
	"GAB": "GA", // 加蓬
	"GBR": "GB", // 英国
	"GRD": "GD", // 格林纳达
	"GEO": "GE", // 格鲁吉亚
	"GUF": "GF", // 法属圭亚那
	"GGY": "GG", // 根西岛
	"GHA": "GH", // 加纳
	"GIB": "GI", // 直布罗陀
	"GRL": "GL", // 格陵兰
	"GMB": "GM", // 冈比亚
	"GIN": "GN", // 几内亚
	"GLP": "GP", // 瓜德罗普
	"GNQ": "GQ", // 赤道几内亚
	"GRC": "GR", // 希腊
	"SGS": "GS", // 南乔治亚岛和南桑威奇群岛
	"GTM": "GT", // 危地马拉
	"GUM": "GU", // 关岛
	"GNB": "GW", // 几内亚比绍
	"GUY": "GY", // 圭亚那
	"HKG": "HK", // 香港
	"HMD": "HM", // 赫德岛和麦克唐纳群岛
	"HND": "HN", // 洪都拉斯
	"HRV": "HR", // 克罗地亚
	"HTI": "HT", // 海地
	"HUN": "HU", // 匈牙利
	"IDN": "ID", // 印尼
	"IRL": "IE", // 爱尔兰
	"ISR": "IL", // 以色列
	"IMN": "IM", // 马恩岛
	"IND": "IN", // 印度
	"IOT": "IO", // 英属印度洋领地
	"IRQ": "IQ", // 伊拉克
	"IRN": "IR", // 伊朗
	"ISL": "IS", // 冰岛
	"ITA": "IT", // 意大利
	"JEY": "JE", // 泽西岛
	"JAM": "JM", // 牙买加
	"JOR": "JO", // 约旦
	"JPN": "JP", // 日本
	"KEN": "KE", // 肯尼亚
	"KGZ": "KG", // 吉尔吉斯斯坦
	"KHM": "KH", // 柬埔寨
	"KIR": "KI", // 基里巴斯
	"COM": "KM", // 科摩罗
	"KNA": "KN", // 圣基茨和尼维斯
	"PRK": "KP", // 朝鲜；
	"KOR": "KR", // 韩国；
	"KWT": "KW", // 科威特
	"CYM": "KY", // 开曼群岛
	"KAZ": "KZ", // 哈萨克斯坦
	"LAO": "LA", // 老挝
	"LBN": "LB", // 黎巴嫩
	"LCA": "LC", // 圣卢西亚
	"LIE": "LI", // 列支敦士登
	"LKA": "LK", // 斯里兰卡
	"LBR": "LR", // 利比里亚
	"LSO": "LS", // 莱索托
	"LTU": "LT", // 立陶宛
	"LUX": "LU", // 卢森堡
	"LVA": "LV", // 拉脱维亚
	"LBY": "LY", // 利比亚
	"MAR": "MA", // 摩洛哥
	"MCO": "MC", // 摩纳哥
	"MDA": "MD", // 摩尔多瓦
	"MNE": "ME", // 黑山
	"MAF": "MF", // 法属圣马丁
	"MDG": "MG", // 马达加斯加
	"MHL": "MH", // 马绍尔群岛
	"MKD": "MK", // 马其顿
	"MLI": "ML", // 马里
	"MMR": "MM", // 缅甸
	"MNG": "MN", // 蒙古国；蒙古
	"MAC": "MO", // 澳门
	"MNP": "MP", // 北马里亚纳群岛
	"MTQ": "MQ", // 马提尼克
	"MRT": "MR", // 毛里塔尼亚
	"MSR": "MS", // 蒙塞拉特岛
	"MLT": "MT", // 马耳他
	"MUS": "MU", // 毛里求斯
	"MDV": "MV", // 马尔代夫
	"MWI": "MW", // 马拉维
	"MEX": "MX", // 墨西哥
	"MYS": "MY", // 马来西亚
	"MOZ": "MZ", // 莫桑比克
	"NAM": "NA", // 纳米比亚
	"NCL": "NC", // 新喀里多尼亚
	"NER": "NE", // 尼日尔
	"NFK": "NF", // 诺福克岛
	"NGA": "NG", // 尼日利亚
	"NIC": "NI", // 尼加拉瓜
	"NKR": "NK", // 纳戈尔诺-卡拉巴赫
	"NLD": "NL", // 荷兰
	"NOR": "NO", // 挪威
	"NPL": "NP", // 尼泊尔
	"NRU": "NR", // 瑙鲁
	"NIU": "NU", // 纽埃
	"NZL": "NZ", // 新西兰
	"OMN": "OM", // 阿曼
	"PAN": "PA", // 巴拿马
	"PER": "PE", // 秘鲁
	"PYF": "PF", // 法属波利尼西亚
	"PNG": "PG", // 巴布亚新几内亚
	"PHL": "PH", // 菲律宾
	"PAK": "PK", // 巴基斯坦
	"POL": "PL", // 波兰
	"SPM": "PM", // 圣皮埃尔和密克隆
	"PCN": "PN", // 皮特凯恩群岛
	"PRI": "PR", // 波多黎各
	"PSE": "PS", // 巴勒斯坦
	"PRT": "PT", // 葡萄牙
	"PLW": "PW", // 帕劳
	"PRY": "PY", // 巴拉圭
	"QAT": "QA", // 卡塔尔
	"REU": "RE", // 留尼汪
	"ROU": "RO", // 罗马尼亚
	"SRB": "RS", // 塞尔维亚
	"RUS": "RU", // 俄罗斯
	"RWA": "RW", // 卢旺达
	"SAU": "SA", // 沙特阿拉伯
	"SLB": "SB", // 所罗门群岛
	"SYC": "SC", // 塞舌尔
	"SDN": "SD", // 苏丹
	"SWE": "SE", // 瑞典
	"SGP": "SG", // 新加坡
	"SHN": "SH", // 圣赫勒拿
	"SVN": "SI", // 斯洛文尼亚
	"SJM": "SJ", // 斯瓦尔巴群岛和扬马延岛
	"SVK": "SK", // 斯洛伐克
	"SLE": "SL", // 塞拉利昂
	"SMR": "SM", // 圣马力诺
	"SEN": "SN", // 塞内加尔
	"SOM": "SO", // 索马里
	"SUR": "SR", // 苏里南
	"SSD": "SS", // 南苏丹
	"STP": "ST", // 圣多美和普林西比
	"SLV": "SV", // 萨尔瓦多
	"SXM": "SX", // 荷属圣马丁
	"SYR": "SY", // 叙利亚
	"SWZ": "SZ", // 斯威士兰
	"TCA": "TC", // 特克斯和凯科斯群岛
	"TCD": "TD", // 乍得
	"ATF": "TF", // 法属南部领地
	"TGO": "TG", // 多哥
	"THA": "TH", // 泰国
	"TJK": "TJ", // 塔吉克斯坦
	"TKL": "TK", // 托克劳
	"TLS": "TL", // 东帝汶
	"TKM": "TM", // 土库曼斯坦
	"TUN": "TN", // 突尼斯
	"TON": "TO", // 汤加
	"TUR": "TR", // 土耳其
	"TTO": "TT", // 特立尼达和多巴哥
	"TUV": "TV", // 图瓦卢
	"TWN": "TW", // 台湾
	"TZA": "TZ", // 坦桑尼亚
	"UKR": "UA", // 乌克兰
	"UGA": "UG", // 乌干达
	"UMI": "UM", // 美国本土外小岛屿
	"USA": "US", // 美国
	"URY": "UY", // 乌拉圭
	"UZB": "UZ", // 乌兹别克斯坦
	"VAT": "VA", // 梵蒂冈
	"VCT": "VC", // 圣文森特和格林纳丁斯
	"VEN": "VE", // 委内瑞拉
	"VGB": "VG", // 英属维尔京群岛
	"VIR": "VI", // 美属维尔京群岛
	"VNM": "VN", // 越南
	"VUT": "VU", // 瓦努阿图
	"WLF": "WF", // 瓦利斯和富图纳
	"WSM": "WS", // 萨摩亚
	"YEM": "YE", // 也门
	"MYT": "YT", // 马约特
	"ZAF": "ZA", // 南非
	"ZMB": "ZM", // 赞比亚
	"ZWE": "ZW", // 津巴布韦
}

func CreativeToVideo(w, h int, ratio, mediaType, url string) *raw_ad.Video {
	if url == "" {
		return nil
	}

	return &raw_ad.Video{
		Ratio: ratio,
		W:     w,
		H:     h,
		Url:   url,
		Type:  mediaType,
		Lang:  "ALL",
	}
}
