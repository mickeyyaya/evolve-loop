package core

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

type recordingRungHandler struct {
	res   RungResult
	err   error
	calls []RungInput
}

func (h *recordingRungHandler) Handle(_ context.Context, in RungInput) (RungResult, error) {
	h.calls = append(h.calls, in)
	return h.res, h.err
}

func auditFixRoundRun(t *testing.T, stage string, h *recordingRungHandler) (*cycleRun, *bytes.Buffer) {
	t.Helper()
	var errOut bytes.Buffer
	opts := []Option{WithStderr(&errOut)}
	if h != nil {
		opts = append(opts, WithRungHandler(retryladder.RungAuditFix, h))
	}
	cr, _ := auditFixRun(t, stage, prescribedSmallFix, opts...)
	return cr, &errOut
}

func TestAuditFixRound_APassedPatchRunsTheAuditAgainAndNoRepairRound(t *testing.T) {
	t.Parallel()
	for _, reason := range []string{"passed", "held_proposal"} {
		t.Run(reason, func(t *testing.T) {
			h := &recordingRungHandler{res: RungResult{Handled: true, Reason: reason}}
			cr, _ := auditFixRoundRun(t, policy.RetryLadderEnforce, h)

			next, why := cr.auditFixRound(PhaseBuild, repairGrant, nil)

			if next != PhaseAudit || !strings.HasPrefix(why, auditFixReasonPrefix+reason) {
				t.Fatalf("auditFixRound = %s %q, want a second audit on the patched tree", next, why)
			}
			if len(h.calls) != 1 {
				t.Fatalf("the handler ran %d times, want 1", len(h.calls))
			}
			in := h.calls[0]
			if in.Rung != retryladder.RungAuditFix || !in.Choice.Choose || in.Cycle != 2035 || in.Workspace != cr.cs.WorkspacePath ||
				in.Worktree != cr.cs.ActiveWorktree || len(in.Failure.Prescription) != 1 {
				t.Fatalf("RungInput = %+v, want the chosen rung with the failure block and the cycle dirs", in)
			}
			attempts := cr.cs.AuditRepairAttempts
			consumeAuditRepairGrant(&cr.cs, why)
			if cr.cs.AuditRepairAttempts != attempts || !cr.o.sm.CanTransition(PhaseAudit, next) || cr.o.transitionLegal(PhaseAudit, next) {
				t.Fatalf("the re-audit spent a repair attempt or is not a decision-only legal edge")
			}
		})
	}
}

func TestAuditFixRound_AnEscalationRunsTheRepairRoundOfToday(t *testing.T) {
	t.Parallel()
	h := &recordingRungHandler{res: RungResult{Handled: true, Escalate: true, Reason: "escalate"}}
	cr, _ := auditFixRoundRun(t, policy.RetryLadderEnforce, h)

	next, why := cr.auditFixRound(PhaseBuild, repairGrant, nil)

	if next != PhaseBuild || why != repairGrant || len(h.calls) != 1 {
		t.Fatalf("auditFixRound = %s %q after %d calls, want the repair round of today", next, why, len(h.calls))
	}
	if cr.cs.Ladder == nil || cr.cs.Ladder.AuditFix != auditFixChosen {
		t.Fatalf("ladder = %+v, want the audit_fix allowance used", cr.cs.Ladder)
	}
}

func TestAuditFixRound_AFailedEscalationSealsTheCycleHandedOff(t *testing.T) {
	t.Parallel()
	h := &recordingRungHandler{res: RungResult{Handled: true, Stop: true, Reason: "audit_fix_exhausted"}}
	cr, _ := auditFixRoundRun(t, policy.RetryLadderEnforce, h)
	cr.cs.Ladder = &LadderState{Rung: string(retryladder.R0), AuditFix: auditFixChosen}
	declined := auditDeclineReasonPrefix + "the repair budget is spent"

	next, why := cr.auditFixRound(PhaseRetro, declined, nil)

	if next != PhaseRetro || !strings.HasPrefix(why, repairHandoffReasonPrefix+"audit_fix_exhausted: ") || !strings.HasSuffix(why, "salvage "+cr.cs.ActiveWorktree) {
		t.Fatalf("auditFixRound = %s %q, want a hand-off with audit_fix_exhausted and the salvage pointer", next, why)
	}
	if len(h.calls) != 1 || h.calls[0].Choice.Choose {
		t.Fatalf("calls = %+v, want one call with no new choice", h.calls)
	}
	var result CycleResult
	if !cr.sealHandedOff(&result, nil, why) || result.TerminationReason != CycleTerminationHandedOff {
		t.Fatalf("the stop did not seal the cycle handed-off: %+v", result)
	}
}

func TestAuditFixRound_ShadowRecordsAndKeepsTheFlow(t *testing.T) {
	t.Parallel()
	h := &recordingRungHandler{res: RungResult{Reason: "shadow"}}
	cr, _ := auditFixRoundRun(t, policy.RetryLadderShadow, h)

	next, why := cr.auditFixRound(PhaseTDD, repairGrant, nil)

	if next != PhaseTDD || why != repairGrant {
		t.Fatalf("auditFixRound = %s %q, want the flow of today at shadow", next, why)
	}
	if len(h.calls) != 1 || !h.calls[0].Choice.Choose || cr.cs.Ladder.AuditFix != auditFixWould {
		t.Fatalf("calls %d ladder %+v, want one shadow record of the would choice", len(h.calls), cr.cs.Ladder)
	}
}

func TestAuditFixRound_KeepsTheFlowWhenNothingCanRun(t *testing.T) {
	t.Parallel()
	exhausted := RungResult{Handled: true, Stop: true, Reason: "audit_fix_exhausted"}
	for name, tc := range map[string]struct {
		handler *recordingRungHandler
		branch  Phase
		reason  string
		sysFail *SystemFailureSignal
		used    bool
		calls   int
		warn    string
	}{
		"no handler":        {nil, PhaseBuild, repairGrant, nil, false, 0, ""},
		"a handler error":   {&recordingRungHandler{err: errors.New("bridge down")}, PhaseBuild, repairGrant, nil, false, 1, "bridge down"},
		"a not chosen run":  {&recordingRungHandler{res: RungResult{Reason: "not_chosen"}}, PhaseBuild, repairGrant, nil, false, 1, ""},
		"a system halt":     {&recordingRungHandler{res: exhausted}, PhaseRetro, "audit-fail-floor: infra", &SystemFailureSignal{Halt: true}, true, 0, ""},
		"a fixability stop": {&recordingRungHandler{res: exhausted}, PhaseRetro, repairHandoffReasonPrefix + "handoff_blocker: blockers H1", nil, true, 0, ""},
		"no repair offered": {&recordingRungHandler{}, PhaseRetro, auditDeclineReasonPrefix + "budget spent", nil, false, 0, ""},
	} {
		t.Run(name, func(t *testing.T) {
			cr, errOut := auditFixRoundRun(t, policy.RetryLadderEnforce, tc.handler)
			if tc.used {
				cr.cs.Ladder = &LadderState{Rung: string(retryladder.R0), AuditFix: auditFixChosen}
			}

			next, why := cr.auditFixRound(tc.branch, tc.reason, tc.sysFail)

			calls := 0
			if tc.handler != nil {
				calls = len(tc.handler.calls)
			}
			if next != tc.branch || why != tc.reason || calls != tc.calls {
				t.Fatalf("auditFixRound = %s %q after %d calls, want the flow of today after %d", next, why, calls, tc.calls)
			}
			if !strings.Contains(errOut.String(), tc.warn) || (tc.warn != "" && !strings.Contains(errOut.String(), "WARN")) {
				t.Fatalf("stderr = %q, want %q", errOut.String(), tc.warn)
			}
		})
	}
}

func TestAuditFixBranch_MapsEachResult(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		res  RungResult
		want Phase
	}{
		{RungResult{Handled: true, Reason: "passed"}, PhaseAudit},
		{RungResult{Handled: true, Escalate: true}, PhaseBuild},
		{RungResult{Handled: true, Stop: true}, PhaseRetro},
		{RungResult{Reason: "no_review_seat"}, PhaseBuild},
	} {
		if got, _ := auditFixBranch(tc.res, PhaseBuild, repairGrant, "/wt"); got != tc.want {
			t.Errorf("auditFixBranch(%+v) = %s, want %s", tc.res, got, tc.want)
		}
	}
}

type twoRoundAuditRunner struct {
	t     *testing.T
	calls int
}

func (r *twoRoundAuditRunner) Name() string { return string(PhaseAudit) }

func (r *twoRoundAuditRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.calls++
	if r.calls > 1 {
		return PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
	}
	report := "# Audit\n\n## Verdict\nFAIL\n\n" + phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL", prescribedSmallFix) + "\n"
	if err := os.WriteFile(filepath.Join(req.Workspace, "audit-report.md"), []byte(report), 0o644); err != nil {
		r.t.Fatal(err)
	}
	return PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictFAIL, ArtifactsDir: req.Workspace}, nil
}

func countPhase(phases []Phase, p Phase) int {
	n := 0
	for _, got := range phases {
		if got == p {
			n++
		}
	}
	return n
}

func TestRunCycle_APassedAuditFixShipsAfterASecondAuditWithNoRepairRound(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		handler *recordingRungHandler
		audits  int
		builds  int
	}{
		"registered": {&recordingRungHandler{res: RungResult{Handled: true, Reason: "passed"}}, 2, 1},
		"absent":     {nil, 2, 2},
	} {
		t.Run(name, func(t *testing.T) {
			wf := ladderWorkflow(policy.RetryLadderShadow)
			wf.RetryLadder.AuditFix.Stage = policy.RetryLadderEnforce
			opts := []Option{WithWorkflowConfig(wf), WithStderr(&bytes.Buffer{})}
			if tc.handler != nil {
				opts = append(opts, WithRungHandler(retryladder.RungAuditFix, tc.handler))
			}
			runners := buildRunners(nil)
			audit := &twoRoundAuditRunner{t: t}
			runners[PhaseAudit] = audit
			o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 2034}}, &fakeLedger{}, runners, opts...)

			res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "goal-2035"})

			if err != nil {
				t.Fatalf("RunCycle: %v", err)
			}
			if audit.calls != tc.audits || countPhase(res.PhasesRun, PhaseBuild) != tc.builds || res.PhasesRun[len(res.PhasesRun)-1] != PhaseShip {
				t.Fatalf("phases = %v after %d audits, want %d audits, %d builds and a ship", res.PhasesRun, audit.calls, tc.audits, tc.builds)
			}
		})
	}
}

func TestRegisteredRungs_NamesEachRegisteredHandler(t *testing.T) {
	t.Parallel()
	if got := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, nil).RegisteredRungs(); len(got) != 0 {
		t.Fatalf("RegisteredRungs = %v, want none", got)
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, nil, WithRungHandler(retryladder.RungAuditFix, &recordingRungHandler{}), WithRungHandler(retryladder.R1, &recordingRungHandler{}))
	if got := o.RegisteredRungs(); len(got) != 2 || got[0] != retryladder.R1 || got[1] != retryladder.RungAuditFix {
		t.Fatalf("RegisteredRungs = %v, want R1 and audit_fix in order", got)
	}
}
