package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type walkScript struct {
	name string
	errs []error
	n    int
}

func (s *walkScript) Name() string { return s.name }

func (s *walkScript) Run(context.Context, PhaseRequest) (PhaseResponse, error) {
	i := min(s.n, len(s.errs)-1)
	s.n++
	return PhaseResponse{Phase: s.name}, s.errs[i]
}

func walkOf(err error, ends ...FamilyEnd) error {
	var w Wall
	for _, e := range ends {
		w = w.Observe(e)
	}
	return WalkError{Wall: w, Err: err}
}

func TestRunCycle_TheWallSpansTheDispatchesOfAPhase(t *testing.T) {
	prevHook := QuotaBoundaryCheckpointer
	t.Cleanup(func() { QuotaBoundaryCheckpointer = prevHook })
	var saved Wall
	QuotaBoundaryCheckpointer = func(cs CycleState, _ string, _ time.Time) error {
		saved = Wall{Ends: cs.QuotaWall}
		return nil
	}
	c, got := recordingCenter()
	runners := buildRunners(nil)
	runners[PhaseScout] = &walkScript{name: "scout", errs: []error{
		walkOf(wrapTransient(85), FamilyEnd{CLI: "claude-tmux", Pattern: "auth_recheck"}, FamilyEnd{CLI: "agy", Pattern: "rate_limit"}),
		walkOf(wrapTransient(85), FamilyEnd{CLI: "claude-tmux"}, FamilyEnd{CLI: "agy", Pattern: "rate_limit"}),
	}}
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, runners, WithSignalCenter(c))
	o.retryConfig.RetryBackoffBaseS = 0

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})

	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err=%v, want the pause", err)
	}
	if saved.StopReason() != "login-required" || !strings.Contains(saved.Clauses(), "claude needs a login") {
		t.Errorf("the pause decides over both dispatches, so the login of the first one stands: %q %q", saved.StopReason(), saved.Clauses())
	}
	if n := len(eventsOfKind(*got, signalcenter.KindLoginRequired)); n != 0 {
		t.Errorf("a pause emits login.required at its own exit, not at the failure exit: %d", n)
	}
}

func TestRunCycle_AFailedPhaseWithALoginRowEmitsLoginRequiredOnce(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ends []FamilyEnd
		want int
	}{
		{"login row", []FamilyEnd{{CLI: "claude-tmux", Pattern: "auth_recheck"}, {CLI: "codex", Pattern: "model_not_found"}}, 1},
		{"no login row", []FamilyEnd{{CLI: "agy", Pattern: "rate_limit"}}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, got := recordingCenter()
			runners := buildRunners(nil)
			runners[PhaseScout] = &walkScript{name: "scout", errs: []error{walkOf(wrapTransient(1), tc.ends...)}}
			o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, runners, WithSignalCenter(c))

			_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})

			if err == nil || errors.Is(err, ErrAllFamiliesExhausted) {
				t.Fatalf("err=%v, want a plain phase failure", err)
			}
			events := eventsOfKind(*got, signalcenter.KindLoginRequired)
			if len(events) != tc.want {
				t.Fatalf("login.required events = %d, want %d: %+v", len(events), tc.want, events)
			}
			if tc.want == 1 && (events[0].Phase != "scout" || !strings.Contains(events[0].Reason, "claude needs a login; codex ended on model_not_found") || events[0].Fields["stop_reason"] != "login-required") {
				t.Errorf("event %+v", events[0])
			}
		})
	}
}

func TestRetryPhaseRunner_AFailedPhaseNotesTheMergedLoginWall(t *testing.T) {
	t.Parallel()
	c, got := recordingCenter()
	runner := &walkScript{name: "evaluator", errs: []error{
		walkOf(wrapTransient(81), FamilyEnd{CLI: "claude-tmux", Pattern: "auth_recheck"}),
		walkOf(wrapTransient(81), FamilyEnd{CLI: "claude-tmux"}, FamilyEnd{CLI: "codex"}),
	}}
	o := retryParityOrchestrator(t, runner, "evaluator", phasespec.PhaseSpec{})
	o.signals = c
	o.retryConfig.RetryBackoffBaseS = 0
	cr := retryParityCycleRun(o, t)

	_, _, err := cr.retryPhaseRunner(Phase("evaluator"), PhaseRequest{}, retryOpts{})

	if err == nil {
		t.Fatal("want the failure")
	}
	events := eventsOfKind(*got, signalcenter.KindLoginRequired)
	if len(events) != 1 || !strings.Contains(events[0].Reason, "claude needs a login; codex ended with no recorded cause") {
		t.Errorf("events %+v", events)
	}
}

func TestRunCycle_AFailedCorrectionDispatchNotesItsLoginWall(t *testing.T) {
	t.Parallel()
	c, got := recordingCenter()
	probe := &walledCorrectionProbe{
		escalationProbe: &escalationProbe{phase: "build", threshold: 99},
		wall:            walkOf(wrapTransient(1), FamilyEnd{CLI: "claude-tmux", Pattern: "auth_recheck"}, FamilyEnd{CLI: "codex", Pattern: "model_not_found"}),
	}
	runners := buildRunners(nil)
	runners[PhaseBuild] = probe
	o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 0}}, &fakeLedger{}, runners, WithReviewer(probe), WithSignalCenter(c))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true})

	if err == nil || errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err=%v, want a failed correction dispatch", err)
	}
	events := eventsOfKind(*got, signalcenter.KindLoginRequired)
	if len(events) != 1 || events[0].Phase != "build" || !strings.Contains(events[0].Reason, "claude needs a login; codex ended on model_not_found") {
		t.Errorf("events %+v", events)
	}
}

type resumeWalkOnCorrectionRunner struct {
	calls int
	err   error
}

func (r *resumeWalkOnCorrectionRunner) Name() string { return "scout" }
func (r *resumeWalkOnCorrectionRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.calls++
	if r.calls > 1 {
		return PhaseResponse{}, r.err
	}
	return PhaseResponse{Phase: "scout", Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func TestResumeExecution_AFailedResumedCorrectionNotesItsLoginWall(t *testing.T) {
	t.Parallel()
	c, got := recordingCenter()
	_, runners, o := resumeRigE4(t, t.TempDir(), nil,
		WithReviewer(stubReviewer{result: ReviewResult{Approve: false, Reason: "deliverable incomplete"}}), WithSignalCenter(c))
	runners[PhaseScout] = &resumeWalkOnCorrectionRunner{
		err: walkOf(wrapTransient(81), FamilyEnd{CLI: "claude-tmux", Pattern: "auth_recheck"}, FamilyEnd{CLI: "codex"}),
	}

	_, err := resumeFromE4(o, PhaseScout)

	if err == nil || errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err=%v, want a plain failure", err)
	}
	events := eventsOfKind(*got, signalcenter.KindLoginRequired)
	if len(events) != 1 || !strings.Contains(events[0].Reason, "claude needs a login; codex ended with no recorded cause") {
		t.Errorf("events %+v", events)
	}
}

func TestRunCycle_APausedResultCarriesTheWallAndTheCheckpointWriteError(t *testing.T) {
	prevHook := QuotaBoundaryCheckpointer
	t.Cleanup(func() { QuotaBoundaryCheckpointer = prevHook })
	QuotaBoundaryCheckpointer = func(CycleState, string, time.Time) error { return errors.New("disk full") }
	runners := buildRunners(nil)
	runners[PhaseScout] = &walkScript{name: "scout", errs: []error{
		walkOf(wrapTransient(85), FamilyEnd{CLI: "claude-tmux", Pattern: "auth_recheck"}, FamilyEnd{CLI: "agy", Pattern: "rate_limit"}),
	}}
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, runners)
	o.retryConfig.RetryBackoffBaseS = 0

	result, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})

	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err=%v", err)
	}
	wall := Wall{Ends: result.QuotaWall}
	if wall.StopReason() != "login-required" || result.WallPhase != "scout" || result.WallWriteErr != "disk full" {
		t.Errorf("result wall=%v phase=%q writeErr=%q", result.QuotaWall, result.WallPhase, result.WallWriteErr)
	}
}
