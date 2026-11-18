package audit

import (
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/regressiontia"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type shadowEmitter func() *signalcenter.Center

func (s shadowEmitter) emit(e signalcenter.Event) {
	if s != nil {
		s().Emit(e)
	}
}

func tiaShadowEvent(req core.PhaseRequest, d regressiontia.Decision, writeErr error) signalcenter.Event {
	e := signalcenter.Event{
		Cycle: req.Cycle, RunID: req.RunID, Phase: string(core.PhaseAudit), Module: signalcenter.ModuleTIA, Origin: "emitTIADecision",
		Kind: signalcenter.KindAuditWarning, Severity: signalcenter.SeverityInfo, Code: signalcenter.CodeTIAShadow,
		Reason: "shadow: enforce would skip " + strconv.Itoa(d.WouldSkipCount) + " regression predicates",
		Fields: map[string]string{"would_skip_count": strconv.Itoa(d.WouldSkipCount), "selected": strconv.Itoa(len(d.Selected))},
	}
	if writeErr != nil {
		e.Severity = signalcenter.SeverityWarn
		e.Reason = "the TIA decision file could not be written; the soak loses this cycle's record: " + writeErr.Error()
		e.Fields["write_error"] = writeErr.Error()
	}
	return e
}
