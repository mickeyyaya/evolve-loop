package core

import (
	"testing"
)

func TestBlockerRoundsDecision_OnlyRefusedSendsACorrectionWithAClaimOnIt(t *testing.T) {
	t.Parallel()
	cases := []struct {
		outcome BlockerOutcome
		want    blockerRounds
	}{
		{NoClaim, blockerRounds{End: false, Claim: false}},
		{Routable, blockerRounds{End: true}},
		{Unroutable, blockerRounds{End: true}},
		{Incomplete, blockerRounds{End: true}},
		{Refused, blockerRounds{End: false, Claim: true}},
	}
	for _, c := range cases {
		if got := blockerRoundsDecision(c.outcome); got != c.want {
			t.Errorf("blockerRoundsDecision(%d) = %+v, want %+v", c.outcome, got, c.want)
		}
	}
}

func TestEndedBlockerOutcome_AFailEndingLeadsWithTheCodeEvenOverAPassingResponse(t *testing.T) {
	t.Parallel()
	end := blockerEnd{outcome: Incomplete, diag: Diagnostic{Code: DiagCodeBlockerIncomplete, Message: "no base"}}
	resp := PhaseResponse{Verdict: VerdictPASS, Diagnostics: []Diagnostic{{Code: "OTHER"}}}

	got := endedBlockerOutcome(PhaseBuild, resp, 2, end, "t0")

	if got.Verdict != VerdictFAIL || got.AbortReason != "no base" || got.AttemptCount != 2 || len(got.Diagnostics) != 2 || got.Diagnostics[0].Code != DiagCodeBlockerIncomplete {
		t.Fatalf("outcome = %+v; an ended blocker is a FAIL that leads with its code and keeps the response diagnostics", got)
	}
}

func TestEndOnBlocker_ADispatchWithNoCountTeachesTheRetroItsFirstAttempt(t *testing.T) {
	t.Parallel()
	h := exhaustionHarness(t, PhaseBuild, codeReviewSpec(), "scout")
	h.dr.attemptCount = 0

	act, err := h.cr.endOnBlocker(PhaseBuild, h.dr, Incomplete, Diagnostic{Code: DiagCodeBlockerIncomplete, Message: "no base"})

	retro := h.cr.o.runners[PhaseRetro].(*fakeRunner)
	if act != loopAbort || err == nil || len(retro.requests) != 1 || attemptOf(retro.requests) != "1" {
		t.Fatalf("act %v err %v attempt %q; an ended blocker aborts and tells the retro attempt 1", act, err, attemptOf(retro.requests))
	}
}

func TestRejectAfterCorrections_ARejectionWithNoClaimEmitsNoRefusal(t *testing.T) {
	t.Parallel()
	h := exhaustionHarness(t, PhaseBuild, codeReviewSpec(), "scout")

	act, err := h.cr.rejectAfterCorrections(PhaseBuild, h.dr, ReviewResult{Reason: "floor"}, 1)

	for _, e := range *h.events {
		if e.Code == CodeBlockerRefused {
			t.Fatalf("act %v err %v signal %+v; a rejection that declared no blocker must not emit a refusal", act, err, e)
		}
	}
	if act != loopAbort || err == nil {
		t.Fatalf("act %v err %v; the rejection aborts the cycle", act, err)
	}
}
