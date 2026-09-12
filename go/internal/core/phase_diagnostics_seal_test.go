package core

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// phase_diagnostics_seal_test.go — a phase's own FAIL reason must reach the seal.
//
// Live incidents (two-wave health batch, 2026-09-12/13): cycles 1634 and 1636
// both FAILed at triage because Classify's protected-surface admission check
// refused a top_n card naming a control-plane file. Classify returned the
// reason as an error-severity diagnostic; the C1 outcome record carried no
// field for it, so phase-timing.json held only `verdict: FAIL` and the seal
// wrote "phase triage: verdict FAIL with no recorded abort reason (phase-infra
// class)" — a deterministic, reasoned rejection paged as infrastructure, twice,
// with the actual reason persisted nowhere. Floor phases never hit this: their
// diagnostics ride a side channel (persistFloorFailReasons). Every other phase
// dropped them at the chokepoint.

// diagnosticFailRunner returns FAIL with the phase's own diagnostics — the
// shape Classify produces for an admission rejection.
type diagnosticFailRunner struct {
	name  string
	diags []Diagnostic
}

func (r diagnosticFailRunner) Name() string { return r.name }
func (r diagnosticFailRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	return PhaseResponse{Phase: r.name, Verdict: VerdictFAIL, ArtifactsDir: req.Workspace, Diagnostics: r.diags}, nil
}

const protectedSurfaceRejection = `top_n card "phase-stub-shape-rule-at-ship-staging" names protected surface "go/internal/phases/ship/gitops.go" — control-plane changes go through the console route (operator-gated), not lane top_n`

func TestRunCycle_TriageOwnFailReasonReachesTheSealAndTheRecord(t *testing.T) {
	t.Parallel()
	storage := &fakeStorage{}
	runners := buildRunners(nil)
	runners[PhaseTriage] = diagnosticFailRunner{name: string(PhaseTriage), diags: []Diagnostic{
		{Severity: "warning", Message: "phase-tracker metrics file absent"},
		{Severity: "error", Message: protectedSurfaceRejection},
	}}
	o := NewOrchestrator(storage, &fakeLedger{}, runners, WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}))

	result, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "protected-top-n"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.FinalVerdict != VerdictFAIL {
		t.Fatalf("FinalVerdict = %q, want FAIL", result.FinalVerdict)
	}
	joined := strings.Join(result.FailReasons, "\n")
	if !strings.Contains(joined, "names protected surface") || !strings.Contains(joined, "phase triage") {
		t.Errorf("the seal must name triage's own reason, got %v", result.FailReasons)
	}
	if strings.Contains(joined, "phase-infra class") {
		t.Errorf("a reasoned rejection must not be sealed as infra: %v", result.FailReasons)
	}
	entries, err := phasetiming.Read(storage.cycleState.WorkspacePath)
	if err != nil {
		t.Fatalf("phase-timing.json unreadable: %v", err)
	}
	var triage *phasetiming.Entry
	for i := range entries {
		if entries[i].Phase == string(PhaseTriage) {
			triage = &entries[i]
		}
	}
	if triage == nil {
		t.Fatalf("no triage entry in phase-timing.json: %+v", entries)
	}
	if len(triage.Diagnostics) != 2 || triage.Diagnostics[1].Message != protectedSurfaceRejection {
		t.Errorf("the durable record must carry the phase's own diagnostics, got %+v", triage.Diagnostics)
	}
	// Writer/reader parity: the batch-report classifier reads the SAME record the
	// chokepoint wrote (through phasetiming.Entry, not a hand-typed mirror), so a
	// schema drift cannot silently revert the detail to verdict-only.
	if outcome, detail := cyclehealth.ClassifyOutcome(storage.cycleState.WorkspacePath); outcome != cyclehealth.OutcomeFailedExplained || !strings.Contains(detail, "names protected surface") {
		t.Errorf("cyclehealth must name the reason from the record core wrote: %s %q", outcome, detail)
	}
}

// redirectStderr runs fn with os.Stderr captured (package-internal twin of the
// external-package helper in cyclerun_chronicle_test.go, which package core
// cannot call).
func redirectStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	defer func() { os.Stderr = old }()
	fn()
	_ = w.Close()
	os.Stderr = old
	return <-done
}

// The C1 chokepoint is the ONE producer: phaseOutcomeFrom relays the phase's
// diagnostics, recordPhaseOutcome writes them to the record and names a
// reasoned FAIL once in the log with the same wording the seal will use. A PASS
// carrying warnings is recorded (the durable trail) but not logged as a failure.
func TestRecordPhaseOutcome_CarriesThePhaseDiagnosticsAndNamesAReasonedFail(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	failing := PhaseResponse{Phase: string(PhaseTriage), Verdict: VerdictFAIL, Diagnostics: []Diagnostic{
		{Severity: "warning", Message: "metrics file absent"},
		{Severity: "error", Message: protectedSurfaceRejection},
	}}
	out := phaseOutcomeFrom(PhaseTriage, failing, 1, "", "")
	if len(out.Diagnostics) != 2 {
		t.Fatalf("phaseOutcomeFrom dropped the diagnostics: %+v", out)
	}
	var result CycleResult
	var timings []phaseTimingEntry
	stderr := redirectStderr(t, func() { o.recordPhaseOutcome(&result, &timings, t.TempDir(), out) })
	if len(timings) != 1 || len(timings[0].Diagnostics) != 2 || timings[0].Diagnostics[1].Message != protectedSurfaceRejection {
		t.Errorf("the record must carry the phase's diagnostics verbatim: %+v", timings)
	}
	want := "[orchestrator] phase triage verdict=FAIL: " + protectedSurfaceRejection
	if !strings.Contains(stderr, want) {
		t.Errorf("a reasoned FAIL must be named once at the chokepoint; stderr:\n%s", stderr)
	}
	if strings.Contains(stderr, "metrics file absent") {
		t.Errorf("warnings are not reasons and must not be logged as one: %s", stderr)
	}

	passing := PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictPASS, Diagnostics: []Diagnostic{{Severity: "warning", Message: "ACS floor overrode a hygiene flag"}}}
	timings = nil
	stderr = redirectStderr(t, func() {
		o.recordPhaseOutcome(&result, &timings, t.TempDir(), phaseOutcomeFrom(PhaseAudit, passing, 1, "", ""))
	})
	if len(timings) != 1 || len(timings[0].Diagnostics) != 1 {
		t.Errorf("a PASS keeps its warning trail on the record: %+v", timings)
	}
	if strings.Contains(stderr, "verdict=FAIL") {
		t.Errorf("a PASS must not be logged as a failure: %s", stderr)
	}
}
