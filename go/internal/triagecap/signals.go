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
	signalcenter.RegisterCode(signalcenter.ModuleTriage, CodeCompanionMalformed, "the triage-decision.json companion beside the triage report is present but malformed (invalid JSON, or a committed_floors or deferred_floors field that is not a list of strings); the capacity clamp falls back to counting floors in the report prose; WARN; reason names the file and the parse error")
	signalcenter.RegisterCode(signalcenter.ModuleTriage, CodeDemotionDefectUnwritten, "the triage capacity clamp demoted itself to shadow for one cycle (ADR-0046 Layer 2), but its auto-filed inbox defect could not be written; the cycle is still relieved, but no record marks the one-cycle relief as used, so a later cycle of the same pair can be relieved again; WARN; reason names the defect file and the write error")
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
