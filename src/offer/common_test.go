package offer

import (
	"strings"
	"testing"

	"raw_ad"
)

func TestGetPkgNameFromFinalURL(t *testing.T) {
	check := func(url string, c string) {
		if url != c {
			t.Error("check GetPkgNameFromFinalURL err, url: ", url, " c: ", c)
		}
	}

	check(GetPkgNameFromFinalURL("com/us/app/id12345?mt=8"), "12345")
	check(GetPkgNameFromFinalURL("com/us/app/id12345"), "12345")
	check(GetPkgNameFromFinalURL("/id12345?mt=8"), "12345")
	check(GetPkgNameFromFinalURL("com/store/details?id=com.api.www"), "com.api.www")
	check(GetPkgNameFromFinalURL("com/store/details?ad&id=com.api.www"), "com.api.www")
	check(GetPkgNameFromFinalURL("com/store/details?id=com.api.www&a=1"), "com.api.www")
	check(GetPkgNameFromFinalURL("id=com.api.www"), "com.api.www")
	check(GetPkgNameFromFinalURL("id=com.api.www&a=2"), "com.api.www")
}

func setCreativeManagerCrawlUrl(url string) {
	CreativeManagerCrawl = url
}

func Test_GetCreativesFromCrawl(t *testing.T) {
	check := func(err error, c string) {
		if !strings.Contains(err.Error(), c) {
			t.Error("check GetCreativesFromCrawl err: ", err, " c: ", c)
		}
	}

	setCreativeManagerCrawlUrl("http://54.255.165.222:10000/query?")

	_, err := GetCreativesFromCrawl("", "com.api", "iOS")
	check(err, "country or pkgName or platform is nil")
	_, err = GetCreativesFromCrawl("US", "", "iOS")
	check(err, "country or pkgName or platform is nil")
	_, err = GetCreativesFromCrawl("UA", "com.api", "")
	check(err, "country or pkgName or platform is nil")
	_, err = GetCreativesFromCrawl("US", "com.api", "Android")
	check(err, "request failed")
}

func Test_GetCrawlCreative(t *testing.T) {
	check := func(img *raw_ad.Img, err error, w, h int, url, e string) {
		if img != nil {
			if img.Width != w || img.Height != h || img.Url != url {
				t.Error("check GetCrawlCreative info err")
			}
		}
		if err != nil && !strings.Contains(err.Error(), e) {
			t.Error("check GetCrawlCreative err: ", err)
		}
	}

	img, err := GetCrawlCreative(1, 2, "www.abc.com", "creative-0")
	check(img, err, 1, 2, "www.abc.com", "")
	img, err = GetCrawlCreative(0, 2, "www.abc.com", "creative-1")
	check(img, err, 1, 2, "www.abc.com", "parameter err")
	img, err = GetCrawlCreative(0, 0, "www.abc.com", "creative-2")
	check(img, err, 1, 2, "www.abc.com", "parameter err")
}
