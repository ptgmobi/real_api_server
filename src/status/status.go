package status

import (
	"io"
	"net/http"
	"time"

	"http_context"
)

type status struct {
	ctrl
	report
}

func (s *status) RecordReq(ctx *http_context.Context, offers []string) {
	if !s.SampleSwitch() || ctx == nil {
		return
	}
	info := &s.report.Info[ctx.Now.Minute()]
	info.visit()
	r := ctxToReq(ctx, offers)
	info.Retrieval.recordRequest(r)
	info.Phase.recordPhase(ctx)
}

func (s *status) GetStatus() (contentType string, r io.Reader) {
	ok, h5Reader := s.getSampleStatus()
	if !ok {
		return "text/html; charset=utf8", h5Reader
	}
	return "application/json; charset=utf8", s.getReports()
}

var defaultStatus *status = &status{}

func RecordReq(ctx *http_context.Context, offers []string) {
	defaultStatus.RecordReq(ctx, offers)
}

func Serve() {
	s := defaultStatus

	go func() {
		// clear report slot
		for t := range time.NewTicker(time.Minute).C {
			s.report.Info[t.Add(time.Minute).Minute()].clear()
		}
	}()

	http.HandleFunc("/status", func(w http.ResponseWriter, req *http.Request) {
		contentType, r := s.GetStatus()
		w.Header().Set("Content-Type", contentType)
		io.Copy(w, r)
	})
	panic(http.ListenAndServe(":8080", nil))
}
