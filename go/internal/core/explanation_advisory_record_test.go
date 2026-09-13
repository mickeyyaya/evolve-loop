package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// ADR-0102: an explanation-review advisory raised on a PASS verdict reaches
// the C1 record — the warning trail #577 keeps on a PASS — which is the
// persistence the decision's feedback loop rests on. The other half (the
// audit classification emits the prefixed advisory on a PASS) is pinned in
// internal/phases/audit.
func TestRecordPhaseOutcome_AnAdvisoryOnAPassRidesTheRecord(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(signalcenter.New()))
	advisory := explanationdocs.AdvisoryPrefix + "explanation review Evidence must cite go/app.go with path:line evidence"
	passing := PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictPASS, Diagnostics: []Diagnostic{{Severity: "warning", Message: advisory}}}
	var result CycleResult
	var timings []phaseTimingEntry
	_ = redirectStderr(t, func() {
		o.recordPhaseOutcome(&result, &timings, t.TempDir(), phaseOutcomeFrom(PhaseAudit, passing, 1, "", ""))
	})
	if len(timings) != 1 || len(timings[0].Diagnostics) != 1 || timings[0].Diagnostics[0].Message != advisory || timings[0].Diagnostics[0].Severity != "warning" {
		t.Fatalf("the ADR-0102 advisory must ride the C1 record on a PASS: %+v", timings)
	}
}
