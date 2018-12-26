package raw_ad

import (
	"fmt"
	"testing"
)

func ExampleGenDnf() {
	raw := NewRawAdObj()
	raw.genDnf()
	fmt.Println(len(raw.Dnf))

	raw.AddTarget("country", "US", false)
	raw.genDnf()
	fmt.Println(raw.Dnf)

	raw.AddTarget("country", "CN", false)
	raw.AddTarget("country", "IN", false)
	raw.genDnf()
	fmt.Println(raw.Dnf)

	raw.AddTarget("lang", "FR", false)
	raw.genDnf()
	fmt.Println(raw.Dnf)

	raw.AddTarget("lang", "FR", false)
	raw.genDnf()
	fmt.Println(raw.Dnf)

	raw.AddTarget("lang", "JA", false)
	raw.genDnf()
	fmt.Println(raw.Dnf)

	raw.AddTarget("country", "RS", true)
	raw.genDnf()
	fmt.Println(raw.Dnf)

	raw.AddTarget("country", "AR", true)
	raw.genDnf()
	fmt.Println(raw.Dnf)

	// Output:
	// 0
	// ( country not in { US } )
	// ( country not in { US,CN,IN } )
	// ( country not in { US,CN,IN } and lang not in { FR } )
	// ( country not in { US,CN,IN } and lang not in { FR } )
	// ( country not in { US,CN,IN } and lang not in { FR,JA } )
	// ( lang not in { FR,JA } and country in { RS } )
	// ( lang not in { FR,JA } and country in { RS,AR } )
}

func TestImgMatchFamily(t *testing.T) {
	check := func(ok bool, msg string) {
		if !ok {
			t.Error(msg)
		}
	}
	img := Img{
		Width:  300,
		Height: 200,
	}

	check(img.Match(300, 200), "CASE 1")
	check(!img.Match(300, 201), "CASE 2")

	check(img.AbsFuzzyMatch(330, 220, 0.2), "CASE 3")
	check(img.AbsFuzzyMatch(360, 240, 0.2), "CASE 4")
	check(!img.AbsFuzzyMatch(361, 241, 0.2), "CASE 5")
	check(img.AbsFuzzyMatch(240, 160, 0.2), "CASE 6")
	check(!img.AbsFuzzyMatch(239, 159, 0.2), "CASE 7")

	check(img.FuzzyMatch(360, 230, 0.1, 0.2), "CASE 8")
	check(!img.FuzzyMatch(361, 230, 0.1, 0.2), "CASE 9")
	check(img.FuzzyMatch(310, 195, 0.1, 0.2), "CASE 10")
}

func ExampleCheckPrefix() {
	imgMap := map[string][]Img{
		"ALL": []Img{
			Img{Url: "https://test1.jpeg"},
			Img{Url: "test2.jpeg"},
			Img{Url: "//test3.jpeg"},
			Img{Url: "http://test4.jpeg"},
		},
	}

	checkAndFixPrefix(imgMap)

	for _, imgs := range imgMap {
		for _, img := range imgs {
			fmt.Println(img)
		}
	}
}
