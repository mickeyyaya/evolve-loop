package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type resumeHookRunnerE4 struct {
	name     string
	verdict  string
	hook     func(PhaseRequest)
	requests []PhaseRequest
}

func (r *resumeHookRunnerE4) Name() string { return r.name }
func (r *resumeHookRunnerE4) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.requests = append(r.requests, req)
	if r.hook != nil {
		r.hook(req)
	}
	verdict := r.verdict
	if verdict == "" {
		verdict = VerdictPASS
	}
	return PhaseResponse{Phase: r.name, Verdict: verdict, ArtifactsDir: req.Workspace}, nil
}

type resumeEndFailingStorageE4 struct{ *fakeStorage }

func (s resumeEndFailingStorageE4) WriteCycleState(ctx context.Context, cs CycleState) error {
	if cs.Phase == string(PhaseEnd) {
		return errors.New("terminal write refused")
	}
	return s.fakeStorage.WriteCycleState(ctx, cs)
}

func resumeRigE4(t *testing.T, workspace string, mutate func(*CycleState), opts ...Option) (*fakeStorage, map[Phase]PhaseRunner, *Orchestrator) {
	t.Helper()
	cs := CycleState{CycleID: 61, WorkspacePath: workspace}
	if mutate != nil {
		mutate(&cs)
	}
	st := &fakeStorage{state: State{LastCycleNumber: 0}, cycleState: cs}
	runners := buildRunners(nil)
	cfg := policy.Policy{}.RetryConfig()
	cfg.PhaseMaxAttempts = 1
	cfg.RetryBackoffBaseS = 0
	o := NewOrchestrator(st, &fakeLedger{}, runners, append([]Option{WithRetryConfig(cfg)}, opts...)...)
	return st, runners, o
}

func resumeFromE4(o *Orchestrator, phase Phase) (CycleResult, error) {
	return o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: "/nonexistent-e4-root"}, &ResumePoint{Phase: string(phase), CycleID: 61})
}

func TestResumeExecution_AVerdictWithNoSuccessorStopsTheResumeNamingTheCursor(t *testing.T) {
	t.Parallel()

	_, runners, o := resumeRigE4(t, t.TempDir(), nil)
	runners[PhaseAudit] = &resumeHookRunnerE4{name: "audit", verdict: VerdictSKIPPED}

	_, err := resumeFromE4(o, PhaseAudit)

	if !errors.Is(err, ErrTransitionInvalid) || !strings.Contains(err.Error(), "transition from audit:") {
		t.Fatalf("err = %v, want the unresolvable audit successor named", err)
	}
}

func TestResumeExecution_ABuildResumedWithoutASealedWorktreeContextIsRefused(t *testing.T) {
	t.Parallel()

	root, workspace := t.TempDir(), t.TempDir()
	if err := explanationdocs.Activate(explanationdocs.CycleBinding{ProjectRoot: root, Workspace: workspace, Cycle: 61, RunID: "run-e4", ContractVersion: explanationdocs.CurrentContractVersion}); err != nil {
		t.Fatalf("activate: %v", err)
	}
	_, runners, o := resumeRigE4(t, workspace, func(cs *CycleState) {
		cs.RunID = "run-e4"
		cs.ExplanationDocumentationVersion = explanationdocs.CurrentContractVersion
	})

	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root}, &ResumePoint{Phase: string(PhaseBuild), CycleID: 61})

	if err == nil || !strings.Contains(err.Error(), "resume seal Build explanation context: active Build explanation contract requires worktree and base SHA") {
		t.Fatalf("err = %v, want the seal refusal", err)
	}
	if calls := runners[PhaseBuild].(*fakeRunner).calls; calls != 0 {
		t.Fatalf("build ran %d times without a sealed context", calls)
	}
}

func TestResumeExecution_ANonCanonicalVerdictStopsTheResumeAndIsRecordedAsAFailedPhase(t *testing.T) {
	t.Parallel()

	_, runners, o := resumeRigE4(t, t.TempDir(), nil)
	runners[PhaseScout] = &resumeHookRunnerE4{name: "scout", verdict: "MAYBE"}

	res, err := resumeFromE4(o, PhaseScout)

	if err == nil || !strings.Contains(err.Error(), `phase scout returned non-canonical verdict "MAYBE"`) {
		t.Fatalf("err = %v, want the verdict named", err)
	}
	if res.FinalVerdict != VerdictFAIL {
		t.Fatalf("FinalVerdict = %q, want the abort recorded as FAIL", res.FinalVerdict)
	}
}

func TestResumeExecution_APhaseWhoseCompletionCannotBePersistedStopsTheResume(t *testing.T) {
	t.Parallel()

	st, runners, o := resumeRigE4(t, t.TempDir(), nil)
	runners[PhaseScout] = &resumeHookRunnerE4{name: "scout", hook: func(PhaseRequest) { st.failOnWriteCS = true }}

	_, err := resumeFromE4(o, PhaseScout)

	if err == nil || !strings.Contains(err.Error(), "write cycle-state post-scout") {
		t.Fatalf("err = %v, want the persist failure named", err)
	}
}

func TestResumeExecution_AHaltedAuditFailRecordsTheSystemFailureForTheBatchLoop(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	_, runners, o := resumeRigE4(t, ws, nil)
	runners[PhaseAudit] = &resumeHookRunnerE4{name: "audit", verdict: VerdictFAIL, hook: func(req PhaseRequest) {
		writeAuditWithFailure(t, req.Workspace, "FAIL", policy.CategoryInfraSystemic, "all CLI families exhausted")
	}}

	res, err := resumeFromE4(o, PhaseAudit)

	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if res.SystemFailure == nil || !res.SystemFailure.Halt {
		t.Fatalf("SystemFailure = %+v, want the audit's halting signal carried to the result", res.SystemFailure)
	}
}

func TestResumeExecution_ARepairBranchTheStateMachineForbidsAbortsTheResume(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		sm        *StateMachine
		wantAbort bool
	}{
		{"audit to tdd is a legal edge", NewStateMachine(), false},
		{"audit to tdd removed from the machine", recordSMWithoutEdgeE4(PhaseAudit, PhaseTDD), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			_, runners, o := resumeRigE4(t, ws, nil)
			o.sm = tc.sm
			runners[PhaseAudit] = &resumeHookRunnerE4{name: "audit", verdict: VerdictFAIL, hook: func(req PhaseRequest) {
				writeAuditWithFailure(t, req.Workspace, "FAIL", policy.CategoryCodeAuditFail, "H1")
			}}

			res, err := resumeFromE4(o, PhaseAudit)

			if tc.wantAbort {
				if err == nil || err.Error() != "audit→tdd not allowed by state machine" {
					t.Fatalf("err = %v, want the illegal edge named", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("resume: %v", err)
			}
			ranTDD := false
			for _, p := range res.PhasesRun {
				ranTDD = ranTDD || p == PhaseTDD
			}
			if !ranTDD {
				t.Fatalf("PhasesRun = %v, want the repair round to re-enter tdd", res.PhasesRun)
			}
		})
	}
}

func TestResumeExecution_ARetroFloorHaltIsCarriedToTheResult(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	writeAuditWithFailure(t, ws, "FAIL", policy.CategoryInfraSystemic, "all CLI families exhausted; systemic infrastructure teardown")
	_, runners, o := resumeRigE4(t, ws, nil)
	runners[PhaseRetro] = &resumeHookRunnerE4{name: "retro", verdict: VerdictFAIL}

	res, err := resumeFromE4(o, PhaseRetro)

	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if res.SystemFailure == nil || !res.SystemFailure.Halt {
		t.Fatalf("SystemFailure = %+v, want the retro floor's halting signal", res.SystemFailure)
	}
}

func TestResumeExecution_ARetroDecisionCarriesItsEnvironmentIntoTheRescheduledPhase(t *testing.T) {
	recordDecisionSeamE4(t, failureadapter.Decision{Action: failureadapter.ActionRetryWithFallback, Reason: "retry once", SetEnv: map[string]string{"EVOLVE_E4_FALLBACK": "1"}})
	_, runners, o := resumeRigE4(t, t.TempDir(), nil)
	tdd := &resumeHookRunnerE4{name: "tdd"}
	runners[PhaseTDD] = tdd

	if _, err := resumeFromE4(o, PhaseRetro); err != nil {
		t.Fatalf("resume: %v", err)
	}

	if len(tdd.requests) == 0 || tdd.requests[0].Env["EVOLVE_E4_FALLBACK"] != "1" {
		t.Fatalf("tdd requests = %+v, want the retro decision's environment on the rescheduled phase", tdd.requests)
	}
}

func TestResumeExecution_ARetroBranchTheStateMachineForbidsAbortsTheResume(t *testing.T) {
	recordDecisionSeamE4(t, failureadapter.Decision{Action: failureadapter.ActionRetryWithFallback, Reason: "retry once"})
	_, _, o := resumeRigE4(t, t.TempDir(), nil)
	o.sm = recordSMWithoutEdgeE4(PhaseRetro, PhaseTDD)

	_, err := resumeFromE4(o, PhaseRetro)

	if err == nil || err.Error() != "retro→tdd not allowed by state machine" {
		t.Fatalf("err = %v, want the illegal edge named", err)
	}
}

func TestResumeExecution_ACycleThatCannotBeFinalizedReturnsTheFinalizeError(t *testing.T) {
	t.Parallel()

	st, runners, o := resumeRigE4(t, t.TempDir(), nil)
	runners[PhaseShip] = &resumeHookRunnerE4{name: "ship", hook: func(PhaseRequest) { st.failOnWriteState = true }}

	_, err := resumeFromE4(o, PhaseShip)

	if err == nil || !strings.Contains(err.Error(), "forced WriteState fail") {
		t.Fatalf("err = %v, want the final state write failure", err)
	}
}

func TestResumeExecution_AFinishedCycleWhoseTerminalStateCannotBeWrittenReportsIt(t *testing.T) {
	t.Parallel()

	base := &fakeStorage{state: State{LastCycleNumber: 0}, cycleState: CycleState{CycleID: 61, WorkspacePath: t.TempDir()}}
	o := NewOrchestrator(resumeEndFailingStorageE4{base}, &fakeLedger{}, buildRunners(nil))

	_, err := resumeFromE4(o, PhaseShip)

	if err == nil || !strings.Contains(err.Error(), "resume terminal state: terminal write refused") {
		t.Fatalf("err = %v, want the terminal write failure", err)
	}
}

type resumeQuotaOnCorrectionRunnerE4 struct{ calls int }

func (r *resumeQuotaOnCorrectionRunnerE4) Name() string { return "scout" }
func (r *resumeQuotaOnCorrectionRunnerE4) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.calls++
	if r.calls > 1 {
		return PhaseResponse{}, wrapTransient(85)
	}
	return PhaseResponse{Phase: "scout", Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func TestResumeExecution_AQuotaWallHitWhileCorrectingAResumedDeliverablePausesTheResume(t *testing.T) {
	t.Parallel()

	_, runners, o := resumeRigE4(t, t.TempDir(), nil, WithReviewer(stubReviewer{result: ReviewResult{Approve: false, Reason: "deliverable incomplete"}}))
	runner := &resumeQuotaOnCorrectionRunnerE4{}
	runners[PhaseScout] = runner

	_, err := resumeFromE4(o, PhaseScout)

	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err = %v, want the resume paused on the quota wall", err)
	}
	if runner.calls != 2 {
		t.Fatalf("scout ran %d times, want the original plus one correction", runner.calls)
	}
}
