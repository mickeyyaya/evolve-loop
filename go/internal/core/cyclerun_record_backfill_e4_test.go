package core

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func recordSMWithoutEdgeE4(from, to Phase) *StateMachine {
	sm := NewStateMachine()
	delete(sm.allowed[from], to)
	return sm
}

func recordRunE4(t *testing.T, phase Phase, workspace string) *cycleRun {
	t.Helper()
	cr := dispatchRunE4(t, &fakeLedger{}, &countingPassRunnerE4{name: string(phase)}, phase, 1)
	if workspace != "" {
		cr.cs.WorkspacePath = workspace
	}
	return cr
}

func recordDecisionSeamE4(t *testing.T, decision failureadapter.Decision) {
	t.Helper()
	orig := failureDecide
	failureDecide = func([]failureadapter.Entry, failureadapter.Options) failureadapter.Decision { return decision }
	t.Cleanup(func() { failureDecide = orig })
}

func TestRecordAndBranch_AnAuditFailThatTheHostHaltsRecordsTheSystemFailureOnce(t *testing.T) {
	cases := []struct {
		name     string
		existing *SystemFailureSignal
		wantCat  string
		wantHalt bool
	}{
		{"no earlier signal is recorded as the cycle's", nil, policy.CategoryInfraSystemic, true},
		{"an earlier signal is never overwritten", &SystemFailureSignal{Category: "earlier", Evidence: "first"}, "earlier", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := auditFailFixture(t, policy.CategoryInfraSystemic, "all CLI families exhausted")
			cr := recordRunE4(t, PhaseAudit, ws)
			cr.result.SystemFailure = tc.existing

			act, err := cr.recordAndBranch(PhaseAudit, dispatchResult{resp: PhaseResponse{Phase: "audit", Verdict: VerdictFAIL}, attemptCount: 1})

			if err != nil || act != loopNext {
				t.Fatalf("act=%v err=%v", act, err)
			}
			sf := cr.result.SystemFailure
			if sf == nil || sf.Halt != tc.wantHalt || sf.Category != tc.wantCat {
				t.Fatalf("SystemFailure = %+v, want category %q halt=%v", sf, tc.wantCat, tc.wantHalt)
			}
			if cr.scheduledNext != "" {
				t.Fatalf("a halted audit goes to retro through the normal edge, not scheduledNext=%q", cr.scheduledNext)
			}
		})
	}
}

func TestRecordAndBranch_AnAuditRepairBranchTheStateMachineForbidsAbortsBeforeItIsScheduled(t *testing.T) {
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
			ws := auditFailFixture(t, policy.CategoryCodeAuditFail, "H1")
			cr := recordRunE4(t, PhaseAudit, ws)
			cr.o.sm = tc.sm

			act, err := cr.recordAndBranch(PhaseAudit, dispatchResult{resp: PhaseResponse{Phase: "audit", Verdict: VerdictFAIL}, attemptCount: 1})

			if tc.wantAbort {
				if act != loopAbort || err == nil || err.Error() != "audit→tdd not allowed by state machine" {
					t.Fatalf("act=%v err=%v, want the illegal edge named", act, err)
				}
				if cr.scheduledNext != "" {
					t.Fatalf("an illegal edge must not be scheduled: %q", cr.scheduledNext)
				}
				return
			}
			if act != loopNext || err != nil || cr.scheduledNext != PhaseTDD {
				t.Fatalf("act=%v err=%v scheduledNext=%q, want the repair round scheduled", act, err, cr.scheduledNext)
			}
		})
	}
}

func TestRecordAndBranch_ARetroDecisionCarriesItsEnvironmentIntoTheNextPhases(t *testing.T) {
	recordDecisionSeamE4(t, failureadapter.Decision{Action: failureadapter.ActionProceed, Reason: "one transient", SetEnv: map[string]string{"EVOLVE_E4_FALLBACK": "1", "EVOLVE_E4_TIER": "deep"}})
	cr := recordRunE4(t, PhaseRetro, "")
	cr.envSnap["EVOLVE_E4_KEEP"] = "kept"

	act, err := cr.recordAndBranch(PhaseRetro, dispatchResult{resp: PhaseResponse{Phase: "retro", Verdict: VerdictPASS}, attemptCount: 1})

	if err != nil || act != loopBreak {
		t.Fatalf("act=%v err=%v, a proceed decision ends the cycle", act, err)
	}
	want := map[string]string{"EVOLVE_E4_FALLBACK": "1", "EVOLVE_E4_TIER": "deep", "EVOLVE_E4_KEEP": "kept"}
	for k, v := range want {
		if cr.envSnap[k] != v {
			t.Fatalf("envSnap[%s] = %q, want %q (full: %v)", k, cr.envSnap[k], v, cr.envSnap)
		}
	}
	if !strings.Contains(cr.result.RetroDecision, "proceed: one transient") {
		t.Fatalf("RetroDecision = %q", cr.result.RetroDecision)
	}
}

func TestRecordAndBranch_ARetroBranchTheStateMachineForbidsAborts(t *testing.T) {
	cases := []struct {
		name      string
		sm        *StateMachine
		wantAbort bool
	}{
		{"retro to tdd is a legal edge", NewStateMachine(), false},
		{"retro to tdd removed from the machine", recordSMWithoutEdgeE4(PhaseRetro, PhaseTDD), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recordDecisionSeamE4(t, failureadapter.Decision{Action: failureadapter.ActionRetryWithFallback, Reason: "retry once"})
			cr := recordRunE4(t, PhaseRetro, "")
			cr.o.sm = tc.sm

			act, err := cr.recordAndBranch(PhaseRetro, dispatchResult{resp: PhaseResponse{Phase: "retro", Verdict: VerdictPASS}, attemptCount: 1})

			if tc.wantAbort {
				if act != loopAbort || err == nil || err.Error() != "retro→tdd not allowed by state machine" {
					t.Fatalf("act=%v err=%v, want the illegal edge named", act, err)
				}
				return
			}
			if act != loopNext || err != nil || cr.scheduledNext != PhaseTDD {
				t.Fatalf("act=%v err=%v scheduledNext=%q, want retro to schedule tdd", act, err, cr.scheduledNext)
			}
		})
	}
}

func TestRecordAndBranch_ADebuggerBranchTheStateMachineForbidsAborts(t *testing.T) {
	cases := []struct {
		name      string
		sm        *StateMachine
		wantAbort bool
	}{
		{"debugger to ship is a legal edge", NewStateMachine(), false},
		{"debugger to ship removed from the machine", recordSMWithoutEdgeE4(PhaseDebugger, PhaseShip), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cr := recordRunE4(t, PhaseDebugger, "")
			cr.o.sm = tc.sm
			resp := PhaseResponse{Phase: "debugger", Verdict: VerdictPASS, Signals: map[string]any{"debugger.action": "RESHIP"}}

			act, err := cr.recordAndBranch(PhaseDebugger, dispatchResult{resp: resp, attemptCount: 1})

			if tc.wantAbort {
				if act != loopAbort || err == nil || err.Error() != "debugger→ship not allowed by state machine" {
					t.Fatalf("act=%v err=%v, want the illegal edge named", act, err)
				}
				return
			}
			if act != loopNext || err != nil || cr.scheduledNext != PhaseShip {
				t.Fatalf("act=%v err=%v scheduledNext=%q, want RESHIP to schedule ship", act, err, cr.scheduledNext)
			}
		})
	}
}

func TestDecideAfterRetro_ABlockingDecisionEndsTheCycleNamingTheAction(t *testing.T) {
	for _, action := range []failureadapter.Action{failureadapter.ActionBlockCode, failureadapter.ActionBlockOperatorAction} {
		t.Run(string(action), func(t *testing.T) {
			recordDecisionSeamE4(t, failureadapter.Decision{Action: action, Reason: "stop here", SetEnv: map[string]string{"EVOLVE_E4_IGNORED": "1"}})
			cr := recordRunE4(t, PhaseRetro, "")

			next, env, reason, sig := cr.o.decideAfterRetro(cr.cs, VerdictFAIL, nil)

			if next != PhaseEnd || env != nil || sig != nil || reason != string(action)+": stop here" {
				t.Fatalf("next=%q env=%v reason=%q sig=%v, want the cycle ended with no environment carried", next, env, reason, sig)
			}
		})
	}
}
