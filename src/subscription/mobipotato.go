package subscription

import (
	"http_context"
)

func GetMoreMobiPotatoJs(ctx *http_context.Context) ([]*Subscription, string) {

	//mopUrl := "http://api.anduads.com/api/job/take"
	subs := make([]*Subscription, 0, 1)
	ctx.SubChannel = "mop"
	return subs, gSubControl.mobipotatoRedirectUrl
}
