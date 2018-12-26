package retrieval

import (
	"math/rand"

	"ad"
	"http_context"
	"raw_ad"
)

func shuffleId(id string) (newId string) {
	if len(id) == 0 {
		return
	}

	runes := []rune(id)
	for i := 0; i != 8; i++ {
		x, y := rand.Intn(len(runes)), rand.Intn(len(runes))
		if runes[x] != rune('-') && runes[y] != rune('-') {
			runes[x], runes[y] = runes[y], runes[x]
		}
	}

	return string(runes)
}

func reduceCvr(nat *ad.NativeAdObj, raw *raw_ad.RawAdObj, ctx *http_context.Context, dup int) *ad.NativeAdObj {
	if dup <= 0 {
		return nat
	}

	oAid, oGaid := ctx.Aid, ctx.Gaid
	ctx.Fake = true
	for i := 0; i != dup; i++ {
		ctx.Aid, ctx.Gaid = shuffleId(oAid), shuffleId(oGaid)
		if newNat := raw.ToNativeAd(ctx, 1); newNat != nil {
			if len(newNat.ImpTkUrl) > 0 {
				nat.ImpTkUrl = append(nat.ImpTkUrl, newNat.ImpTkUrl...)
				nat.ImpTkUrl = append(nat.ImpTkUrl, newNat.ClkUrl)
			}
		}
	}
	ctx.Fake = false
	ctx.Aid, ctx.Gaid = oAid, oGaid

	return nat
}
