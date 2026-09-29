package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func explanationRouteState(t *testing.T, attempts int, defects ...string) CycleState {
	t.Helper()
	return CycleState{
		CycleID:             correctionCycle,
		RunID:               correctionRunID,
		WorkspacePath:       auditFailFixture(t, policy.CategoryCodeAuditFail, defects...),
		AuditRepairAttempts: attempts,
	}
}

func explanationRouteOrchestrator(adj RetryAdjudicator) *Orchestrator {
	o := floorOrchestrator(fixedNextStrategy{next: "end"})
	o.failurePolicy = policy.DefaultSystemFailurePolicy()
	o.retryAdjudicator = adj
	return o
}

func TestDecideAfterAuditFail_AnExplanationOnlyFailReauthorsAtBuildNotTDD(t *testing.T) {
	cs := explanationRouteState(t, 0, cycle1745Defects...)

	next, reason, sig := explanationRouteOrchestrator(nil).decideAfterAuditFail(cs)

	if sig != nil || next != PhaseBuild {
		t.Fatalf("next = %s (sig %+v), want build: a FAIL naming only the explanation document re-authors it, it does not restart at tdd (reason %q)", next, sig, reason)
	}
	if !strings.HasPrefix(reason, auditRepairReasonPrefix+string(retryActionReauthorExplanation)+": ") {
		t.Errorf("reason %q must be a repair grant for the explanation re-author, so the round is counted and seeded", reason)
	}
	if !strings.Contains(reason, explanationNeedsCorrection) {
		t.Errorf("reason %q must name the %s class for the operator", reason, explanationNeedsCorrection)
	}
}

func TestDecideAfterAuditFail_AMixedFailKeepsTDD(t *testing.T) {
	cs := explanationRouteState(t, 0, cycle1745Defects[0], "H1: go/internal/cyclesimulator/characterization_test.go:263-281 builds a raw git repo outside internal/gittest")

	next, reason, _ := explanationRouteOrchestrator(nil).decideAfterAuditFail(cs)

	if next != PhaseTDD {
		t.Fatalf("next = %s, want tdd: a FAIL with any defect outside the document keeps today's routing (reason %q)", next, reason)
	}
	if strings.Contains(reason, string(retryActionReauthorExplanation)) {
		t.Errorf("a mixed FAIL was routed as explanation-only: %q", reason)
	}
	if _, err := os.Stat(filepath.Join(cs.WorkspacePath, "audit-fail-reason.json")); !os.IsNotExist(err) {
		t.Errorf("a mixed FAIL must leave the workspace as today (no explanation record); stat err = %v", err)
	}
}

func TestDecideAfterAuditFail_AnExplanationOnlyFailRecordsItsFindings(t *testing.T) {
	cs := explanationRouteState(t, 0, cycle1745Defects...)

	explanationRouteOrchestrator(nil).decideAfterAuditFail(cs)

	var findings string
	stderr := captureStderr(t, func() {
		findings = readContinuationFindings(filepath.Join(cs.WorkspacePath, "audit-fail-reason.json"))
	})
	if strings.Contains(stderr, "unreadable") {
		t.Fatalf("the continuation WARNs the findings artifact is unreadable after a doc-only FAIL:\n%s", stderr)
	}
	if !strings.Contains(findings, explanationNeedsCorrection+": "+cycle1745Defects[1]) {
		t.Fatalf("the recorded findings lack the audit's defect:\n%s", findings)
	}
}

func TestDecideAfterAuditFail_AnExplanationOnlyFailAtTheCapDeclinesAsToday(t *testing.T) {
	cs := explanationRouteState(t, 2, cycle1745Defects...)

	next, reason, _ := explanationRouteOrchestrator(nil).decideAfterAuditFail(cs)

	if next != PhaseRetro || !strings.HasPrefix(reason, auditDeclineReasonPrefix) {
		t.Fatalf("next = %s reason %q: the policy's retry budget still bounds a document correction", next, reason)
	}
	if _, err := os.Stat(filepath.Join(cs.WorkspacePath, "audit-fail-reason.json")); !os.IsNotExist(err) {
		t.Errorf("a declined FAIL must leave the retro's digest inputs as today; stat err = %v", err)
	}
}

func TestDecideAfterAuditFail_TheAdjudicatorCannotSendAnExplanationOnlyFailToTDD(t *testing.T) {
	stub := &stubAdjudicator{give: &adjudication{Action: retryActionRetryTDD, Justification: "encode the defects as tests first"}}
	cs := explanationRouteState(t, 0, cycle1745Defects...)

	next, reason, _ := explanationRouteOrchestrator(stub).decideAfterAuditFail(cs)

	if next != PhaseBuild || !strings.Contains(reason, "clamped") {
		t.Fatalf("next = %s reason %q: tdd is outside a document correction's envelope and must be clamped", next, reason)
	}
}

func TestExplanationCorrectionEnvelope_NarrowsOnlyAGrantedRetry(t *testing.T) {
	granted := retryEnvelope{Legal: []retryAction{retryActionRetryTDD, retryActionRetryBuild, retryActionDecline}, Reason: "policy grants"}
	got := explanationCorrectionEnvelope(granted)
	if !sameActions(got.Legal, []retryAction{retryActionReauthorExplanation, retryActionDecline}) || !strings.HasPrefix(got.Reason, "policy grants; ") {
		t.Fatalf("granted envelope narrowed to %+v", got)
	}
	for _, env := range []retryEnvelope{declineOnly("budget spent"), {Halt: true, Reason: "floor"}} {
		if got := explanationCorrectionEnvelope(env); !sameActions(got.Legal, env.Legal) || got.Halt != env.Halt || got.Reason != env.Reason {
			t.Errorf("an envelope with no retry must pass through unchanged: %+v became %+v", env, got)
		}
	}
	if next, ok := reentryPhase(retryActionReauthorExplanation); !ok || next != PhaseBuild {
		t.Errorf("reentryPhase(%s) = (%s, %v), want (build, true): the re-author is Build's", retryActionReauthorExplanation, next, ok)
	}
}

func TestSeedAuditRepairContext_AnExplanationOnlyRoundScopesTheBuild(t *testing.T) {
	cs := explanationRouteState(t, 1, cycle1745Defects...)
	cs.AuditRepairActive = true

	build := seedAuditRepairContext(map[string]string{}, PhaseBuild, cs)
	if build[CtxKeyExplanationReauthor] != correctionDocument {
		t.Fatalf("build context %q = %q, want %q", CtxKeyExplanationReauthor, build[CtxKeyExplanationReauthor], correctionDocument)
	}

	mixed := explanationRouteState(t, 1, cycle1745Defects[0], "H1: go/internal/x.go:12 is wrong")
	mixed.AuditRepairActive = true
	cs.AuditRepairActive = false
	for name, got := range map[string]map[string]string{
		"a mixed round":        seedAuditRepairContext(map[string]string{}, PhaseBuild, mixed),
		"no repair in flight":  seedAuditRepairContext(map[string]string{}, PhaseBuild, cs),
		"the test-first phase": seedAuditRepairContext(map[string]string{}, PhaseTDD, explanationActive(cs)),
	} {
		if _, set := got[CtxKeyExplanationReauthor]; set {
			t.Errorf("%s must not scope the dispatch to the document: %+v", name, got)
		}
	}
}

func explanationActive(cs CycleState) CycleState {
	cs.AuditRepairActive = true
	return cs
}
