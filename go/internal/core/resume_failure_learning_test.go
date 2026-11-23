package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type resumedEnd struct {
	o         *Orchestrator
	root      string
	workspace string
	phase     Phase
	retro     *fakeRunner
}

func resumedUntypedEnd(t *testing.T, phase Phase, dispatch func(next Phase) (PhaseResponse, int, error)) resumedEnd {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	st := &fakeStorage{
		state:      State{LastCycleNumber: 9},
		cycleState: CycleState{CycleID: 9, WorkspacePath: ws, ActiveWorktree: t.TempDir(), CompletedPhases: []string{string(PhaseScout), string(PhaseTriage)}},
	}
	runners := buildRunners(nil)
	retro := runners[PhaseRetro].(*fakeRunner)
	o := NewOrchestrator(st, &fakeLedger{}, runners)
	o.resumeDispatch = func(_ *cycleRun, next Phase, _ PhaseRequest, _ retryOpts) (map[string]bool, PhaseResponse, int, error) {
		resp, attempts, err := dispatch(next)
		return nil, resp, attempts, err
	}
	return resumedEnd{o: o, root: root, workspace: ws, phase: phase, retro: retro}
}

func (e resumedEnd) resume() (CycleResult, error) {
	return e.o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: e.root, GoalHash: "g"}, &ResumePoint{Phase: string(e.phase), CycleID: 9})
}

func rejectedReviewEnd(t *testing.T) resumedEnd {
	t.Helper()
	o, builder, _, root := resumedConvergenceHarness(t, PhaseBuild, [][]string{findings(1, "a"), findings(1, "a")})
	builder.failErr, builder.failUntil = errors.New("builder crashed"), 99
	o.resumeDispatch = func(_ *cycleRun, next Phase, _ PhaseRequest, _ retryOpts) (map[string]bool, PhaseResponse, int, error) {
		return nil, PhaseResponse{Phase: string(next), Verdict: VerdictPASS}, 1, nil
	}
	return resumedEnd{o: o, root: root, workspace: filepath.Join(root, ".evolve", "runs", "cycle-9"), phase: PhaseBuild, retro: o.runners[PhaseRetro].(*fakeRunner)}
}

func TestRunCycleFromPhase_AnUntypedResumedEndRunsTheRetrospective(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		end       func(t *testing.T) resumedEnd
		wantDiag  string
		wantPhase Phase
	}{
		"a dispatch error": {
			end: func(t *testing.T) resumedEnd {
				return resumedUntypedEnd(t, PhaseAudit, func(Phase) (PhaseResponse, int, error) {
					return PhaseResponse{}, 1, errors.New("bridge exploded")
				})
			},
			wantDiag: "audit-failure-diag.json",
		},
		"a non-canonical verdict": {
			end: func(t *testing.T) resumedEnd {
				return resumedUntypedEnd(t, PhaseAudit, func(next Phase) (PhaseResponse, int, error) {
					return PhaseResponse{Phase: string(next), Verdict: "BOGUS"}, 1, nil
				})
			},
			wantDiag: "audit-failure-diag.json",
		},
		"a correction that fails to dispatch": {end: rejectedReviewEnd},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			end := tc.end(t)

			result, err := end.resume()

			var cycleLevel *ErrCycleLevelFailure
			if err == nil || errors.As(err, &cycleLevel) {
				t.Fatalf("err = %v, want the raw untyped error so the batch still stops", err)
			}
			if end.retro.calls != 1 || result.RetroDecision == "" {
				t.Errorf("retro runner calls = %d, RetroDecision = %q; want the retrospective dispatched once and its decision recorded", end.retro.calls, result.RetroDecision)
			}
			if tc.wantDiag != "" {
				if _, serr := os.Stat(filepath.Join(end.workspace, tc.wantDiag)); serr != nil {
					t.Errorf("the resumed end must write %s like a fresh cycle: %v", tc.wantDiag, serr)
				}
			}
		})
	}
}

func TestRunCycleFromPhase_AResumedEndThatTheFreshPathDoesNotLearnFromDispatchesNoRetrospective(t *testing.T) {
	t.Parallel()

	t.Run("no runner registered", func(t *testing.T) {
		t.Parallel()
		end := resumedUntypedEnd(t, PhaseAudit, func(Phase) (PhaseResponse, int, error) { return PhaseResponse{}, 0, nil })
		delete(end.o.runners, PhaseAudit)

		result, err := end.resume()

		if !errors.Is(err, ErrPhaseInvalid) || end.retro.calls != 0 || result.RetroDecision != "" {
			t.Errorf("err = %v, retro calls = %d, decision = %q; want the error and no retrospective", err, end.retro.calls, result.RetroDecision)
		}
	})
	t.Run("ledger append failure", func(t *testing.T) {
		t.Parallel()
		end := resumedUntypedEnd(t, PhaseAudit, func(next Phase) (PhaseResponse, int, error) {
			return PhaseResponse{Phase: string(next), Verdict: VerdictPASS}, 1, nil
		})
		end.o.ledger = &fakeLedger{failOnAppend: true}

		result, err := end.resume()

		if err == nil || end.retro.calls != 0 || result.RetroDecision != "" {
			t.Errorf("err = %v, retro calls = %d, decision = %q; want the error and no retrospective", err, end.retro.calls, result.RetroDecision)
		}
	})
	t.Run("quota wall", func(t *testing.T) {
		t.Parallel()
		end := resumedUntypedEnd(t, PhaseAudit, func(Phase) (PhaseResponse, int, error) {
			return PhaseResponse{}, 1, ErrAllFamiliesExhausted
		})

		_, err := end.resume()

		if !errors.Is(err, ErrAllFamiliesExhausted) || end.retro.calls != 0 {
			t.Errorf("err = %v, retro calls = %d; want the pause with no retrospective", err, end.retro.calls)
		}
	})
}

func TestRunCycleFromPhase_AResumedRefreshAbortRunsTheRetrospectiveAndStaysRaw(t *testing.T) {
	t.Parallel()

	f := newStaleExplanationFixture(t)
	writer := postBuildWriter{phase: PhaseTDD, effect: f.dropsBuildReport}
	rig := f.resumeRig(2, writer)
	retro := rig.o.runners[PhaseRetro].(*fakeRunner)

	result, err := rig.resumeAt(f, writer.phase)

	var cycleLevel *ErrCycleLevelFailure
	if err == nil || errors.As(err, &cycleLevel) {
		t.Fatalf("err = %v, want the raw refresh abort that stops the batch", err)
	}
	if retro.calls != 1 || result.RetroDecision == "" {
		t.Errorf("retro runner calls = %d, RetroDecision = %q; want the retrospective dispatched once", retro.calls, result.RetroDecision)
	}
}

func TestRunCycleFromPhase_AResumedDispatchErrorAsksTheAdvisor(t *testing.T) {
	t.Parallel()
	end := resumedUntypedEnd(t, PhaseAudit, func(Phase) (PhaseResponse, int, error) {
		return PhaseResponse{}, 1, wrapTimeout()
	})
	writeEscalation(t, end.workspace, "audit", "⚠ some never-seen fatal pane state, definitely novel")
	adviser := &fakeAdviser{advice: &recovery.FailureAdvice{Cause: "solar_flare", PaneSubstr: "never-seen fatal pane", Justification: "j"}}
	end.o.failureAdviser = adviser
	end.o.cfg.PhaseRecovery = config.StageEnforce

	_, err := end.resume()

	if err == nil || adviser.calls != 1 {
		t.Errorf("err = %v, advisor calls = %d; want the failure and one advisor call, as on a fresh cycle", err, adviser.calls)
	}
}

func TestRunCycleFromPhase_AResumedDispatchErrorNotesItsLoginWall(t *testing.T) {
	t.Parallel()
	center, got := recordingCenter()
	end := resumedUntypedEnd(t, PhaseAudit, func(Phase) (PhaseResponse, int, error) {
		return PhaseResponse{}, 1, walkOf(wrapTransient(81), FamilyEnd{CLI: "claude-tmux", Pattern: "auth_recheck"}, FamilyEnd{CLI: "codex"})
	})
	end.o.signals = center

	_, err := end.resume()

	events := eventsOfKind(*got, signalcenter.KindLoginRequired)
	if err == nil || len(events) != 1 || !strings.Contains(events[0].Reason, "claude needs a login") {
		t.Errorf("err = %v, events %+v; want one login-required event", err, events)
	}
}

func TestRunCycleFromPhase_AResumedFailureBeforeAnyDispatchCountsAsAttemptOne(t *testing.T) {
	t.Parallel()
	end := resumedUntypedEnd(t, PhaseAudit, func(Phase) (PhaseResponse, int, error) {
		return PhaseResponse{}, 0, errors.New("snapshot failed")
	})

	_, err := end.resume()

	if err == nil || end.retro.calls != 1 {
		t.Fatalf("err = %v, retro calls = %d; want the failure learned from once", err, end.retro.calls)
	}
	if got := end.retro.requests[0].Context["failure_attempt"]; got != "1" {
		t.Errorf("failure_attempt = %q, want 1", got)
	}
}
