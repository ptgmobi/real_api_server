package retrieval

import (
	"net/http"
	"strconv"

	"http_context"
	"raw_ad"
	"real_api"
)

/*
See (https://git.oschina.net/CloudTech/Document/blob/master/adserver_native.md) for detail
*/
func (s *Service) nativeHandler(w http.ResponseWriter, r *http.Request) {
	s.stat.GetNatStat().IncrTot()
	ctx, err := http_context.NewContext(r, s.l)
	if err != nil {
		s.l.Println("nativeHandler new context err: ", err)
		if n, err := NewRtvResp("url parameters error", 42, nil).WriteTo(w); err != nil {
			s.l.Println("[native] context err resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrCtxErr()
		return
	}
	s.SetCtxTks(ctx)

	ctx.Estimate("BeginNavieAd: " + ctx.Platform)
	defer func() {
		ctx.Estimate("End")
		ctx.LogEstimate()
	}()

	raws := make([]*raw_ad.RawAdObj, 0, 1)
	if raw, err := real_api.Request(ctx); err == nil {
		raws = append(raws, raw)
	} else {
		s.l.Println("[real_api] ", err)
	}

	if len(raws) == 0 {
		ctx.Phase = "NativeRankZero"
		if n, err := NewRtvResp("No ads", 1, ctx).WriteTo(w); err != nil {
			s.l.Println("[native] rank no ads resp write: ", n, ", error:", err)
		}
		s.stat.GetNatStat().IncrRankFilted()
		return
	}

	ctx.Phase = "NativeOK"
	resp := NewRtvResp("ok", 0, ctx)

	for i, raw := range raws {
		if adv := raw.ToNativeAd(ctx, i); adv != nil {
			if ctx.IsWugan() {
				ctx.FreqInfo.PFreqFields = append(ctx.FreqInfo.PFreqFields, raw.AppDownload.PkgName)
			}

			ctx.AdPkgName = append(ctx.AdPkgName, raw.AppDownload.PkgName)

			resp.AdList = append(resp.AdList, adv)
			resp.offers = append(resp.offers, raw.UniqId)
		}
	}

	ctx.Estimate("ToNativeAds: " + strconv.Itoa(len(resp.AdList)))

	if len(resp.AdList) > 0 {
		if n, err := resp.WriteTo(w); err != nil {
			s.l.Println("[native] resp write: ", n, ", error:", err)
		}
		ctx.Estimate("WriteTo")
		s.stat.GetNatStat().IncrImp()
		return
	}

	ctx.Phase = "NativeZero"
	if n, err := NewRtvResp("no native ad suggestted", 6, ctx).WriteTo(w); err != nil {
		s.l.Println("[native] no wugan ad resp write: ", n, ", error:", err)
	}
	s.stat.GetNatStat().IncrWuganFilted()
	return
}
