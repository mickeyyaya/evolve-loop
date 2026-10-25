package core

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func (cr *cycleRun) reportProseFallbacks(sig router.RoutingSignals) {
	if cr.proseReported == nil {
		cr.proseReported = map[router.ProseFallback]bool{}
	}
	for _, f := range sig.ProseFallbacks {
		if cr.proseReported[f] {
			continue
		}
		cr.proseReported[f] = true
		cell := phasecontract.FlowCell{Cycle: cr.cycle, Phase: string(cr.current), Origin: "cycleRun.reportProseFallbacks", Path: f.Path}
		cr.o.signals.Emit(phasecontract.FlowViolationEvent(proseViolation(f), cell))
	}
}

func proseViolation(f router.ProseFallback) phasecontract.FlowViolation {
	v := phasecontract.FlowViolation{Seat: f.Seat, Stage: f.Stage, Action: "logged", Rule: phasecontract.FlowRuleProseFallback,
		Reason: "the " + f.Seat + " routing signals came from the prose headers of " + filepath.Base(f.Path) + " because its handoff is absent"}
	if f.Refused {
		v.Action = "refused"
		v.Reason = "the " + f.Seat + " handoff is absent, and the I-8 stage enforce refused the prose headers of " + filepath.Base(f.Path)
	}
	return v
}
