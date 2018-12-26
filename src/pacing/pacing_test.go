package pacing_test

import (
	"testing"
	"time"

	"pacing"
)

func TestPacing(t *testing.T) {
	now := time.Now().UTC()
	id := "ym_111"
	pc := pacing.NewPacingController("OfferServerNewASG", "ap-southeast-1")

	if pc.OverCap(id, "US", -1, now) {
		t.Error("unexpected overcap")
	}

	pc.Add(id, now, 1)
	if pc.OverCap(id, "US", -1, now) {
		t.Error("unexpected overcap")
	}

	pc.Add(id, now, 1)
	if pc.OverCap(id, "US", -1, now) {
		t.Error("unexpected overcap")
	}

	pc.Add(id, now, 1)
	if !pc.OverCap(id, "US", -1, now) {
		t.Error("unexpected not overcap")
	}

	if pc.Size() != 1 {
		t.Error("pc.Size error")
	}

	time.Sleep(3 * time.Minute)

	if pc.OverCap(id, "US", -1, now) {
		t.Error("expired")
	}
}
