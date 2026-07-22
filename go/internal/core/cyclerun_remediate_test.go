package core

// cyclerun_remediate_test.go — graduated remediation (operator directive
// 2026-07-21; inbox graduated-remediation-fix-forward): when a configured
// DETERMINISTIC gate phase FAILs, the orchestrator dispatches the builder ONCE
// with the gate's report as a correction directive, re-runs the SAME gate, and
// records the final verdict — instead of discarding a sound cycle over a
// mechanical, prescribed defect (the 983/992/1007/1019/1020 waste class:
// cycle-1019's audit-PASSed S5 implementation was thrown away over three
// missing test files the gate itself had prescribed; 1020 then re-implemented
// it from scratch and failed the same gate the same way).
//
// Integrity properties pinned here: nothing downstream is bypassed (the SAME
// gate must pass and the spine continues normally); the round cap is hard; a
// zero-value workflow config means ZERO remediation (byte-identical legacy
// behavior — compiled defaults live at the composition root, not in core).

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// scriptedRunner returns queued verdicts call-by-call (last repeats) and
// records every CorrectionDirective it was dispatched with.
type scriptedRunner struct {
	name       Phase
	verdicts   []string
	calls      int
	directives []string
}

func (r *scriptedRunner) Name() string { return string(r.name) }
func (r *scriptedRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	i := r.calls
	if i >= len(r.verdicts) {
		i = len(r.verdicts) - 1
	}
	r.calls++
	r.directives = append(r.directives, req.CorrectionDirective)
	return PhaseResponse{Phase: string(r.name), Verdict: r.verdicts[i], ArtifactsDir: req.Workspace}, nil
}

func remediationHarness(t *testing.T, wf policy.WorkflowConfig, gate Phase, gateVerdicts []string) (*scriptedRunner, *scriptedRunner, CycleResult, error) {
	t.Helper()
	runners := buildRunners(nil)
	gr := &scriptedRunner{name: gate, verdicts: gateVerdicts}
	build := &scriptedRunner{name: PhaseBuild, verdicts: []string{VerdictPASS}}
	runners[gate] = gr
	runners[PhaseBuild] = build
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorkflowConfig(wf))
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	return gr, build, res, err
}

// remediationDispatches counts builder calls that carried a REMEDIATION
// directive (the spine's own build phase runs with an empty one).
func remediationDispatches(build *scriptedRunner) []string {
	var out []string
	for _, d := range build.directives {
		if strings.Contains(d, "REMEDIATION") {
			out = append(out, d)
		}
	}
	return out
}

func TestRemediation_GateFailThenPassContinuesSpine(t *testing.T) {
	// tdd stands in for a deterministic gate: spine-reachable with the fake
	// runner map and NOT on the judgment deny-list (which the mechanism
	// enforces regardless of configuration — pinned separately below).
	wf := policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"tdd"}}
	gate, build, res, err := remediationHarness(t, wf, PhaseTDD, []string{VerdictFAIL, VerdictPASS})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	rems := remediationDispatches(build)
	if len(rems) != 1 {
		t.Fatalf("want exactly 1 remediation builder dispatch, got %d (directives: %q)", len(rems), build.directives)
	}
	if !strings.Contains(rems[0], "tdd") || !strings.Contains(rems[0], "tdd-report.md") {
		t.Errorf("remediation directive must name the gate and its report; got %q", rems[0])
	}
	if gate.calls != 2 {
		t.Fatalf("gate must re-run after the fix: calls=%d, want 2 (remediated PASS ends the retry pressure)", gate.calls)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Fatalf("remediated cycle must continue to a PASS verdict; got %q", res.FinalVerdict)
	}
	if len(res.Remediations) != 1 || !strings.Contains(res.Remediations[0], "tdd") || !strings.Contains(res.Remediations[0], VerdictPASS) {
		t.Errorf("provenance must record the remediated gate + outcome; got %v", res.Remediations)
	}
}

func TestRemediation_RoundCapIsHard(t *testing.T) {
	wf := policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"tdd"}}
	gate, build, res, err := remediationHarness(t, wf, PhaseTDD, []string{VerdictFAIL, VerdictFAIL})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if got := len(remediationDispatches(build)); got != 1 {
		t.Fatalf("cap=1 must mean exactly 1 remediation dispatch, got %d", got)
	}
	// After the capped remediation fails, the cycle proceeds down the SAME
	// legacy path as an unremediated FAIL (retro / fluent-vs-strict semantics
	// own the final verdict — remediation never overrides them). The audit may
	// re-run additional times via the legacy retry loop; the cap governs
	// REMEDIATION dispatches only.
	if gate.calls < 2 {
		t.Fatalf("gate calls=%d, want >=2 (original + the one remediation re-run)", gate.calls)
	}
	if len(res.Remediations) != 1 || !strings.Contains(res.Remediations[0], VerdictFAIL) {
		t.Errorf("provenance must record the failed remediation; got %v", res.Remediations)
	}
}

func TestRemediation_ZeroConfigIsByteIdenticalLegacy(t *testing.T) {
	_, build, res, err := remediationHarness(t, policy.WorkflowConfig{}, PhaseTDD, []string{VerdictFAIL})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// Byte-identical legacy = the DELTAS are zero: no remediation dispatches,
	// no provenance. (The final verdict and audit retry count belong to the
	// legacy fluent/strict path and are pinned by the existing orchestrator
	// tests, not re-asserted here.)
	if got := len(remediationDispatches(build)); got != 0 {
		t.Fatalf("zero-value config must never remediate; got %d dispatches", got)
	}
	if len(res.Remediations) != 0 {
		t.Errorf("no remediation provenance expected; got %v", res.Remediations)
	}
}

func TestRemediation_UnlistedPhaseUntouched(t *testing.T) {
	wf := policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"coverage-gate"}}
	_, build, res, err := remediationHarness(t, wf, PhaseTDD, []string{VerdictFAIL})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if got := len(remediationDispatches(build)); got != 0 {
		t.Fatalf("audit is not in the remediable list; got %d dispatches", got)
	}
	if len(res.Remediations) != 0 {
		t.Errorf("no remediation provenance for an unlisted phase; got %v", res.Remediations)
	}
}

// TestRemediation_JudgmentPhasesDeniedRegardlessOfConfig pins the deny-list:
// configuring a judgment phase (audit) as remediable is REFUSED by the
// mechanism itself — a builder re-roll against an LLM-judgment verdict would
// be a gamed verdict, not a fix.
func TestRemediation_JudgmentPhasesDeniedRegardlessOfConfig(t *testing.T) {
	wf := policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"audit"}}
	_, build, res, err := remediationHarness(t, wf, PhaseAudit, []string{VerdictFAIL})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if got := len(remediationDispatches(build)); got != 0 {
		t.Fatalf("judgment phase must never remediate; got %d dispatches", got)
	}
	if len(res.Remediations) != 0 {
		t.Errorf("no provenance for a denied phase; got %v", res.Remediations)
	}
}

// TestRemediation_RecordsFixDispatchInPhaseRecord pins the ADR-0044 C1
// chokepoint parity: the remediation fix dispatch appears in the phase record
// under its own label (never clobbering the build phase's own records).
func TestRemediation_RecordsFixDispatchInPhaseRecord(t *testing.T) {
	wf := policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"tdd"}}
	_, _, res, err := remediationHarness(t, wf, PhaseTDD, []string{VerdictFAIL, VerdictPASS})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	found := false
	for _, p := range res.PhasesRun {
		if p == Phase("build-remediation") {
			found = true
		}
	}
	if !found {
		t.Fatalf("PhasesRun must record the remediation fix dispatch; got %v", res.PhasesRun)
	}
}

// diagRunner returns a FAIL with error-severity diagnostics — the audit
// in-process override shape (cycle-1022).
type diagRunner struct{ name Phase }

func (r *diagRunner) Name() string { return string(r.name) }
func (r *diagRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	return PhaseResponse{Phase: string(r.name), Verdict: VerdictFAIL, ArtifactsDir: req.Workspace,
		Diagnostics: []Diagnostic{{Severity: "error", Message: "apicover -enforce flagged 1 line(s) — unnamed export"}}}, nil
}

// TestFailReasonsSurfaceInResult pins the cycle-1022 lesson: a floor-override
// FAIL's explanation must reach the RESULT (summary + dossier surfaces), not
// just workspace artifacts and orchestrator memory.
func TestFailReasonsSurfaceInResult(t *testing.T) {
	runners := buildRunners(nil)
	runners[PhaseAudit] = &diagRunner{name: PhaseAudit}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners)
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	found := false
	for _, r := range res.FailReasons {
		if strings.Contains(r, "apicover") {
			found = true
		}
	}
	if !found {
		t.Fatalf("FailReasons must surface the override explanation; got %v", res.FailReasons)
	}
}
