package convergence

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const landingGuardCause = "landing-guard"

func rungReasonOf(d Decision) string {
	for _, r := range d.Reasons {
		if strings.HasPrefix(r, "CONVERGENCE_RUNG ") {
			return r
		}
	}
	return ""
}

func withKeptRoundOpenCritical(t *testing.T, open bool) {
	t.Helper()
	prev := keptRoundHasOpenCritical
	keptRoundHasOpenCritical = func(decider, int) bool { return open }
	t.Cleanup(func() { keptRoundHasOpenCritical = prev })
}

func cleanFirstRound(loop Loop) Input {
	return Input{Loop: loop, Round: 0, Rounds: []Judgment{{Index: 0}}, Config: policy.Policy{}.ConvergenceConfig()}
}

func TestLandingGuard_StopNamesItsOwnCause(t *testing.T) {
	for _, loop := range []Loop{LoopCodeReview, LoopConsoleLane, LoopCycle, LoopInboxItem} {
		t.Run(string(loop), func(t *testing.T) {
			withKeptRoundOpenCritical(t, true)

			d := Decide(cleanFirstRound(loop))

			if d.Action != ActionStop {
				t.Fatalf("action %s, want Stop when the kept round holds an open CRITICAL (reasons %q)", d.Action, d.Reasons)
			}
			reason := rungReasonOf(d)
			if !strings.Contains(reason+" ", " cause="+landingGuardCause+" ") {
				t.Fatalf("rung reason %q, want cause=%s: the guard must not borrow the next rung's cause", reason, landingGuardCause)
			}
			if !strings.Contains(reason+" ", " action=Stop ") {
				t.Fatalf("rung reason %q, want action=Stop", reason)
			}
		})
	}
}

func TestLandingGuard_AGuardThatDoesNotFireKeepsTheLandingAndItsCause(t *testing.T) {
	withKeptRoundOpenCritical(t, false)

	d := Decide(cleanFirstRound(LoopCodeReview))

	if d.Action != ActionLand {
		t.Fatalf("action %s, want Land", d.Action)
	}
	if reason := rungReasonOf(d); strings.Contains(reason, landingGuardCause) {
		t.Fatalf("rung reason %q names the landing guard on a clean landing", reason)
	}
}

func TestLandingGuard_ARungThreeStopKeepsTheLaddersCause(t *testing.T) {
	withKeptRoundOpenCritical(t, false)
	critical := Finding{ID: "c1", Severity: SeverityCritical, Status: StatusOpen}
	in := Input{Loop: LoopCodeReview, Round: 2, Config: policy.Policy{}.ConvergenceConfig(), Rounds: []Judgment{
		{Index: 0, Findings: []Finding{critical}}, {Index: 1, Findings: []Finding{critical}},
		{Index: 2, Findings: []Finding{critical}},
	}}

	d := Decide(in)

	if d.Action != ActionStop {
		t.Fatalf("action %s, want Stop for an open CRITICAL (reasons %q)", d.Action, d.Reasons)
	}
	if reason := rungReasonOf(d); strings.Contains(reason, landingGuardCause) {
		t.Fatalf("rung reason %q: a scope-change Stop is not the landing guard", reason)
	}
}
