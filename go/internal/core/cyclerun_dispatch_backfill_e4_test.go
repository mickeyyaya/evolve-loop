package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurediag"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type dispatchScriptRunnerE4 struct {
	name  string
	steps []func() (PhaseResponse, error)
	calls int
}

func (r *dispatchScriptRunnerE4) Name() string { return r.name }
func (r *dispatchScriptRunnerE4) Run(context.Context, PhaseRequest) (PhaseResponse, error) {
	i := r.calls
	r.calls++
	if i >= len(r.steps) {
		i = len(r.steps) - 1
	}
	return r.steps[i]()
}

func dispatchStepE4(resp PhaseResponse, err error) func() (PhaseResponse, error) {
	return func() (PhaseResponse, error) { return resp, err }
}

func dispatchRunE4(t *testing.T, ledger Ledger, runner PhaseRunner, phase Phase, attempts int, opts ...Option) *cycleRun {
	t.Helper()
	runners := buildRunners(nil)
	runners[phase] = runner
	cfg := policy.Policy{}.RetryConfig()
	cfg.PhaseMaxAttempts = attempts
	cfg.RetryBackoffBaseS = 0
	opts = append([]Option{WithRetryConfig(cfg)}, opts...)
	o := NewOrchestrator(&fakeStorage{}, ledger, runners, opts...)
	return &cycleRun{
		o: o, ctx: context.Background(), cycle: 5, req: CycleRequest{ProjectRoot: t.TempDir()},
		cs:          CycleState{CycleID: 5, WorkspacePath: t.TempDir(), RunID: "run-5"},
		retryConfig: o.retryConfig, envSnap: map[string]string{}, ctxSnap: map[string]string{},
		current: PhaseScout,
	}
}

func TestDispatch_AReconciledTimeoutShipsOnTheAgentsVerdictAndLeavesAnAuditRow(t *testing.T) {
	cases := []struct {
		name       string
		failLedger bool
		wantWarn   bool
	}{
		{"the row lands", false, false},
		{"a lost row warns and the verdict still ships", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ledger := &kindFailingLedgerE4{failKinds: map[string]bool{}}
			if tc.failLedger {
				ledger.failKinds["reconciled_timeout"] = true
			}
			runner := &dispatchScriptRunnerE4{name: "scout", steps: []func() (PhaseResponse, error){
				dispatchStepE4(PhaseResponse{Phase: "scout", Verdict: VerdictPASS, Reconciled: true}, nil),
			}}
			cr := dispatchRunE4(t, ledger, runner, PhaseScout, 2)

			var dr dispatchResult
			var act loopAction
			var err error
			log := captureStderr(t, func() { dr, act, err = cr.dispatch(PhaseScout) })

			if err != nil || act != loopNext || dr.resp.Verdict != VerdictPASS || runner.calls != 1 {
				t.Fatalf("act=%v err=%v verdict=%q calls=%d, want loopNext on the agent's PASS after one call", act, err, dr.resp.Verdict, runner.calls)
			}
			warned := strings.Contains(log, "WARN reconciled_timeout ledger append: forced reconciled_timeout append fail")
			if warned != tc.wantWarn {
				t.Fatalf("warned=%v want %v:\n%s", warned, tc.wantWarn, log)
			}
			rows := 0
			for _, e := range ledger.entries {
				if e.Kind == "reconciled_timeout" {
					rows++
					if e.ExitCode != failurediag.ExitCodeArtifactTimeout || e.Role != "scout" || e.Cycle != 5 {
						t.Fatalf("row = %+v, want exit %d for scout cycle 5", e, failurediag.ExitCodeArtifactTimeout)
					}
				}
			}
			if wantRows := map[bool]int{false: 1, true: 0}[tc.failLedger]; rows != wantRows {
				t.Fatalf("reconciled_timeout rows = %d, want %d", rows, wantRows)
			}
		})
	}
}

func TestDispatch_ANonReconciledPassWritesNoReconciledRow(t *testing.T) {
	ledger := &kindFailingLedgerE4{failKinds: map[string]bool{}}
	runner := &dispatchScriptRunnerE4{name: "scout", steps: []func() (PhaseResponse, error){
		dispatchStepE4(PhaseResponse{Phase: "scout", Verdict: VerdictPASS}, nil),
	}}
	cr := dispatchRunE4(t, ledger, runner, PhaseScout, 2)

	_, act, err := cr.dispatch(PhaseScout)

	if err != nil || act != loopNext {
		t.Fatalf("act=%v err=%v", act, err)
	}
	for _, k := range ledger.kinds() {
		if k == "reconciled_timeout" {
			t.Fatal("a plain PASS must not claim a reconciled timeout")
		}
	}
}

func TestDispatch_ABestEffortObserverThatFailsAfterShipIsSkippedNotFatal(t *testing.T) {
	catalog, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{{Name: "post-ship-monitor", Optional: true, After: "ship", Role: string(phasespec.RoleControl)}})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	policyErr := errors.New("memo envelope rejected: tier unsupported")
	cases := []struct {
		name      string
		shipped   bool
		wantSkip  bool
		wantFatal bool
	}{
		{"shipped cycle skips the observer", true, true, false},
		{"an unshipped cycle keeps the failure fatal", false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ledger := &kindFailingLedgerE4{failKinds: map[string]bool{}}
			runner := &dispatchScriptRunnerE4{name: "post-ship-monitor", steps: []func() (PhaseResponse, error){dispatchStepE4(PhaseResponse{}, policyErr)}}
			cr := dispatchRunE4(t, ledger, runner, Phase("post-ship-monitor"), 1, WithCatalog(catalog))
			cr.cs.Shipped = tc.shipped

			var dr dispatchResult
			var act loopAction
			var derr error
			log := captureStderr(t, func() { dr, act, derr = cr.dispatch(Phase("post-ship-monitor")) })

			if tc.wantFatal {
				var clf *ErrCycleLevelFailure
				if act != loopAbort || !errors.As(derr, &clf) || clf.Phase != "post-ship-monitor" {
					t.Fatalf("act=%v err=%v, want a cycle-level abort naming the observer", act, derr)
				}
				return
			}
			if derr != nil || act != loopNext || dr.resp.Verdict != VerdictSKIPPED {
				t.Fatalf("act=%v err=%v verdict=%q, want loopNext with SKIPPED", act, derr, dr.resp.Verdict)
			}
			if !strings.Contains(log, "post_ship_observer_skip") || !strings.Contains(strings.Join(ledger.kinds(), ","), "post_ship_observer_skip") {
				t.Fatalf("the skip must be loud and audited; kinds=%v\n%s", ledger.kinds(), log)
			}
		})
	}
}

func TestDispatch_ANonCanonicalVerdictAtTheRetryCapDegradesOnlyANonFloorPhase(t *testing.T) {
	catalog, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{{Name: "amplify-tests", Optional: true, After: "build"}})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	bogus := dispatchStepE4(PhaseResponse{Phase: "amplify-tests", Verdict: "MAYBE"}, nil)
	cases := []struct {
		name      string
		completed []string
		wantSkip  bool
	}{
		{"a floor verdict already exists", []string{"audit"}, true},
		{"no floor verdict yet", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ledger := &kindFailingLedgerE4{failKinds: map[string]bool{}}
			runner := &dispatchScriptRunnerE4{name: "amplify-tests", steps: []func() (PhaseResponse, error){bogus}}
			cr := dispatchRunE4(t, ledger, runner, Phase("amplify-tests"), 1, WithCatalog(catalog))
			cr.cs.CompletedPhases = tc.completed

			var dr dispatchResult
			var act loopAction
			var derr error
			log := captureStderr(t, func() { dr, act, derr = cr.dispatch(Phase("amplify-tests")) })

			if !tc.wantSkip {
				var clf *ErrCycleLevelFailure
				if act != loopAbort || !errors.As(derr, &clf) || !strings.Contains(derr.Error(), `non-canonical verdict "MAYBE"`) {
					t.Fatalf("act=%v err=%v, want a fatal non-canonical verdict", act, derr)
				}
				return
			}
			if derr != nil || act != loopNext || dr.resp.Verdict != VerdictSKIPPED {
				t.Fatalf("act=%v err=%v verdict=%q, want a SKIPPED advance", act, derr, dr.resp.Verdict)
			}
			if !strings.Contains(log, "non-floor phase degrading to SKIPPED") || !strings.Contains(strings.Join(ledger.kinds(), ","), ledgerKindContractExhaustionSkip) {
				t.Fatalf("the degrade must be loud and audited; kinds=%v\n%s", ledger.kinds(), log)
			}
		})
	}
}

func TestDispatch_ARetryRowThatCannotBeWrittenWarnsAndTheRelaunchStillHappens(t *testing.T) {
	good := PhaseResponse{Phase: "scout", Verdict: VerdictPASS}
	cases := []struct {
		name  string
		first func() (PhaseResponse, error)
	}{
		{"a non-canonical verdict", dispatchStepE4(PhaseResponse{Phase: "scout", Verdict: "MAYBE"}, nil)},
		{"a transient bridge timeout", dispatchStepE4(PhaseResponse{}, infraTimeoutErr("scout"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ledger := &kindFailingLedgerE4{failKinds: map[string]bool{"phase_retry": true}}
			runner := &dispatchScriptRunnerE4{name: "scout", steps: []func() (PhaseResponse, error){tc.first, dispatchStepE4(good, nil)}}
			cr := dispatchRunE4(t, ledger, runner, PhaseScout, 2)

			var dr dispatchResult
			var act loopAction
			var err error
			log := captureStderr(t, func() { dr, act, err = cr.dispatch(PhaseScout) })

			if err != nil || act != loopNext || dr.resp.Verdict != VerdictPASS || dr.attemptCount != 2 || runner.calls != 2 {
				t.Fatalf("act=%v err=%v verdict=%q attempts=%d calls=%d, want the second attempt to ship", act, err, dr.resp.Verdict, dr.attemptCount, runner.calls)
			}
			if !strings.Contains(log, "WARN phase_retry ledger append: forced phase_retry append fail") {
				t.Fatalf("a lost retry row must warn:\n%s", log)
			}
		})
	}
}
