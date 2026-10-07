package convergence_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

func rung3Input(loop convergence.Loop, remaining []convergence.Finding, facts map[string]convergence.ComponentFacts) convergence.Input {
	in := inputFor(loop, many("a", 4, convergence.SeverityCritical), join(asFixed(many("a", 4, convergence.SeverityCritical)), remaining))
	in.Config = withBudget(1)
	in.Components = facts
	return in
}

var (
	twoHighInX     = []convergence.Finding{finding("h1", convergence.SeverityHigh, ofComponent("X")), finding("h2", convergence.SeverityHigh, ofComponent("X"))}
	criticalInX    = []convergence.Finding{finding("c1", convergence.SeverityCritical, ofComponent("X")), finding("h1", convergence.SeverityHigh, ofComponent("X"))}
	separableX     = map[string]convergence.ComponentFacts{"X": {Separable: true}}
	certifiedX     = map[string]convergence.ComponentFacts{"X": {FailSafeCertificate: "TestXFailsSafe"}}
	separableSafeX = map[string]convergence.ComponentFacts{"X": {Separable: true, FailSafeCertificate: "TestXFailsSafe"}}
)

func TestExit_AdjudicatesTheRemainingHighsAtTopWhenThereIsHeadroom(t *testing.T) {
	in := rung3Input(convergence.LoopConsoleLane, twoHighInX, nil)
	in.Judge, in.Headroom = deepJudge, v3bTables

	d := decide(t, in)

	requireOutcome(t, d, convergence.ActionAdjudicate, 3)
	if d.JudgeRaise != topJudge {
		t.Fatalf("judge raise %+v, want the family's top", d.JudgeRaise)
	}

	withCritical := rung3Input(convergence.LoopConsoleLane, criticalInX, nil)
	withCritical.Judge, withCritical.Headroom = deepJudge, v3bTables

	requireOutcome(t, decide(t, withCritical), convergence.ActionStop, 3)
}

func TestExit_NoHeadroomSkipsAdjudicationWithTheReason(t *testing.T) {
	in := rung3Input(convergence.LoopConsoleLane, twoHighInX, nil)
	in.Judge, in.Headroom = deepJudge, todaysTables

	d := decide(t, in)

	requireOutcome(t, d, convergence.ActionStop, 3)
	requireReason(t, d, "CONVERGENCE_NO_HEADROOM", "loop=console-lane", "role=judge", "family=claude-tmux", "from=deep", "to=top")
	if d.JudgeRaise != noRaise {
		t.Fatalf("judge raise %+v, want none faked", d.JudgeRaise)
	}

	atTop := rung3Input(convergence.LoopConsoleLane, twoHighInX, nil)
	atTop.Judge, atTop.Headroom = topJudge, v3bTables

	requireReason(t, decide(t, atTop), "CONVERGENCE_NO_HEADROOM", "role=judge", "from=top", "to=top")
}

func TestExit_AdjudicationNeedsATopThatDiffersFromTheJudgesOwnModelAndEffort(t *testing.T) {
	for _, judge := range []convergence.TierEffort{
		{Family: "claude-tmux", Tier: "deep", Model: "opus", Effort: "xhigh"},
		{Family: "claude-tmux", Tier: "deep", Effort: "xhigh"},
	} {
		in := rung3Input(convergence.LoopConsoleLane, twoHighInX, nil)
		in.Judge, in.Headroom = judge, v3bTables

		d := decide(t, in)

		requireOutcome(t, d, convergence.ActionStop, 3)
		requireReason(t, d, "CONVERGENCE_NO_HEADROOM", "role=judge", "family=claude-tmux", "from=deep", "to=top")
	}
}

func TestExit_SplitUnstagesTheComponentHoldingEveryStrictFinding(t *testing.T) {
	remaining := join(twoHighInX, []convergence.Finding{finding("m", convergence.SeverityMedium, ofComponent("Y")),
		finding("i", convergence.SeverityInfo, ofComponent("X"))})

	d := decide(t, rung3Input(convergence.LoopConsoleLane, remaining, separableX))

	requireOutcome(t, d, convergence.ActionSplit, 3)
	if d.SplitComponent != "X" || !reflect.DeepEqual(d.Defer, []string{"m"}) || len(d.File) != 0 || d.LandRound != 1 {
		t.Fatalf("%+v, want X split with its own findings, the rest's MEDIUM deferred and round 1 landing", d)
	}
	requireReason(t, d, "CONVERGENCE_SPLIT", "component=X", "followup=X")
}

func TestExit_SplitNeedsOneSeparableComponentHoldingEveryStrictFinding(t *testing.T) {
	spreadOut := []convergence.Finding{finding("h1", convergence.SeverityHigh, ofComponent("X")), finding("h2", convergence.SeverityHigh, ofComponent("Y"))}

	requireOutcome(t, decide(t, rung3Input(convergence.LoopConsoleLane, spreadOut, separableX)), convergence.ActionStop, 3)
	requireOutcome(t, decide(t, rung3Input(convergence.LoopConsoleLane, twoHighInX, nil)), convergence.ActionStop, 3)
}

func TestExit_ACycleLoopsSplitIsAStopPlusAContinuation(t *testing.T) {
	d := decide(t, rung3Input(convergence.LoopCodeReview, twoHighInX, separableX))

	requireOutcome(t, d, convergence.ActionStop, 3)
	if d.SplitComponent != "X" {
		t.Fatalf("split %q, want the continuation to unwire X", d.SplitComponent)
	}
	requireReason(t, d, "CONVERGENCE_SPLIT", "component=X")
	requireReason(t, d, "CONVERGENCE_STOP", "loop=code-review", "round=1")
}

func TestExit_AcceptWithLimitsDefersTheCertifiedResidue(t *testing.T) {
	d := decide(t, rung3Input(convergence.LoopCodeReview, twoHighInX, certifiedX))

	requireOutcome(t, d, convergence.ActionAcceptWithLimits, 3)
	if !reflect.DeepEqual(d.Defer, []string{"h1", "h2"}) || !reflect.DeepEqual(d.Redesign, []string{"X"}) || d.LandRound != 1 {
		t.Fatalf("%+v, want both HIGHs deferred with the certificate and X's redesign follow-up", d)
	}
	requireReason(t, d, "CONVERGENCE_ACCEPTED_LIMITS", "component=X", "certificate=TestXFailsSafe")

	both := []convergence.Finding{finding("h1", convergence.SeverityHigh, ofComponent("X")), finding("h2", convergence.SeverityHigh, ofComponent("Y"))}
	facts := map[string]convergence.ComponentFacts{"X": {FailSafeCertificate: "TestX"}, "Y": {FailSafeCertificate: "TestY"}}
	twoComponents := decide(t, rung3Input(convergence.LoopConsoleLane, both, facts))

	requireOutcome(t, twoComponents, convergence.ActionAcceptWithLimits, 3)
	if !reflect.DeepEqual(twoComponents.Redesign, []string{"X", "Y"}) {
		t.Fatalf("redesign %q, want both certified components", twoComponents.Redesign)
	}
}

func TestExit_AcceptDefersTheLandedRoundsResidueAndNeverAnEarlierCritical(t *testing.T) {
	inA := ofComponent("A")
	certifiedA := map[string]convergence.ComponentFacts{"A": {FailSafeCertificate: "TestAFailsSafe"}}
	console := inputOf(convergence.LoopConsoleLane, judgments(
		[]convergence.Finding{finding("c1", convergence.SeverityCritical, inA)},
		[]convergence.Finding{finding("c1", convergence.SeverityCritical, inA, fixed), finding("h1", convergence.SeverityHigh, inA)},
		[]convergence.Finding{finding("h1", convergence.SeverityHigh, inA, fixed), finding("h2", convergence.SeverityHigh, inA)},
		[]convergence.Finding{finding("h2", convergence.SeverityHigh, inA)}))
	codeReview := inputOf(convergence.LoopCodeReview, judgments(
		[]convergence.Finding{finding("c1", convergence.SeverityCritical, inA)},
		[]convergence.Finding{finding("c1", convergence.SeverityCritical, inA, fixed), finding("h1", convergence.SeverityHigh, inA)}))
	codeReview.Config = withBudget(1)
	for _, in := range []convergence.Input{console, codeReview} {
		in.Components = certifiedA

		d := decide(t, in)

		requireOutcome(t, d, convergence.ActionAcceptWithLimits, 3)
		if d.LandRound != 1 || !reflect.DeepEqual(d.Defer, []string{"h1"}) {
			t.Fatalf("%s: land round %d defer %q, want round 1, the first round without c1 open, with its own h1 deferred", in.Loop, d.LandRound, d.Defer)
		}
	}
}

func TestExit_AcceptNeedsEveryStrictFindingCertified(t *testing.T) {
	both := []convergence.Finding{finding("h1", convergence.SeverityHigh, ofComponent("X")), finding("h2", convergence.SeverityHigh, ofComponent("Y"))}

	requireOutcome(t, decide(t, rung3Input(convergence.LoopConsoleLane, both, certifiedX)), convergence.ActionStop, 3)
}

func TestExit_SplitAndAcceptBothRefuseWithAnOpenCritical(t *testing.T) {
	cleanOnceXIsExempt := inputFor(convergence.LoopConsoleLane, twoHighInX, join(asFixed(twoHighInX), criticalInX[:1]))
	cleanOnceXIsExempt.Config = withBudget(1)
	for name, facts := range map[string]map[string]convergence.ComponentFacts{"split": separableX, "accept": certifiedX} {
		for _, in := range []convergence.Input{rung3Input(convergence.LoopConsoleLane, criticalInX, facts), cleanOnceXIsExempt} {
			in.Components = facts

			d := decide(t, in)

			requireOutcome(t, d, convergence.ActionStop, 3)
			if d.SplitComponent != "" || len(d.Defer) != 0 {
				t.Fatalf("%s beside an open CRITICAL: %+v, want a plain Stop", name, d)
			}
		}
	}
}

func TestExit_SplitComesBeforeAccept(t *testing.T) {
	requireOutcome(t, decide(t, rung3Input(convergence.LoopConsoleLane, twoHighInX, separableSafeX)), convergence.ActionSplit, 3)
}

func TestExit_StopFilesEveryOpenFindingOfTheKeptRound(t *testing.T) {
	remaining := []convergence.Finding{finding("h", convergence.SeverityHigh), finding("m", convergence.SeverityMedium),
		finding("l", convergence.SeverityLow), finding("i", convergence.SeverityInfo), finding("d", convergence.SeverityHigh, disputed)}

	d := decide(t, rung3Input(convergence.LoopConsoleLane, remaining, nil))

	requireOutcome(t, d, convergence.ActionStop, 3)
	if !reflect.DeepEqual(d.File, []string{"h", "m", "l", "i"}) || len(d.Defer) != 0 || d.LandRound != 1 {
		t.Fatalf("%+v, want every OPEN finding of round 1 filed and nothing deferred", d)
	}
	requireReason(t, d, "CONVERGENCE_STOP", "loop=console-lane", "round=1", "open=4", "land_round=1")
}

func TestExit_LoopsWithoutScopeExitsOnlyStop(t *testing.T) {
	for _, loop := range []convergence.Loop{convergence.LoopAuditRepair, convergence.LoopExplanationReauthor, convergence.LoopCycle, convergence.LoopShipRecovery} {
		in := rung3Input(loop, twoHighInX, separableSafeX)
		in.Config.MaxBackwardEdges = 1
		in.Judge, in.Headroom = deepJudge, v3bTables

		d := decide(t, in)

		requireOutcome(t, d, convergence.ActionStop, 3)
		if d.SplitComponent != "" || len(d.Defer) != 0 {
			t.Fatalf("%s: %+v, want a plain Stop", loop, d)
		}
	}
}

func TestExit_AnInboxItemSplitsButNeverAcceptsWithLimits(t *testing.T) {
	requireOutcome(t, decide(t, rung3Input(convergence.LoopInboxItem, twoHighInX, separableX)), convergence.ActionSplit, 3)
	requireOutcome(t, decide(t, rung3Input(convergence.LoopInboxItem, twoHighInX, certifiedX)), convergence.ActionStop, 3)
}
