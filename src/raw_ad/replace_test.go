package raw_ad

import (
	"fmt"

	"http_context"
)

func ExampleReplace() {
	imgs := []Img{
		{Width: 100, Height: 100, Url: "this-is-first-img", Lang: "EN"},
		{Width: 300, Height: 250, Url: "this-is-second-img", Lang: "ALL"},
		{Width: 1080, Height: 680, Url: "this-is-third-img", Lang: "EN"},
		{Width: 300, Height: 250, Url: "this-is-fourth-img", Lang: "EN"},
	}

	icons := []Img{
		{Width: 100, Height: 100, Url: "icon1", Lang: "EN"},
		{Width: 300, Height: 300, Url: "icon2", Lang: "ALL"},
	}

	templ := []byte("this is {$icon} = {$icon}, {$img_300x250} = {$img_300x250}")
	raw := NewRawAdObj()
	raw.AddCreatives(imgs)

	m := make(map[string]string)
	m["{$icon}"] = icons[1].Url
	repl := raw.ReplaceTpl(nil, templ, m, &http_context.Context{
		Country: "US",
		Lang:    "EN",
		ImgRule: 1,
	})
	fmt.Println(string(repl))
	m["{$icon}"] = icons[0].Url
	repl = raw.ReplaceTpl(nil, templ, m, &http_context.Context{
		Country: "US",
		Lang:    "CH",
		ImgRule: 1,
	})
	fmt.Println(string(repl))

	// Output:
	// this is icon2 = {$icon}, this-is-fourth-img = {$img_300x250}
	// this is icon1 = {$icon}, this-is-second-img = {$img_300x250}

}
