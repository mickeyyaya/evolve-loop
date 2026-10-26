package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const auditLadderReason = "EGPS: red_count=1 [TestC2035_004]"

func auditRepairRun(t *testing.T, stage string, center *signalcenter.Center) *cycleRun {
	t.Helper()
	root, ws := t.TempDir(), t.TempDir()
	writeRepairProfile(t, root, "builder", repairBuilderProfile)
	writeRepairProfile(t, root, "tdd-engineer", `{"name":"tdd-engineer","model_tier_default":"balanced","model_tier_overrides":{"audit_retry_2plus":"deep"}}`)
	report := "# Audit\n\n## Verdict\nFAIL\n\n" + phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL",
		&phasecontract.FailureBlock{Class: "code-audit-fail", Defects: []string{auditLadderReason}}) + "\n"
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := []Option{WithWorkflowConfig(ladderWorkflow(stage))}
	if center != nil {
		opts = append(opts, WithSignalCenter(center))
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), opts...)
	return &cycleRun{o: o, cycle: 2035, req: CycleRequest{ProjectRoot: root}, cs: CycleState{
		CycleID: 2035, WorkspacePath: ws, AuditRepairActive: true, AuditRepairAttempts: 1, AuditFailReasons: []string{auditLadderReason},
	}}
}

func TestApplyDispatchPolicy_AuditRepairAsksTheLadderOncePerRound(t *testing.T) {
	t.Parallel()
	center, events := captureSignals()
	cr := auditRepairRun(t, policy.RetryLadderShadow, center)

	tdd, build := PhaseRequest{}, PhaseRequest{}
	cr.applyDispatchPolicy(PhaseTDD, &tdd)
	cr.applyDispatchPolicy(PhaseBuild, &build)

	decisions := eventsWithCode(*events, CodeLadderDecision)
	if len(decisions) != 1 || decisions[0].Fields["origin"] != originAuditRepair || decisions[0].Fields["class"] != string(retryladder.Capability) {
		t.Fatalf("a repair round asked the ladder %d times: %+v", len(decisions), decisions)
	}
	if tdd.ModelRoutingTier != "deep" || build.ModelRoutingTier != "deep" {
		t.Fatalf("shadow must keep the repair-round raise of today: tdd %q build %q", tdd.ModelRoutingTier, build.ModelRoutingTier)
	}
	if tdd.CorrectionDirective != "" || tdd.Ladder.LadderTier() != "" || tdd.ModelRoutingEffort != "" {
		t.Fatalf("shadow changed the re-entry dispatch: %+v", tdd)
	}

	cr.cs.AuditRepairAttempts = 2
	again := PhaseRequest{}
	cr.applyDispatchPolicy(PhaseTDD, &again)
	if n := len(eventsWithCode(*events, CodeLadderDecision)); n != 2 {
		t.Fatalf("the next repair round asked %d times in total, want 2", n)
	}
}

func TestApplyDispatchPolicy_EnforceRaisesTheRepairRoundWithAFreshDigest(t *testing.T) {
	t.Parallel()
	cr := auditRepairRun(t, policy.RetryLadderEnforce, nil)

	tdd, build := PhaseRequest{}, PhaseRequest{}
	cr.applyDispatchPolicy(PhaseTDD, &tdd)
	cr.applyDispatchPolicy(PhaseBuild, &build)

	if tdd.Ladder == nil || tdd.Ladder.Context != retryladder.ContextFresh || tdd.Ladder.Rung == retryladder.R0 {
		t.Fatalf("the re-entry try is not a fresh raised try: %+v", tdd.Ladder)
	}
	if !strings.Contains(tdd.CorrectionDirective, auditLadderReason) {
		t.Fatalf("the re-entry digest lacks the audit failure:\n%s", tdd.CorrectionDirective)
	}
	if tdd.ModelRoutingTier != "" {
		t.Fatalf("at enforce the ladder, not the profile override, raises the repair round: tier %q", tdd.ModelRoutingTier)
	}
	if build.Ladder == nil || build.Ladder.Rung != tdd.Ladder.Rung || build.Ladder.Context == retryladder.ContextFresh || build.CorrectionDirective != "" {
		t.Fatalf("the build of the same round must keep the rung without a second digest: %+v %q", build.Ladder, build.CorrectionDirective)
	}
}

func TestMaybeRemediate_KeepsTheRungAtEnforceAndClearsItAtShadow(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		stage    string
		wantRung retryladder.Rung
	}{
		{policy.RetryLadderEnforce, retryladder.R2},
		{policy.RetryLadderShadow, ""},
	} {
		t.Run(tc.stage, func(t *testing.T) {
			builder := &ladderProbe{escalationProbe: &escalationProbe{phase: "build"}}
			gate := &ladderProbe{escalationProbe: &escalationProbe{phase: "coverage-gate"}}
			runners := buildRunners(nil)
			runners[PhaseBuild] = builder
			wf := ladderWorkflow(tc.stage)
			wf.RemediablePhases = []string{"coverage-gate"}
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorkflowConfig(wf))
			cr := &cycleRun{o: o, ctx: context.Background(), cycle: 5, req: CycleRequest{ProjectRoot: t.TempDir()}, cs: CycleState{
				CycleID: 5, WorkspacePath: t.TempDir(), Ladder: &LadderState{Rung: string(retryladder.R2), Tier: "deep", Effort: "high"},
			}}
			dr := &dispatchResult{resp: PhaseResponse{Verdict: VerdictFAIL}, runner: gate, phaseReq: PhaseRequest{ModelRoutingTier: "fast"}}
			_, _ = cr.maybeRemediate(Phase("coverage-gate"), dr)
			if len(builder.requests) != 1 {
				t.Fatalf("remediation dispatched the builder %d times", len(builder.requests))
			}
			fix := builder.requests[0]
			var rung retryladder.Rung
			if fix.Ladder != nil {
				rung = fix.Ladder.Rung
			}
			if rung != tc.wantRung || (tc.wantRung != "" && (fix.Ladder.LadderTier() != "deep" || fix.ModelRoutingEffort != "high")) {
				t.Fatalf("remediation fix at %s = %+v effort %q, want rung %q", tc.stage, fix.Ladder, fix.ModelRoutingEffort, tc.wantRung)
			}
			if fix.ModelRoutingTier != "" {
				t.Fatalf("remediation kept the gate tier %q", fix.ModelRoutingTier)
			}
		})
	}
}

func TestEscalatedBuildTier_EnforceStartsOnlyACapabilityFailureRaised(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, stage, class string
		wantRaise          bool
	}{
		{"shadow keeps the count rule for a spec failure", policy.RetryLadderShadow, "spec", true},
		{"enforce starts a spec failure at R0", policy.RetryLadderEnforce, "spec", false},
		{"enforce starts a capability failure raised", policy.RetryLadderEnforce, "capability", true},
		{"enforce starts an unclassed failure at R0", policy.RetryLadderEnforce, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			center, events := captureSignals()
			cr := escalationRun(t, t.TempDir(), "item-a", func(string) int { return 1 }, 1)
			cr.o.workflowConfig = ladderWorkflow(tc.stage)
			cr.o.signals = center
			cr.o.failureClassFor = func(string) string { return tc.class }
			_, raised := cr.escalatedBuildTier("")
			if raised != tc.wantRaise {
				t.Fatalf("raised = %v, want %v", raised, tc.wantRaise)
			}
			decisions := eventsWithCode(*events, CodeLadderDecision)
			if len(decisions) != 1 || decisions[0].Fields["origin"] != originNextItem {
				t.Fatalf("the start rung was not recorded once: %+v", decisions)
			}
			wantNext := string(retryladder.R0)
			if tc.class == "capability" {
				wantNext = string(retryladder.R2)
			}
			if decisions[0].Fields["next"] != wantNext {
				t.Fatalf("start rung = %q, want %q", decisions[0].Fields["next"], wantNext)
			}
			if cr.cs.Ladder == nil || cr.cs.Ladder.Rung != wantNext {
				t.Fatalf("the start rung is not the rung of the cycle: %+v", cr.cs.Ladder)
			}
			_, _ = cr.escalatedBuildTier("")
			if n := len(eventsWithCode(*events, CodeLadderDecision)); n != 1 {
				t.Fatalf("a second build dispatch recorded the start rung again: %d events", n)
			}
		})
	}
}

func TestWithFailureClassReader_WiresTheReader(t *testing.T) {
	t.Parallel()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithFailureClassReader(func(string) string { return "spec" }))
	if o.failureClassFor == nil || o.failureClassFor("x") != "spec" {
		t.Fatalf("WithFailureClassReader did not wire the reader")
	}
}

func TestApplyDispatchPolicy_TheCountFloorStillRaisesACapabilityBuild(t *testing.T) {
	t.Parallel()
	cr := escalationRun(t, t.TempDir(), "item-a", func(string) int { return 1 }, 1)
	cr.o.workflowConfig = ladderWorkflow(policy.RetryLadderEnforce)
	cr.o.failureClassFor = func(string) string { return string(retryladder.Capability) }
	req := PhaseRequest{}
	cr.applyDispatchPolicy(PhaseBuild, &req)
	if req.ModelRoutingTier != "deep" {
		t.Fatalf("a capability failure must start the next build at deep, got %q", req.ModelRoutingTier)
	}
	if req.Ladder == nil || req.Ladder.Rung != retryladder.R2 || req.Ladder.LadderTier() != "deep" {
		t.Fatalf("the next-item build does not carry the R2 start rung: %+v", req.Ladder)
	}
}

func TestReviewResumedDeliverable_AsksTheLadderWhenItsCorrectionsAreSpent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		stage      string
		dispatches int
	}{
		{policy.RetryLadderShadow, 1},
		{policy.RetryLadderEnforce, 2},
	} {
		t.Run(tc.stage, func(t *testing.T) {
			root := t.TempDir()
			writeCLIProfile(t, root, "builder", "agy-tmux", nil)
			center, events := captureSignals()
			probe := newLadderProbe(0)
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithReviewer(probe), WithSignalCenter(center),
				WithRetryConfig(policy.RetryConfig{ContractCorrectionRetries: 1}), WithWorkflowConfig(ladderWorkflow(tc.stage)))
			_, err := o.reviewResumedDeliverable(context.Background(), root, 9, CycleState{CycleID: 9, WorkspacePath: t.TempDir()},
				PhaseBuild, probe, PhaseRequest{}, PhaseResponse{Phase: "build", Verdict: VerdictPASS}, nil, func(_ Phase, r PhaseRequest) PhaseRequest { return r })
			if err == nil {
				t.Fatalf("a deliverable that never passes must end rejected")
			}
			if len(probe.requests) != tc.dispatches {
				t.Fatalf("resume dispatched %d corrections, want %d", len(probe.requests), tc.dispatches)
			}
			decisions := eventsWithCode(*events, CodeLadderDecision)
			if len(decisions) == 0 || decisions[0].Fields["origin"] != originResumedCorrection {
				t.Fatalf("the resumed correction did not ask the ladder: %+v", decisions)
			}
			if tc.stage == policy.RetryLadderEnforce {
				raised := probe.requests[1]
				if raised.Ladder == nil || raised.Ladder.Context != retryladder.ContextFresh || !strings.Contains(raised.CorrectionDirective, ladderFloorReason) {
					t.Fatalf("the resumed raised try = %+v %q", raised.Ladder, raised.CorrectionDirective)
				}
			}
		})
	}
}

func TestContractLadderAsk_CarriesTheRemedyBesideTheCheck(t *testing.T) {
	t.Parallel()
	ask := contractLadderAsk(PhaseBuild, "o", "/wt", ReviewResult{Reason: "[a] red", Remediation: "add the test"})
	if len(ask.failures) != 2 || ask.failures[0] != "[a] red" || ask.failures[1] != "add the test" || !ask.facts.ContractRejected {
		t.Fatalf("ask = %+v", ask)
	}
	if plain := contractLadderAsk(PhaseBuild, "o", "/wt", ReviewResult{Reason: "[a] red"}); len(plain.failures) != 1 {
		t.Fatalf("an ask without a remedy carries %v", plain.failures)
	}
}

func TestWithRepairRound_StampsACopy(t *testing.T) {
	t.Parallel()
	prior := &LadderState{Rung: "R2", RepairRound: 1}
	got := withRepairRound(prior, 2, "")
	if got == prior || got.RepairRound != 2 || got.Rung != "R2" || prior.RepairRound != 1 {
		t.Fatalf("withRepairRound = %+v, prior %+v: want a stamped copy", got, prior)
	}
	if fresh := withRepairRound(nil, 3, "would"); fresh.RepairRound != 3 || fresh.Rung != string(retryladder.R0) || fresh.AuditFix != "would" {
		t.Fatalf("withRepairRound(nil) = %+v", fresh)
	}
}
