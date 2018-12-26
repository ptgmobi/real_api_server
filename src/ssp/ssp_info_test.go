package ssp

import (
	"testing"
)

func TestRegexpOK(t *testing.T) {
	tpls := []string{
		"<html><body><img src=\"{$img_300x250}\"/></body></html>",
		"<html><body><img src=\"{$img_300x250}\"/><img2 src=\"{$img_320x200}\"/></body></html>",
	}
	for i, tpl := range tpls {
		w, h := matchTplSize([]byte(tpl))
		if w != 300 || h != 250 {
			t.Error("tpl[", i, "] w error: ", w, ", h error: ", h)
		}
	}
}

func TestRegexpFail(t *testing.T) {
	tpls := []string{
		"<html><body><img src=\"{$img_300x250a}\"/></body></html>",
		"<html><body><img src=\"{$img_300x}\"/></body></html>",
		"<html><body><img src=\"{$img_250}\"/></body></html>",
		"<html><body><img src=\"{$img_0x-250}\"/></body></html>",
		"<html><body><img src=\"{$img_-300x+250}\"/></body></html>",
		"<html><body><img src=\"{$img}\"/></body></html>",
	}
	for _, tpl := range tpls {
		w, h := matchTplSize([]byte(tpl))
		if w != 0 || h != 0 {
			t.Error("w error: ", w, ", h error: ", h)
		}
	}
}
