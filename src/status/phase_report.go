package status

import (
	"sync"

	"http_context"
)

type phaseReport struct {
	sync.Mutex
	Phase     map[string]int `json:"phase"`
	FuyuPhase map[string]int `json:"fuyu_phase"`
}

func NewPhaseReport() *phaseReport {
	pr := &phaseReport{
		Phase:     make(map[string]int, 256),
		FuyuPhase: make(map[string]int, 16),
	}
	pr.FuyuPhase["CNiOSWuganEmpty"] = 0 // init
	return pr
}

func (p *phaseReport) recordPhase(ctx *http_context.Context) {
	p.Lock()
	defer p.Unlock()
	p.Phase[ctx.Phase]++

	if len(ctx.FuyuPhase) == 0 {
		p.FuyuPhase["Normal"]++
	} else {
		p.FuyuPhase[ctx.FuyuPhase]++
	}
}
