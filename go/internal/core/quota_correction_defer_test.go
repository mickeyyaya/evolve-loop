package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// walledCorrectionProbe passes its first dispatch, blocks it at review, and answers the correction
// re-dispatch with the quota wall the runner returns after its whole family chain was walled.
type walledCorrectionProbe struct {
	*escalationProbe
	dispatches int
	wall       error // the correction's answer; nil means the quota wall (exit 85)
}

func (p *walledCorrectionProbe) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	p.dispatches++
	if req.CorrectionDirective != "" {
		if p.wall != nil {
			return PhaseResponse{}, p.wall
		}
		return PhaseResponse{}, wrapTransient(85)
	}
	return p.escalationProbe.Run(ctx, req)
}

func runWalledCorrectionCycle(t *testing.T) (root string, probe *walledCorrectionProbe, runners map[Phase]PhaseRunner, led *fakeLedger, err error) {
	t.Helper()
	root = t.TempDir()
	probe = &walledCorrectionProbe{escalationProbe: &escalationProbe{phase: "build", threshold: 99}}
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led = &fakeLedger{}
	runners = buildRunners(nil)
	runners[PhaseBuild] = probe
	o := NewOrchestrator(st, led, runners, WithReviewer(probe))
	_, err = o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true})
	return root, probe, runners, led, err
}

// A wall met on a correction re-dispatch is the same deferral a wall on the first dispatch is: the
// cycle is resumable, not failed, and nothing learns from it.
func TestRunCycle_WalledCorrectionRedispatch_DefersLikeAFirstDispatch(t *testing.T) {
	prevHook := QuotaBoundaryCheckpointer
	t.Cleanup(func() { QuotaBoundaryCheckpointer = prevHook })
	hookCalls, hookPhase := 0, ""
	QuotaBoundaryCheckpointer = func(cs CycleState, _ string, _ time.Time) error {
		hookCalls++
		hookPhase = cs.Phase
		return nil
	}

	root, probe, runners, led, err := runWalledCorrectionCycle(t)
	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err = %v, want the exhaustion sentinel the loop defers on", err)
	}
	if hookCalls != 1 || hookPhase != string(PhaseBuild) {
		t.Fatalf("quota checkpoint calls=%d phase=%q, want one checkpoint at build", hookCalls, hookPhase)
	}
	deferred := false
	for _, e := range led.entries {
		if e.Kind == "all_families_exhausted" && e.Role == string(PhaseBuild) {
			deferred = true
		}
	}
	if !deferred {
		t.Fatalf("ledger %+v lacks kind=all_families_exhausted for build", led.entries)
	}
	if calls := runners[PhaseRetro].(*fakeRunner).calls; calls != 0 {
		t.Fatalf("retro ran %d time(s) on a deferral; a wall is nothing to learn from", calls)
	}
	// The harness has no worktree provisioner, so the cycle start writes the one "worktree|…" digest; the
	// deferral must not add a build one for the consecutive-failure breaker to count.
	digests, _ := filepath.Glob(filepath.Join(root, ".evolve", "runs", "*", "failure-digest.json"))
	if len(digests) != 1 {
		t.Fatalf("digests = %v, want the harness's one worktree digest and nothing from the deferral", digests)
	}
	if raw, _ := os.ReadFile(digests[0]); !strings.Contains(string(raw), `"fingerprint":"worktree|`) {
		t.Fatalf("the only digest must be the harness's worktree one, not the deferral's:\n%s", raw)
	}
	if probe.dispatches != 2 {
		t.Fatalf("build dispatched %d times, want the first dispatch and one walled correction (no burn past the wall)", probe.dispatches)
	}
}

// walledGateRunner fails its gate verdict once, then answers the remediation re-run with the wall.
type walledGateRunner struct {
	name  Phase
	calls int
}

func (r *walledGateRunner) Name() string { return string(r.name) }
func (r *walledGateRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.calls++
	if r.calls > 1 {
		return PhaseResponse{}, wrapTransient(85)
	}
	return PhaseResponse{Phase: string(r.name), Verdict: VerdictFAIL, ArtifactsDir: req.Workspace}, nil
}

func TestRunCycle_WalledRemediationRerun_DefersLikeAFirstDispatch(t *testing.T) {
	prevHook := QuotaBoundaryCheckpointer
	t.Cleanup(func() { QuotaBoundaryCheckpointer = prevHook })
	hookCalls := 0
	QuotaBoundaryCheckpointer = func(CycleState, string, time.Time) error { hookCalls++; return nil }

	wf := policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"tdd"}}
	runners := buildRunners(nil)
	gate := &walledGateRunner{name: PhaseTDD}
	runners[PhaseTDD] = gate
	runners[PhaseBuild] = &scriptedRunner{name: PhaseBuild, verdicts: []string{VerdictPASS}}
	led := &fakeLedger{}
	o := NewOrchestrator(&fakeStorage{}, led, runners, WithWorkflowConfig(wf))
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err = %v, want the exhaustion sentinel: a walled re-run is a deferral, not \"the original FAIL stands\"", err)
	}
	if hookCalls != 1 {
		t.Fatalf("quota checkpoint calls=%d, want 1", hookCalls)
	}
	deferred := false
	for _, e := range led.entries {
		if e.Kind == "all_families_exhausted" && e.Role == string(PhaseTDD) {
			deferred = true
		}
	}
	if !deferred {
		t.Fatalf("ledger %+v lacks kind=all_families_exhausted for tdd", led.entries)
	}
	if calls := runners[PhaseRetro].(*fakeRunner).calls; calls != 0 {
		t.Fatalf("retro ran %d time(s) on a deferral", calls)
	}
	if gate.calls != 2 {
		t.Fatalf("gate dispatched %d times, want the FAIL and one walled re-run", gate.calls)
	}
}

func TestReviewResumedDeliverable_WalledCorrection_ReturnsTheExhaustionSentinel(t *testing.T) {
	reviewer := &rejectFirstResumedBuild{}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	o.reviewer = reviewer
	cs := CycleState{
		CycleID: 9, RunID: "run-9", WorkspacePath: t.TempDir(),
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	}
	_, err := o.reviewResumedDeliverable(
		context.Background(), t.TempDir(), 9, cs, PhaseBuild,
		&fakeRunner{name: string(PhaseBuild), failErr: wrapTransient(85), failUntil: 99},
		PhaseRequest{Workspace: cs.WorkspacePath}, PhaseResponse{Phase: string(PhaseBuild), Verdict: VerdictPASS}, nil,
	)
	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err = %v, want the exhaustion sentinel the resume path defers on", err)
	}
}

// The resume path consumes the review gate's sentinel through the same pause as its own dispatch.
func TestRunCycleFromPhase_WalledResumedCorrection_Defers(t *testing.T) {
	prevHook := QuotaBoundaryCheckpointer
	t.Cleanup(func() { QuotaBoundaryCheckpointer = prevHook })
	hookCalls := 0
	QuotaBoundaryCheckpointer = func(CycleState, string, time.Time) error { hookCalls++; return nil }

	workspace := t.TempDir()
	storage := &fakeStorage{cycleState: CycleState{CycleID: 5, RunID: "run-5", WorkspacePath: workspace}}
	// tdd stands in for the resumed phase: it carries no explanation binding, so the resume reaches the
	// review gate with the harness's fakes alone.
	probe := &walledCorrectionProbe{escalationProbe: &escalationProbe{phase: string(PhaseTDD), threshold: 99}}
	runners := buildRunners(nil)
	runners[PhaseTDD] = probe
	led := &fakeLedger{}
	o := NewOrchestrator(storage, led, runners, WithReviewer(probe))
	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: t.TempDir()}, &ResumePoint{Phase: string(PhaseTDD), CycleID: 5})
	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err = %v, want the exhaustion sentinel", err)
	}
	if hookCalls != 1 {
		t.Fatalf("quota checkpoint calls=%d, want 1: the resumed cycle must stay resumable", hookCalls)
	}
	deferred := false
	for _, e := range led.entries {
		if e.Kind == "all_families_exhausted" && e.Role == string(PhaseTDD) {
			deferred = true
		}
	}
	if !deferred {
		t.Fatalf("ledger %+v lacks kind=all_families_exhausted for tdd", led.entries)
	}
}

// A correction re-dispatch that fails for any other reason is still a failure, never a deferral.
func TestRunCycle_FailedCorrectionRedispatch_IsNotADeferral(t *testing.T) {
	prevHook := QuotaBoundaryCheckpointer
	t.Cleanup(func() { QuotaBoundaryCheckpointer = prevHook })
	hookCalls := 0
	QuotaBoundaryCheckpointer = func(CycleState, string, time.Time) error { hookCalls++; return nil }

	probe := &walledCorrectionProbe{escalationProbe: &escalationProbe{phase: "build", threshold: 99}, wall: wrapTransient(80)}
	runners := buildRunners(nil)
	runners[PhaseBuild] = probe
	led := &fakeLedger{}
	o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 0}}, led, runners, WithReviewer(probe))
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true})
	if err == nil || errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err = %v, want a failure that is not the exhaustion sentinel", err)
	}
	if hookCalls != 0 {
		t.Fatalf("a non-wall failure wrote %d quota checkpoint(s)", hookCalls)
	}
	for _, e := range led.entries {
		if e.Kind == "all_families_exhausted" {
			t.Fatalf("a non-wall failure was recorded as a deferral: %+v", e)
		}
	}
}
