package core

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/codereview"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const malformedReview = "code-review deliverable failed contract: [bad_grammar] ## Scores is missing dimension(s) security"

type exhaustion struct {
	cr     *cycleRun
	dr     *dispatchResult
	rev    *recordingReviewer
	runner *fakeRunner
	ledger *fakeLedger
	events *[]signalcenter.Event
}

func exhaustionHarness(t *testing.T, phase Phase, spec phasespec.PhaseSpec, completed ...string) exhaustion {
	t.Helper()
	cat, warnings := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{spec})
	if len(warnings) != 0 {
		t.Fatalf("catalog: %v", warnings)
	}
	center := signalcenter.New()
	events := &[]signalcenter.Event{}
	center.Subscribe(func(e signalcenter.Event) { *events = append(*events, e) })
	rev := &recordingReviewer{default_: ReviewResult{Approve: false, Reason: malformedReview}}
	led := &fakeLedger{}
	o := NewOrchestrator(&fakeStorage{}, led, buildRunners(nil), WithReviewer(rev), WithCatalog(cat), WithSignalCenter(center))
	runner := &fakeRunner{name: string(phase)}
	cr := &cycleRun{
		o: o, ctx: context.Background(), req: CycleRequest{ProjectRoot: t.TempDir()}, cycle: 21,
		cs:          CycleState{CycleID: 21, WorkspacePath: t.TempDir(), CompletedPhases: completed},
		retryConfig: policy.Policy{}.RetryConfig(), envSnap: map[string]string{},
	}
	dr := &dispatchResult{resp: PhaseResponse{Phase: string(phase), Verdict: VerdictPASS}, attemptCount: 1, runner: runner}
	return exhaustion{cr: cr, dr: dr, rev: rev, runner: runner, ledger: led, events: events}
}

func codeReviewSpec() phasespec.PhaseSpec {
	return phasespec.PhaseSpec{Name: codereview.PhaseName, Role: string(phasespec.RoleEvaluate), Optional: true, After: "build"}
}

func (h exhaustion) ledgerKinds(phase Phase) []string {
	var kinds []string
	for _, e := range h.ledger.entries {
		if e.Role == string(phase) {
			kinds = append(kinds, e.Kind)
		}
	}
	return kinds
}

func (h exhaustion) exemptions() []bool {
	var seen []bool
	for _, in := range h.rev.calls {
		seen = append(seen, in.BreakerExempt)
	}
	return seen
}

func TestReviewWithCorrections_AnOptionalEvaluatePhaseThatExhaustsItsCorrectionsIsSkippedNotAborted(t *testing.T) {
	h := exhaustionHarness(t, Phase(codereview.PhaseName), codeReviewSpec(), "scout", "build")

	act, err := h.cr.reviewWithCorrections(Phase(codereview.PhaseName), h.dr)

	if act != loopNext || err != nil {
		t.Fatalf("reviewWithCorrections = (%v, %v), want the walk to continue: an optional evaluate phase never aborts the cycle", act, err)
	}
	if h.dr.resp.Verdict != VerdictSKIPPED || h.runner.calls != 2 {
		t.Errorf("verdict %s after %d correction dispatch(es), want SKIPPED after the 2 corrections", h.dr.resp.Verdict, h.runner.calls)
	}
	if kinds := strings.Join(h.ledgerKinds(Phase(codereview.PhaseName)), ","); !strings.HasSuffix(kinds, "contract_exhaustion_skip") || strings.Contains(kinds, ledgerKindContractGateDemoted) {
		t.Errorf("ledger kinds %s, want a contract_exhaustion_skip and never a demotion", kinds)
	}
	for i, exempt := range h.exemptions() {
		if !exempt {
			t.Errorf("review %d was not breaker-exempt: an optional evaluate phase's blocks must never reach the global breaker", i)
		}
	}
	var skipped []signalcenter.Event
	for _, e := range *h.events {
		if e.Code == codereview.CodeSkipped {
			skipped = append(skipped, e)
		}
	}
	if len(skipped) != 1 || skipped[0].Fields["reason"] != "malformed" || skipped[0].Severity != signalcenter.SeverityWarn || skipped[0].Cycle != 21 {
		t.Errorf("REVIEW_SKIPPED = %+v, want one WARN for cycle 21 with reason malformed", skipped)
	}
}

func TestReviewWithCorrections_AnOptionalEvaluatePhaseWhoseDeliverableIsAbsentStillAborts(t *testing.T) {
	h := exhaustionHarness(t, Phase(codereview.PhaseName), codeReviewSpec(), "scout", "build")
	h.rev.default_.DeliverableAbsent = true

	act, err := h.cr.reviewWithCorrections(Phase(codereview.PhaseName), h.dr)

	if act != loopAbort || err == nil || h.dr.resp.Verdict == VerdictSKIPPED {
		t.Fatalf("reviewWithCorrections = (%v, %v) verdict %s, want today's abort: a non-admitted missing deliverable never reaches Ship, only a malformed one degrades", act, err, h.dr.resp.Verdict)
	}
	for i, exempt := range h.exemptions() {
		if !exempt {
			t.Errorf("review %d was not breaker-exempt: the breaker must not demote an absent deliverable into an approval either", i)
		}
	}
}

func TestReviewWithCorrections_AnotherOptionalEvaluatePhaseDegradesWithoutAReviewSignal(t *testing.T) {
	smell := phasespec.PhaseSpec{Name: "smell-scan", Role: string(phasespec.RoleEvaluate), Optional: true, After: "build"}
	h := exhaustionHarness(t, "smell-scan", smell, "scout", "build")

	act, err := h.cr.reviewWithCorrections("smell-scan", h.dr)

	if act != loopNext || err != nil || h.dr.resp.Verdict != VerdictSKIPPED {
		t.Fatalf("reviewWithCorrections = (%v, %v) verdict %s, want the degrade for any optional evaluate phase", act, err, h.dr.resp.Verdict)
	}
	for _, e := range *h.events {
		if e.Code == codereview.CodeSkipped {
			t.Errorf("REVIEW_SKIPPED fired for smell-scan: the review module speaks only for code-review (%+v)", e)
		}
	}
}

func TestReviewWithCorrections_ABreakerExemptLadderNeverWarnsOfABreakerThatCannotOpen(t *testing.T) {
	h := exhaustionHarness(t, Phase(codereview.PhaseName), codeReviewSpec(), "scout", "build")

	_, _ = h.cr.reviewWithCorrections(Phase(codereview.PhaseName), h.dr)

	if h.runner.calls != 2 {
		t.Fatalf("%d correction dispatch(es), want 2", h.runner.calls)
	}
	if got, want := h.runner.requests[1].CorrectionDirective, composeCorrection(2, malformedReview, ""); got != want {
		t.Errorf("second correction directive = %q, want the plain correction: the salvage re-prompt promises a breaker opening an exempt phase never reaches", got)
	}
}

func TestReviewWithCorrections_APhaseOutsideTheOptionalEvaluateClassStillAborts(t *testing.T) {
	cases := map[string]struct {
		phase     Phase
		spec      phasespec.PhaseSpec
		completed []string
		mandatory bool
	}{
		"a floor phase":                                 {PhaseBuild, codeReviewSpec(), []string{"scout"}, false},
		"an optional evaluate phase pre-floor":          {Phase(codereview.PhaseName), codeReviewSpec(), []string{"scout"}, false},
		"a required evaluate phase":                     {"gatekeeper", phasespec.PhaseSpec{Name: "gatekeeper", Role: string(phasespec.RoleEvaluate), After: "build"}, []string{"scout", "build"}, false},
		"an optional plan phase":                        {"sketch", phasespec.PhaseSpec{Name: "sketch", Role: string(phasespec.RolePlan), Optional: true, After: "build"}, []string{"scout", "build"}, false},
		"an operator-mandatory optional evaluate phase": {"secret-leak-scan", phasespec.PhaseSpec{Name: "secret-leak-scan", Role: string(phasespec.RoleEvaluate), Optional: true, After: "build"}, []string{"scout", "build"}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := exhaustionHarness(t, tc.phase, tc.spec, tc.completed...)
			if tc.mandatory {
				h.cr.o.cfg.Mandatory = []string{"scout", "build", string(tc.phase), "audit", "ship"}
			}

			act, err := h.cr.reviewWithCorrections(tc.phase, h.dr)

			if act != loopAbort || err == nil || !strings.Contains(err.Error(), "deliverable rejected after 2 correction(s)") {
				t.Fatalf("reviewWithCorrections = (%v, %v), want today's abort", act, err)
			}
			for i, exempt := range h.exemptions() {
				if exempt {
					t.Errorf("review %d was breaker-exempt: only an optional evaluate phase past the floor is", i)
				}
			}
		})
	}
}

func TestReviewWithCorrections_ADegradedReviewStillRecordsItsFailureLearning(t *testing.T) {
	h := exhaustionHarness(t, Phase(codereview.PhaseName), codeReviewSpec(), "scout", "build")

	_, _ = h.cr.reviewWithCorrections(Phase(codereview.PhaseName), h.dr)

	if len(h.cr.state.FailedAt) != 1 {
		t.Errorf("FailedAt = %+v, want the exhausted ladder recorded once: a degrade is a WARN, never a silent pass", h.cr.state.FailedAt)
	}
}

func TestReviewWithCorrections_ASkippedReviewNamesTheRoundItWouldHaveBeen(t *testing.T) {
	h := exhaustionHarness(t, Phase(codereview.PhaseName), codeReviewSpec(), "scout", "build")

	_, _ = h.cr.reviewWithCorrections(Phase(codereview.PhaseName), h.dr)

	var rounds []string
	for _, e := range *h.events {
		if e.Code == codereview.CodeSkipped {
			rounds = append(rounds, e.Fields["round"])
		}
	}
	if len(rounds) != 1 || rounds[0] != "1" {
		t.Errorf("REVIEW_SKIPPED rounds = %v, want [1]: the skipped dispatch was the cycle's first review round", rounds)
	}
}

func TestReviewWithCorrections_OneCorrectionIsCountedInTheAbortMessage(t *testing.T) {
	h := exhaustionHarness(t, PhaseBuild, codeReviewSpec(), "scout")
	h.cr.retryConfig.ContractCorrectionRetries = 1

	_, err := h.cr.reviewWithCorrections(PhaseBuild, h.dr)

	if err == nil || !strings.Contains(err.Error(), `phase "build" deliverable rejected after 1 correction(s)`) {
		t.Errorf("err = %v, want the abort to count its one correction: only a ladder with no corrections keeps the legacy message", err)
	}
}

func TestReviewWithCorrections_AnAbortWithNoCorrectionsTeachesTheRetroItsFirstAttempt(t *testing.T) {
	h := exhaustionHarness(t, PhaseBuild, codeReviewSpec(), "scout")
	h.cr.retryConfig.ContractCorrectionRetries = 0

	_, _ = h.cr.reviewWithCorrections(PhaseBuild, h.dr)

	retro := h.cr.o.runners[PhaseRetro].(*fakeRunner)
	if len(retro.requests) != 1 || retro.requests[0].Context["failure_attempt"] != "1" {
		t.Errorf("retro requests = %d, failure_attempt = %q, want one retro told attempt 1: the failed dispatch is the first attempt even with no correction", len(retro.requests), attemptOf(retro.requests))
	}
}

func attemptOf(reqs []PhaseRequest) string {
	if len(reqs) == 0 {
		return ""
	}
	return reqs[0].Context["failure_attempt"]
}

func resumedReview(t *testing.T, phase Phase, spec phasespec.PhaseSpec, verdictOf ReviewResult, mandatory bool) (*Orchestrator, *recordingReviewer, *fakeLedger, PhaseResponse, error) {
	t.Helper()
	cat, warnings := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{spec})
	if len(warnings) != 0 {
		t.Fatalf("catalog: %v", warnings)
	}
	rev := &recordingReviewer{default_: verdictOf}
	led := &fakeLedger{}
	o := NewOrchestrator(&fakeStorage{}, led, buildRunners(nil), WithReviewer(rev), WithCatalog(cat))
	if mandatory {
		o.cfg.Mandatory = []string{"scout", "build", string(phase), "audit", "ship"}
	}
	cs := CycleState{CycleID: 21, WorkspacePath: t.TempDir(), CompletedPhases: []string{"scout", "build"}}
	resp, err := o.reviewResumedDeliverable(context.Background(), t.TempDir(), 21, cs, phase, &fakeRunner{name: string(phase)}, PhaseRequest{}, PhaseResponse{Phase: string(phase), Verdict: VerdictPASS}, nil)
	return o, rev, led, resp, err
}

func TestReviewResumedDeliverable_AResumedReviewIsBreakerExemptAndDegradesLikeAFreshOne(t *testing.T) {
	_, rev, led, resp, err := resumedReview(t, Phase(codereview.PhaseName), codeReviewSpec(), ReviewResult{Approve: false, Reason: malformedReview}, false)

	if err != nil || resp.Verdict != VerdictSKIPPED {
		t.Fatalf("reviewResumedDeliverable = (%s, %v), want SKIPPED and the walk continuing: a resumed review degrades like a fresh one", resp.Verdict, err)
	}
	for i, in := range rev.calls {
		if !in.BreakerExempt {
			t.Errorf("resumed review %d was not breaker-exempt: the resume ladder must never count the review's blocks toward the global breaker", i)
		}
	}
	if kinds := ledgerKindsOf(led, Phase(codereview.PhaseName)); !strings.Contains(kinds, ledgerKindContractExhaustionSkip) {
		t.Errorf("ledger kinds %s, want the %s entry", kinds, ledgerKindContractExhaustionSkip)
	}
}

func TestReviewResumedDeliverable_StillAbortsOutsideTheDegrade(t *testing.T) {
	scan := phasespec.PhaseSpec{Name: "secret-leak-scan", Role: string(phasespec.RoleEvaluate), Optional: true, After: "build"}
	cases := map[string]struct {
		phase     Phase
		spec      phasespec.PhaseSpec
		review    ReviewResult
		mandatory bool
		exempt    bool
	}{
		"an absent review report":                       {Phase(codereview.PhaseName), codeReviewSpec(), ReviewResult{Approve: false, Reason: "missing", DeliverableAbsent: true}, false, true},
		"an operator-mandatory optional evaluate phase": {"secret-leak-scan", scan, ReviewResult{Approve: false, Reason: malformedReview}, true, false},
		"a floor phase":                                 {PhaseBuild, codeReviewSpec(), ReviewResult{Approve: false, Reason: malformedReview}, false, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, rev, _, resp, err := resumedReview(t, tc.phase, tc.spec, tc.review, tc.mandatory)

			if err == nil || !strings.Contains(err.Error(), "resume review gate") || resp.Verdict == VerdictSKIPPED {
				t.Fatalf("reviewResumedDeliverable = (%s, %v), want the resume abort", resp.Verdict, err)
			}
			for i, in := range rev.calls {
				if in.BreakerExempt != tc.exempt {
					t.Errorf("review %d BreakerExempt = %v, want %v", i, in.BreakerExempt, tc.exempt)
				}
			}
		})
	}
}

func ledgerKindsOf(led *fakeLedger, phase Phase) string {
	var kinds []string
	for _, e := range led.entries {
		if e.Role == string(phase) {
			kinds = append(kinds, e.Kind)
		}
	}
	return strings.Join(kinds, ",")
}
