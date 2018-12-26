package retrieval

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"aes"
	"http_context"
	"subscription"
)

type subResp struct {
	subscriptions []*subscription.Subscription
	ctx           *http_context.Context
}

func NewSubResp(ctx *http_context.Context) *subResp {
	return &subResp{
		ctx: ctx,
	}
}

func (resp *subResp) WriteTo(w http.ResponseWriter) (int, error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var b []byte

	if len(resp.subscriptions) == 0 {
		b = []byte("[]")
	} else {
		b, _ = json.Marshal(resp.subscriptions)
	}

	if resp.ctx != nil {

		var buf bytes.Buffer

		if resp.ctx.UseGzipCT && len(b) > 1000 {
			w.Header().Set("CT-Content-Encoding", "gzip")
			enc := gzip.NewWriter(&buf)
			enc.Write(b)
			enc.Close()
			b = buf.Bytes()
		}

		if resp.ctx.UseAes {
			b = aes.EncryptBytes(b)
			w.Header().Set("CT-Encrypt", "hex") // by default, set hex

			if resp.ctx.TestEncode != "hex" {
				w.Header().Set("CT-Encrypt", "binary") // overwrite CT-Encrypt
				decodeBytes := make([]byte, hex.DecodedLen(len(b)))
				hex.Decode(decodeBytes, b)
				b = decodeBytes
			}
		}

		if resp.ctx.UseGzip && len(b) > 1000 {
			w.Header().Set("Content-Encoding", "gzip")
			enc := gzip.NewWriter(w)
			defer enc.Close()
			return enc.Write(b)
		}
	}

	return w.Write(b)
}

func (s *Service) subHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		NewSubResp(nil).WriteTo(w)
		return
	}

	conds := s.makeRetrievalConditions(ctx)

	var subs []*subscription.Subscription
	var redirectUrl string
	if ctx.Platform == "Android" && len(ctx.UA) > 0 {
		subs, redirectUrl = subscription.GetMoreSubJs(ctx, conds)
	}

	if len(redirectUrl) > 0 { // 需要重定向
		http.Redirect(w, r, redirectUrl+ctx.GetRawPath(), http.StatusFound)
		return
	}

	resp := NewSubResp(ctx)
	resp.subscriptions = subs
	resp.WriteTo(w)
}
