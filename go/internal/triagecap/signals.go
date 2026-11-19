package triagecap

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeCompanionMalformed      signalcenter.Code = "TRIAGE_CAP_COMPANION_MALFORMED"
	CodeDemotionDefectUnwritten signalcenter.Code = "TRIAGE_CAP_DEMOTION_DEFECT_UNWRITTEN"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleTriage, CodeCompanionMalformed, "triage-decision.json beside the triage report is malformed (bad JSON or non-string-list committed_floors or deferred_floors). Capacity clamp falls back to counting report prose; WARN. Reason names file and error")
	signalcenter.RegisterCode(signalcenter.ModuleTriage, CodeDemotionDefectUnwritten, "triage capacity clamp demoted to shadow for one cycle (ADR-0046 Layer 2), but auto-filed defect write failed. Cycle is relieved, but relief record is missing, so later cycles can re-trigger; WARN. Reason names file and error")
}

func (r *CapReviewer) warnMalformedCompanion(in core.ReviewInput, companionPath string) {
	msg := MalformedCommittedFloorWarning(companionPath)
	if msg == "" {
		msg = MalformedDeferredFloorWarning(companionPath)
	}
	if msg != "" {
		r.warn(in, CodeCompanionMalformed, msg)
	}
}

func (r *CapReviewer) warn(in core.ReviewInput, code signalcenter.Code, reason string) {
	r.logf("[triage-cap] WARN %s: %s", code, reason)
	in.Signals.Emit(signalcenter.Event{
		Cycle: in.Cycle, RunID: in.RunID, Phase: in.Phase,
		Module: signalcenter.ModuleTriage, Origin: "CapReviewer.Review", Kind: signalcenter.KindGatePassed,
		Severity: signalcenter.SeverityWarn, Code: code, Reason: reason,
	})
}
