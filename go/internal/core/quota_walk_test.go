package core

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestRunCycle_AWalledPauseHandsTheWalkedCLIsToTheCheckpoint(t *testing.T) {
	prevHook := QuotaBoundaryCheckpointer
	t.Cleanup(func() { QuotaBoundaryCheckpointer = prevHook })
	var walked Wall
	QuotaBoundaryCheckpointer = func(cs CycleState, _ string, _ time.Time) error {
		walked = Wall{Ends: cs.QuotaWall}
		return nil
	}
	walk := WalkError{Wall: Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "rate_limit"}).Observe(FamilyEnd{CLI: "agy-claude-tmux"}), Err: wrapTransient(85)}
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: walk, failUntil: 99}
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})

	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err=%v, want the quota pause", err)
	}
	if !slices.Equal(walked.Ends, walk.Wall.Ends) {
		t.Errorf("checkpoint saw wall %v, want %v: the reset evidence must come from the families this walk met", walked, walk.Wall)
	}
}

func TestWalkError_KeepsTheWrappedErrorsTextAndIdentity(t *testing.T) {
	t.Parallel()

	inner := wrapTransient(85)
	walk := WalkError{Wall: Wall{}.Observe(FamilyEnd{CLI: "claude-tmux"}), Err: inner}

	if walk.Error() != inner.Error() || !errors.Is(walk, inner) {
		t.Errorf("WalkError text=%q is=%v, want the wrapped error unchanged: the walk is metadata, not a new cause", walk.Error(), errors.Is(walk, inner))
	}
}

func TestPauseForQuota_AMixedWallReadsAsTheFactsOfEachFamily(t *testing.T) {
	t.Parallel()
	c, got := recordingCenter()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))
	cr := &cycleRun{o: o, ctx: context.Background(), cycle: 2174, req: CycleRequest{ProjectRoot: t.TempDir()},
		cs: CycleState{Phase: string(PhaseAudit), WorkspacePath: t.TempDir(), RunID: "run-2174"}}
	wall := Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "auth_recheck"}).Observe(FamilyEnd{CLI: "agy", Pattern: "rate_limit"})

	err := cr.pauseForQuota(PhaseAudit, PhaseResponse{Phase: string(PhaseAudit)}, 2, WalkError{Wall: wall, Err: wrapTransient(85)})

	want := "claude needs a login; agy is on quota, reset unknown"
	if !errors.Is(err, ErrAllFamiliesExhausted) || !strings.Contains(err.Error(), want) || strings.Contains(err.Error(), "quota wall") {
		t.Errorf("the pause text must render the facts and name no cause of its own: %v", err)
	}
	if !strings.Contains(err.Error(), "open claude and run /login") {
		t.Errorf("the pause text must carry the next step: %v", err)
	}
	paused := eventsOfKind(*got, signalcenter.KindQuotaPaused)
	if len(paused) != 1 || !strings.Contains(paused[0].Reason, want) || paused[0].Fields["stop_reason"] != "login-required" {
		t.Errorf("the quota.paused event carries the clauses and the stop reason: %+v", paused)
	}
}

func TestErrAllFamiliesExhausted_NamesNoCause(t *testing.T) {
	t.Parallel()
	if msg := ErrAllFamiliesExhausted.Error(); strings.Contains(msg, "quota") || !strings.Contains(msg, "ended at a wall") {
		t.Errorf("the sentinel states a wall, not its cause: %q", msg)
	}
}

func TestPauseForQuota_AWallWithNoQuotaRowFiresNoQuotaPaused(t *testing.T) {
	t.Parallel()
	cases := map[string]Wall{
		"all login":    Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "auth_recheck"}),
		"other rows":   Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "bridge_crash"}),
		"empty cause":  Wall{}.Observe(FamilyEnd{CLI: "claude-tmux"}),
		"quota beside": Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "auth_recheck"}).Observe(FamilyEnd{CLI: "agy", Pattern: "rate_limit"}),
		"no rows":      {},
	}
	wantFired := map[string]int{"quota beside": 1, "no rows": 1, "empty cause": 1}
	for name, wall := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, got := recordingCenter()
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))
			cr := &cycleRun{o: o, ctx: context.Background(), cycle: 2174, req: CycleRequest{ProjectRoot: t.TempDir()},
				cs: CycleState{Phase: string(PhaseAudit), WorkspacePath: t.TempDir(), RunID: "run-2174"}}

			err := cr.pauseForQuota(PhaseAudit, PhaseResponse{Phase: string(PhaseAudit)}, 2, WalkError{Wall: wall, Err: wrapTransient(85)})

			if !errors.Is(err, ErrAllFamiliesExhausted) {
				t.Fatalf("err=%v, want the pause", err)
			}
			if n := len(eventsOfKind(*got, signalcenter.KindQuotaPaused)); n != wantFired[name] {
				t.Errorf("quota.paused events=%d, want %d: a wall with no quota row must not read as quota", n, wantFired[name])
			}
		})
	}
}

func TestRetryPhaseRunner_AResumedPauseCarriesTheMergedWall(t *testing.T) {
	t.Parallel()
	login := Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "auth_recheck"})
	quota := Wall{}.Observe(FamilyEnd{CLI: "agy", Pattern: "rate_limit"})
	runner := &dispatchScriptRunnerE4{name: "scout", steps: []func() (PhaseResponse, error){
		dispatchStepE4(PhaseResponse{}, WalkError{Wall: login, Err: wrapTransient(85)}),
		dispatchStepE4(PhaseResponse{}, WalkError{Wall: quota, Err: wrapTransient(85)}),
	}}
	cr := dispatchRunE4(t, &fakeLedger{}, runner, PhaseScout, 2)

	_, _, err := cr.retryPhaseRunner(PhaseScout, PhaseRequest{}, cr.resumeDispatchRetryOpts())

	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err=%v, want the pause", err)
	}
	got := walkedWall(err)
	if !got.HasLogin() || !got.HasQuota() || len(got.Ends) != 2 {
		t.Errorf("wall=%+v, want the login of attempt one and the quota of attempt two: the last attempt alone loses the login", got)
	}
}
