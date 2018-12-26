package status

import (
	"fmt"

	"http_context"
)

type Request struct {
	numReq    int
	numFilled int
	country   string
	platform  string
	adtype    string
	slotid    string
	offers    []string
}

func ctxToReq(ctx *http_context.Context, offers []string) *Request {
	if ctx.AdNum < len(offers) {
		fmt.Println("[WARNING] ctx.AdNum: ", ctx.AdNum, "<", "len(offers): ", len(offers))
		ctx.AdNum = len(offers)
	}
	return &Request{
		numReq:    ctx.AdNum,
		numFilled: len(offers),
		country:   ctx.Country,
		platform:  ctx.Platform,
		adtype:    ctx.AdType,
		slotid:    ctx.SlotId,
		offers:    offers,
	}
}
