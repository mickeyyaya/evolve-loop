package convergence_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

var noRaise convergence.TierEffort

func TestDecide_AJ0WithNothingStrictLandsAtRoundZero(t *testing.T) {
	d := decide(t, inputFor(convergence.LoopConsoleLane, []convergence.Finding{finding("l", convergence.SeverityLow)}))

	requireOutcome(t, d, convergence.ActionLand, 0)
	if d.LandRound != 0 || d.BlockingBar != convergence.SeverityMedium {
		t.Fatalf("land round %d bar %s, want 0 and MEDIUM", d.LandRound, d.BlockingBar)
	}
}

func TestDecide_Rung0_TheFirstFixIsVerifyOnlyForReviewJudgesOnly(t *testing.T) {
	for loop, verifyOnly := range map[convergence.Loop]bool{
		convergence.LoopConsoleLane: true, convergence.LoopCodeReview: true,
		convergence.LoopAuditRepair: false, convergence.LoopExplanationReauthor: false, convergence.LoopCycle: false,
	} {
		in := inputFor(loop, []convergence.Finding{finding("h", convergence.SeverityHigh, ofClass("correctness"))})
		in.Fixer, in.Judge, in.Headroom = deepJudge, deepJudge, v3bTables

		d := decide(t, in)

		requireOutcome(t, d, convergence.ActionContinue, 0)
		if d.VerifyOnly != verifyOnly || d.FreshContext || d.FixerRaise != noRaise || d.JudgeRaise != noRaise {
			t.Fatalf("%s: %+v, want a plain fix with verify-only %v", loop, d, verifyOnly)
		}
	}
}

func rung1Input(class string) convergence.Input {
	in := inputFor(convergence.LoopConsoleLane,
		many("h0", 2, convergence.SeverityHigh),
		join(asFixed(many("h0", 2, convergence.SeverityHigh)), []convergence.Finding{finding("h1", convergence.SeverityHigh, ofClass(class)),
			finding("l1", convergence.SeverityLow)}))
	in.Fixer = convergence.TierEffort{Family: "claude-tmux", Tier: "balanced", Model: "sonnet", Effort: "medium"}
	in.Judge, in.Headroom = deepJudge, v3bTables
	return in
}

func TestDecide_Rung1_ChangesTheFeedback(t *testing.T) {
	d := decide(t, rung1Input("correctness"))

	requireOutcome(t, d, convergence.ActionContinue, 1)
	wantFixer := convergence.TierEffort{Family: "claude-tmux", Tier: "deep", Model: "opus", Effort: "high"}
	if !d.VerifyOnly || d.FreshContext || d.BlockingBar != convergence.SeverityMedium || d.FixerRaise != wantFixer || d.JudgeRaise != topJudge {
		t.Fatalf("%+v, want verify-only, the base bar, the fixer raised one tier and the judge raised to top xhigh", d)
	}
	refuseReason(t, d, "CONVERGENCE_NO_HEADROOM")
	if len(d.Defer) != 0 {
		t.Fatalf("defer %q, want nothing deferred before rung 2: the LOW stays open below the base bar", d.Defer)
	}
}

func TestDecide_Rung1_RaisesTheJudgeOnlyForAReasoningClassBlocker(t *testing.T) {
	for _, class := range []string{"correctness", "concurrency", "architecture"} {
		if d := decide(t, rung1Input(class)); d.JudgeRaise != topJudge {
			t.Fatalf("%s blocker: judge raise %+v, want top", class, d.JudgeRaise)
		}
	}
	for _, class := range []string{"format", "docs", "hygiene", ""} {
		d := decide(t, rung1Input(class))

		if d.JudgeRaise != noRaise || reasonFor(d, "CONVERGENCE_NO_HEADROOM") != "" {
			t.Fatalf("%q blocker: judge raise %+v reasons %q, want no raise attempted", class, d.JudgeRaise, d.Reasons)
		}
		if d.FixerRaise.Tier != "deep" {
			t.Fatalf("%q blocker: fixer raise %+v, want the fixer still raised", class, d.FixerRaise)
		}
	}
}

func TestDecide_Rung1_ABelowBarReasoningFindingDoesNotRaiseTheJudge(t *testing.T) {
	in := rung1Input("format")
	in.Rounds[1].Findings = append(in.Rounds[1].Findings, finding("l2", convergence.SeverityLow, ofClass("correctness")))

	d := decide(t, in)

	requireOutcome(t, d, convergence.ActionContinue, 1)
	if d.JudgeRaise != noRaise {
		t.Fatalf("judge raise %+v, want none: the only strict blocker is a format finding", d.JudgeRaise)
	}
}

func TestDecide_Rung1_TheFixerIsRaisedOnlyOnTheFirstRung1Round(t *testing.T) {
	fast := convergence.TierEffort{Family: "claude-tmux", Tier: "fast", Model: "haiku", Effort: "low"}
	balanced := convergence.TierEffort{Family: "claude-tmux", Tier: "balanced", Model: "sonnet", Effort: "medium"}
	afterRound := func(r int, fixer convergence.TierEffort) convergence.Decision {
		in := inputOf(convergence.LoopConsoleLane, steadyRounds(r))
		in.Config = withBudget(5)
		in.Fixer, in.Judge, in.Headroom = fixer, deepJudge, v3bTables
		return decide(t, in)
	}

	first, second := afterRound(1, fast), afterRound(2, balanced)

	requireOutcome(t, first, convergence.ActionContinue, 1)
	requireOutcome(t, second, convergence.ActionContinue, 1)
	if first.FixerRaise != balanced || second.FixerRaise != noRaise {
		t.Fatalf("fixer raises %+v then %+v, want fast to balanced once and no second raise", first.FixerRaise, second.FixerRaise)
	}
	refuseReason(t, second, "CONVERGENCE_NO_HEADROOM")
}

func rung2Rounds() []convergence.Judgment {
	return judgments(
		many("h0", 3, convergence.SeverityHigh, ofClass("correctness")),
		join(asFixed(many("h0", 3, convergence.SeverityHigh)), many("h1", 2, convergence.SeverityHigh, ofClass("correctness"))),
		join(asFixed(many("h1", 2, convergence.SeverityHigh)), []convergence.Finding{
			finding("h2", convergence.SeverityHigh, ofClass("correctness")), finding("m2", convergence.SeverityMedium),
			finding("l2", convergence.SeverityLow), finding("i2", convergence.SeverityInfo)}))
}

func TestDecide_Rung2_ChangesTheStrategyAndDefersBelowTheRaisedBar(t *testing.T) {
	in := inputOf(convergence.LoopConsoleLane, rung2Rounds())
	in.Fixer, in.Judge, in.Headroom = deepJudge, deepJudge, v3bTables

	d := decide(t, in)

	requireOutcome(t, d, convergence.ActionContinue, 2)
	if !d.FreshContext || d.BlockingBar != convergence.SeverityHigh || d.FixerRaise != noRaise || d.JudgeRaise != noRaise {
		t.Fatalf("%+v, want a fresh context at the HIGH bar and no second feedback raise", d)
	}
	if !reflect.DeepEqual(d.Defer, []string{"m2", "l2"}) || !reflect.DeepEqual(d.File, []string{"i2"}) {
		t.Fatalf("defer %q file %q, want MEDIUM and LOW deferred and the INFO filed", d.Defer, d.File)
	}
	requireReason(t, d, "CONVERGENCE_DEFERRED", "count=2", "followups=2")
}

func TestDecide_Rung2_ACodeReviewLoopDefersButAnAuditRepairOrCycleLoopNever(t *testing.T) {
	for loop, wantDefer := range map[convergence.Loop]bool{
		convergence.LoopCodeReview: true, convergence.LoopAuditRepair: false,
		convergence.LoopExplanationReauthor: false, convergence.LoopCycle: false,
	} {
		in := inputOf(loop, rung2Rounds())

		d := decide(t, in)

		if (len(d.Defer) > 0) != wantDefer {
			t.Fatalf("%s: defer %q, want deferral %v", loop, d.Defer, wantDefer)
		}
	}
}

func TestDecide_Rung2_AnInboxItemOrShipRecoveryLoopNeverDefersAndKeepsTheBaseBar(t *testing.T) {
	for _, loop := range []convergence.Loop{convergence.LoopInboxItem, convergence.LoopShipRecovery} {
		in := inputFor(loop,
			[]convergence.Finding{finding("h", convergence.SeverityHigh), finding("m", convergence.SeverityMedium), finding("l", convergence.SeverityLow)},
			[]convergence.Finding{finding("h", convergence.SeverityHigh, fixed), finding("m", convergence.SeverityMedium), finding("l", convergence.SeverityLow), finding("h2", convergence.SeverityHigh)})
		in.Config = withBudget(2)

		d := decide(t, in)

		requireOutcome(t, d, convergence.ActionContinue, 2)
		if len(d.Defer) != 0 || d.BlockingBar != convergence.SeverityMedium {
			t.Fatalf("%s: defer %q bar %s, want nothing deferred at the base bar", loop, d.Defer, d.BlockingBar)
		}
	}
}

func TestDecide_Rung2_TheCycleLoopChangesItsContextButKeepsItsBaseBar(t *testing.T) {
	js := judgments(
		[]convergence.Finding{finding("h", convergence.SeverityHigh), finding("m", convergence.SeverityMedium)},
		[]convergence.Finding{finding("h", convergence.SeverityHigh, fixed), finding("m", convergence.SeverityMedium), finding("h2", convergence.SeverityHigh)},
		[]convergence.Finding{finding("h2", convergence.SeverityHigh, fixed), finding("m", convergence.SeverityMedium)})

	final := decide(t, inputOf(convergence.LoopCycle, js[:2]))

	requireOutcome(t, final, convergence.ActionContinue, 2)
	if !final.FreshContext || final.BlockingBar != convergence.SeverityMedium {
		t.Fatalf("the cycle's rung 2: %+v, want a fresh context at the base MEDIUM bar", final)
	}

	after := decide(t, inputOf(convergence.LoopCycle, js))

	requireOutcome(t, after, convergence.ActionStop, 3)
	if after.BlockingBar != convergence.SeverityMedium || !reflect.DeepEqual(after.File, []string{"m"}) || after.LandRound != 2 {
		t.Fatalf("after the cycle's final round: %+v, want a Stop at MEDIUM that files the open MEDIUM, never a land that drops it", after)
	}
}

func TestDecide_Rung2_AFinalRoundWithNoRung1RoundCarriesTheFeedbackChange(t *testing.T) {
	in := inputOf(convergence.LoopConsoleLane, rung2Rounds()[:2])
	in.Config = withBudget(2)
	in.Fixer, in.Judge, in.Headroom = deepJudge, deepJudge, v3bTables

	d := decide(t, in)

	requireOutcome(t, d, convergence.ActionContinue, 2)
	if !d.FreshContext || d.JudgeRaise != topJudge || d.FixerRaise != topJudge {
		t.Fatalf("%+v, want the fresh context combined with the fixer and judge raises", d)
	}
}

func TestDecide_Rung3_FollowsTheFinalRoundWithAnExitNeverAnotherRound(t *testing.T) {
	js := append(rung2Rounds(), convergence.Judgment{Index: 3, Findings: []convergence.Finding{
		finding("h2", convergence.SeverityHigh), finding("m2", convergence.SeverityMedium, withStatus(convergence.StatusDeferred))}})

	d := decide(t, inputOf(convergence.LoopConsoleLane, js))

	requireOutcome(t, d, convergence.ActionStop, 3)
}

func TestDecide_MaxFixRoundsIsNeverExceeded(t *testing.T) {
	for _, budget := range []int{1, 2, 3, 4} {
		for r := budget; r <= budget+4; r++ {
			in := inputOf(convergence.LoopConsoleLane, steadyRounds(r))
			in.Config = withBudget(budget)

			d := decide(t, in)

			if d.Action == convergence.ActionContinue || d.Rung != 3 {
				t.Fatalf("N=%d after round %d: %s at rung %d, want an exit at rung 3", budget, r, d.Action, d.Rung)
			}
		}
	}
	l2Length := inputOf(convergence.LoopConsoleLane, steadyRounds(7))
	requireOutcome(t, decide(t, l2Length), convergence.ActionStop, 3)
}

func damageRounds(newLocation string) []convergence.Judgment {
	js := judgments(
		[]convergence.Finding{finding("a", convergence.SeverityCritical), finding("b", convergence.SeverityHigh)},
		[]convergence.Finding{finding("a", convergence.SeverityCritical, fixed), finding("b", convergence.SeverityHigh),
			finding("c", convergence.SeverityMedium, at(newLocation))})
	js[1].FixHunks = []convergence.Hunk{{File: "go/x/x.go", From: 10, To: 12}}
	return js
}

func TestTrigger_RepairDamageAtLeastTheRepairsSkipsToTheStrategyChange(t *testing.T) {
	in := inputOf(convergence.LoopConsoleLane, damageRounds("go/x/x.go:11"))
	in.Config = withBudget(4)

	d := decide(t, in)

	requireOutcome(t, d, convergence.ActionContinue, 2)
	requireReason(t, d, "CONVERGENCE_REPAIR_DAMAGE", "damage=1", "repairs=1")
	refuseReason(t, d, "CONVERGENCE_NO_PROGRESS")
}

func TestTrigger_OnlyANewFindingInsideTheFixHunksIsDamage(t *testing.T) {
	for location, isDamage := range map[string]bool{
		"go/x/x.go:10": true, "go/x/x.go:12": true, "go/x/x.go:9": false, "go/x/x.go:13": false,
		"go/y/x.go:11": false, "go/x/x.go": false, "go/x/x.go:11x": false, "": false,
	} {
		in := inputOf(convergence.LoopConsoleLane, damageRounds(location))
		in.Config = withBudget(4)

		d := decide(t, in)

		if got := reasonFor(d, "CONVERGENCE_REPAIR_DAMAGE") != ""; got != isDamage || (d.Rung == 2) != isDamage {
			t.Fatalf("location %q: damage %v at rung %d, want damage %v", location, got, d.Rung, isDamage)
		}
	}
}

func TestTrigger_AFindingStillOpenInsideTheFixHunksIsNotDamage(t *testing.T) {
	js := judgments(
		[]convergence.Finding{finding("a", convergence.SeverityMedium, at("go/x/x.go:5")), finding("b", convergence.SeverityHigh)},
		[]convergence.Finding{finding("a", convergence.SeverityMedium, at("go/x/x.go:5")), finding("b", convergence.SeverityHigh, fixed)})
	js[1].FixHunks = []convergence.Hunk{{File: "go/x/x.go", From: 1, To: 9}}

	d := decide(t, inputOf(convergence.LoopCodeReview, js))

	requireOutcome(t, d, convergence.ActionContinue, 1)
	refuseReason(t, d, "CONVERGENCE_REPAIR_DAMAGE")
}

func reopenRounds(reopen bool) []convergence.Judgment {
	aStatus := mod(fixed)
	if reopen {
		aStatus = withStatus(convergence.StatusOpen)
	}
	return judgments(
		[]convergence.Finding{finding("a", convergence.SeverityLow), finding("b", convergence.SeverityCritical),
			finding("c", convergence.SeverityHigh), finding("p", convergence.SeverityHigh)},
		[]convergence.Finding{finding("a", convergence.SeverityLow, fixed), finding("b", convergence.SeverityCritical, fixed),
			finding("c", convergence.SeverityHigh), finding("p", convergence.SeverityHigh)},
		[]convergence.Finding{finding("a", convergence.SeverityLow, aStatus), finding("c", convergence.SeverityHigh, fixed),
			finding("p", convergence.SeverityHigh)})
}

func TestTrigger_AReopenedFixedFindingIsRepairDamage(t *testing.T) {
	for reopen, wantRung := range map[bool]int{true: 2, false: 1} {
		in := inputOf(convergence.LoopConsoleLane, reopenRounds(reopen))
		in.Config = withBudget(4)

		d := decide(t, in)

		requireOutcome(t, d, convergence.ActionContinue, wantRung)
	}
}

func TestTrigger_NoProgressAtTheCurrentBarSkipsToTheStrategyChange(t *testing.T) {
	for _, tc := range []struct {
		name     string
		second   []convergence.Finding
		wantRung int
	}{
		{"no progress", []convergence.Finding{finding("a", convergence.SeverityHigh, fixed), finding("b", convergence.SeverityHigh), finding("c", convergence.SeverityHigh)}, 2},
		{"progress", []convergence.Finding{finding("a", convergence.SeverityHigh, fixed), finding("b", convergence.SeverityHigh), finding("c", convergence.SeverityMedium)}, 1},
	} {
		in := inputFor(convergence.LoopConsoleLane,
			[]convergence.Finding{finding("a", convergence.SeverityHigh), finding("b", convergence.SeverityHigh)}, tc.second)
		in.Config = withBudget(4)

		d := decide(t, in)

		requireOutcome(t, d, convergence.ActionContinue, tc.wantRung)
		if (reasonFor(d, "CONVERGENCE_NO_PROGRESS") != "") != (tc.wantRung == 2) {
			t.Fatalf("%s: reasons %q", tc.name, d.Reasons)
		}
	}
}

func TestTrigger_MarginalGainOnTheFinalRoundGoesToRung3(t *testing.T) {
	in := inputOf(convergence.LoopConsoleLane, damageRounds("go/x/x.go:11"))
	in.Config = withBudget(1)

	d := decide(t, in)

	requireOutcome(t, d, convergence.ActionStop, 3)
	requireReason(t, d, "CONVERGENCE_REPAIR_DAMAGE", "damage=1", "repairs=1")
}

func oscillationRounds(reopenings int) []convergence.Judgment {
	pool := many("p", 10, convergence.SeverityHigh)
	xStatus := []convergence.Status{convergence.StatusOpen, convergence.StatusFixed, convergence.StatusOpen, convergence.StatusFixed, convergence.StatusFixed}
	if reopenings == 2 {
		xStatus[4] = convergence.StatusOpen
	}
	rounds := make([][]convergence.Finding, len(xStatus))
	for k := range rounds {
		rounds[k] = join(asFixed(pool[:2*k]), pool[2*k:], []convergence.Finding{finding("x", convergence.SeverityLow, withStatus(xStatus[k]))})
	}
	return judgments(rounds...)
}

func TestTrigger_AnIDReopenedAfterFixedTwiceJumpsToRung3(t *testing.T) {
	for reopenings, wantRung := range map[int]int{2: 3, 1: 2} {
		in := inputOf(convergence.LoopConsoleLane, oscillationRounds(reopenings))
		in.Config = withBudget(5)

		d := decide(t, in)

		if d.Rung != wantRung {
			t.Fatalf("%d reopenings: rung %d, want %d (reasons %q)", reopenings, d.Rung, wantRung, d.Reasons)
		}
	}
}

func fingerprintRounds(edges ...string) []convergence.Judgment {
	js := steadyRounds(len(edges) - 1)
	for k, edge := range edges {
		js[k].Fingerprint, js[k].Edge = "fp-audit-red", edge
	}
	return js
}

func TestTrigger_OneFingerprintBehindTwoDifferentBackwardEdgesJumpsToRung3(t *testing.T) {
	for _, tc := range []struct {
		edges    []string
		wantRung int
	}{
		{[]string{"audit->build", "audit->build", "retro->tdd"}, 3},
		{[]string{"audit->build", "audit->build", "audit->build"}, 1},
	} {
		in := inputOf(convergence.LoopCycle, fingerprintRounds(tc.edges...))
		in.Config.MaxBackwardEdges = 5

		d := decide(t, in)

		if d.Rung != tc.wantRung {
			t.Fatalf("edges %q: rung %d, want %d (reasons %q)", tc.edges, d.Rung, tc.wantRung, d.Reasons)
		}
	}
	distinct := inputOf(convergence.LoopCycle, fingerprintRounds("audit->build", "audit->build", "retro->tdd"))
	distinct.Rounds[2].Fingerprint = "fp-retro-red"
	distinct.Config.MaxBackwardEdges = 5

	if d := decide(t, distinct); d.Rung != 1 {
		t.Fatalf("two fingerprints: rung %d, want 1", d.Rung)
	}
}
