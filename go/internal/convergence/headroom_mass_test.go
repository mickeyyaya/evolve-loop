package convergence_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

func TestHeadroom_TodaysTablesHaveNoneSoNoRaiseIsFaked(t *testing.T) {
	in := rung1Input("correctness")
	in.Fixer, in.Judge, in.Headroom = deepJudge, deepJudge, todaysTables

	d := decide(t, in)

	requireOutcome(t, d, convergence.ActionContinue, 1)
	if d.FixerRaise != noRaise || d.JudgeRaise != noRaise {
		t.Fatalf("raises %+v / %+v, want none: deep and top are both opus", d.FixerRaise, d.JudgeRaise)
	}
	requireReason(t, d, "CONVERGENCE_NO_HEADROOM", "role=judge", "family=claude-tmux", "from=deep", "to=top")
	requireReason(t, d, "CONVERGENCE_NO_HEADROOM", "role=fixer", "family=claude-tmux", "from=deep", "to=top")
}

func TestHeadroom_AJudgeAlreadyAtTheTargetsModelAndEffortIsNotRaised(t *testing.T) {
	in := rung1Input("correctness")
	in.Judge = convergence.TierEffort{Family: "claude-tmux", Tier: "deep", Model: "opus", Effort: "xhigh"}

	d := decide(t, in)

	if d.JudgeRaise != noRaise {
		t.Fatalf("judge raise %+v, want none: the judge already ran at opus xhigh", d.JudgeRaise)
	}
	requireReason(t, d, "CONVERGENCE_NO_HEADROOM", "role=judge", "from=deep", "to=top")
}

func TestHeadroom_ARaiseStaysWithinTheJudgesOwnFamily(t *testing.T) {
	in := rung1Input("correctness")
	in.Judge = convergence.TierEffort{Family: "codex-tmux", Tier: "deep", Model: "gpt-5.6-sol"}

	d := decide(t, in)

	if d.JudgeRaise != noRaise {
		t.Fatalf("judge raise %+v, want none: no family's tiers are borrowed", d.JudgeRaise)
	}
	requireReason(t, d, "CONVERGENCE_NO_HEADROOM", "role=judge", "family=codex-tmux", "from=deep", "to=none")
}

func TestHeadroom_TheTopTierHasNoTierAbove(t *testing.T) {
	in := rung1Input("correctness")
	in.Fixer, in.Judge = topJudge, topJudge

	d := decide(t, in)

	if d.FixerRaise != noRaise || d.JudgeRaise != noRaise {
		t.Fatalf("raises %+v / %+v, want none above top", d.FixerRaise, d.JudgeRaise)
	}
	requireReason(t, d, "CONVERGENCE_NO_HEADROOM", "role=fixer", "from=top", "to=none")
}

func TestHeadroom_ARaiseSkipsAMissingTierUpToTheNextOneTheFamilyHas(t *testing.T) {
	in := rung1Input("correctness")
	in.Fixer = convergence.TierEffort{Family: "claude-tmux", Tier: "balanced"}
	top := convergence.ModelEffort{Model: "opus", Effort: "xhigh"}
	in.Headroom = convergence.HeadroomTable{"claude-tmux": {"balanced": {Model: "sonnet"}, "top": top}}

	d := decide(t, in)

	if d.FixerRaise != topJudge {
		t.Fatalf("fixer raise %+v, want top: the family declares no deep tier", d.FixerRaise)
	}
}

func TestMass_TheDefaultWeightsAreCritical8High4Medium2(t *testing.T) {
	in := inputFor(convergence.LoopConsoleLane,
		[]convergence.Finding{finding("m", convergence.SeverityMedium)},
		[]convergence.Finding{finding("m", convergence.SeverityMedium, fixed), finding("c", convergence.SeverityCritical), finding("h", convergence.SeverityHigh)})

	requireReason(t, decide(t, in), "CONVERGENCE_NO_PROGRESS", "loop=console-lane", "round=1", "mass_prev=2", "mass=12", "bar=MEDIUM")
}

func TestMass_ALoopSuppliedMassFuncReplacesTheDefaultAndSeesOnlyTheBar(t *testing.T) {
	seen := map[int][]string{}
	in := inputFor(convergence.LoopCodeReview,
		[]convergence.Finding{finding("h", convergence.SeverityHigh), finding("l", convergence.SeverityLow)},
		[]convergence.Finding{finding("h", convergence.SeverityHigh), finding("g", convergence.SeverityHigh), finding("x", convergence.SeverityHigh, late, refuted)})
	var uOfN convergence.MassFunc = func(round int, atBar []convergence.Finding) float64 {
		var ids []string
		for _, f := range atBar {
			ids = append(ids, f.ID)
		}
		seen[round] = ids
		return float64(10*len(atBar) + round)
	}
	in.Mass = uOfN

	d := decide(t, in)

	requireReason(t, d, "CONVERGENCE_NO_PROGRESS", "mass_prev=10", "mass=21")
	if want := map[int][]string{0: {"h"}, 1: {"h", "g"}}; !reflect.DeepEqual(seen, want) {
		t.Fatalf("the mass function saw %v, want %v: unresolved findings at the bar only", seen, want)
	}
}

func TestReasons_EveryDecisionStartsWithItsRungReason(t *testing.T) {
	inputs := []convergence.Input{
		inputFor(convergence.LoopConsoleLane, []convergence.Finding{finding("l", convergence.SeverityLow)}),
		rung1Input("correctness"),
		inputOf(convergence.LoopCodeReview, rung2Rounds()),
		rung3Input(convergence.LoopConsoleLane, twoHighInX, separableX),
		rung3Input(convergence.LoopCodeReview, twoHighInX, certifiedX),
		rung3Input(convergence.LoopAuditRepair, twoHighInX, nil),
		hotInput(convergence.LoopConsoleLane, convergence.ComponentFacts{}, convergence.SeverityHigh),
	}
	adjudicate := rung3Input(convergence.LoopConsoleLane, twoHighInX, nil)
	adjudicate.Judge, adjudicate.Headroom = deepJudge, v3bTables
	inputs = append(inputs, adjudicate)
	for _, in := range inputs {
		d := decide(t, in)

		if len(d.Reasons) == 0 || !strings.HasPrefix(d.Reasons[0], "CONVERGENCE_RUNG ") {
			t.Fatalf("%s round %d: reasons %q, want the rung reason first", in.Loop, in.Round, d.Reasons)
		}
		want := []string{"loop=" + string(in.Loop), fmt.Sprintf("round=%d", in.Round), fmt.Sprintf("rung=%d", d.Rung),
			"bar=" + string(d.BlockingBar), "action=" + string(d.Action)}
		if !hasFields(d.Reasons[0], want) || !strings.Contains(d.Reasons[0], " cause=") {
			t.Fatalf("rung reason %q, want %q and a cause", d.Reasons[0], want)
		}
	}
}

func TestReasons_TheRungReasonNamesItsCause(t *testing.T) {
	for _, tc := range []struct {
		in    convergence.Input
		cause string
	}{
		{inputFor(convergence.LoopConsoleLane, []convergence.Finding{finding("h", convergence.SeverityHigh)}), "schedule"},
		{func() convergence.Input {
			in := inputOf(convergence.LoopConsoleLane, damageRounds("go/x/x.go:11"))
			in.Config = withBudget(4)
			return in
		}(), "marginal-gain"},
		{func() convergence.Input {
			in := inputOf(convergence.LoopConsoleLane, oscillationRounds(2))
			in.Config = withBudget(5)
			return in
		}(), "oscillation"},
		{rung3Input(convergence.LoopConsoleLane, twoHighInX, nil), "final-round"},
		{loadL2(t), "schedule"},
		{inputOf(convergence.LoopCodeReview, judgments(
			[]convergence.Finding{finding("m1", convergence.SeverityMedium), finding("m2", convergence.SeverityMedium)},
			[]convergence.Finding{finding("m1", convergence.SeverityMedium, fixed), finding("m2", convergence.SeverityMedium)},
			[]convergence.Finding{finding("m2", convergence.SeverityMedium)})), "marginal-gain"},
	} {
		requireReason(t, decide(t, tc.in), "CONVERGENCE_RUNG", "cause="+tc.cause)
	}
}

func reasonOrderInput() convergence.Input {
	inX, inY := ofComponent("X"), ofComponent("Y")
	js := judgments(
		[]convergence.Finding{finding("x1", convergence.SeverityHigh, inX), finding("x2", convergence.SeverityHigh, inX), finding("x3", convergence.SeverityHigh, inX),
			finding("x4", convergence.SeverityLow, inX), finding("y1", convergence.SeverityHigh, inY, ofClass("correctness"))},
		[]convergence.Finding{finding("x1", convergence.SeverityHigh, inX, fixed), finding("x2", convergence.SeverityHigh, inX), finding("x3", convergence.SeverityHigh, inX),
			finding("x4", convergence.SeverityLow, inX), finding("y1", convergence.SeverityHigh, inY, ofClass("correctness")),
			finding("n1", convergence.SeverityMedium, inX, at("go/x/x.go:5")), finding("n2", convergence.SeverityHigh, inX, at("go/x/x.go:6")),
			finding("i", convergence.SeverityInfo)})
	js[1].FixHunks = []convergence.Hunk{{File: "go/x/x.go", From: 1, To: 9}}
	in := inputOf(convergence.LoopConsoleLane, js)
	in.Config = withBudget(4)
	in.Config.ConcentrationWindow = 1
	in.Components = map[string]convergence.ComponentFacts{"X": {FailSafeCertificate: "TestXFailsSafe"}}
	in.Fixer, in.Judge, in.Headroom = deepJudge, deepJudge, todaysTables
	return in
}

func TestReasons_FollowTheRungInAFixedOrder(t *testing.T) {
	stop := rung3Input(convergence.LoopCodeReview, twoHighInX, separableX)
	stop.Judge, stop.Headroom = deepJudge, todaysTables
	for _, tc := range []struct {
		in   convergence.Input
		want []string
	}{
		{reasonOrderInput(), []string{
			"CONVERGENCE_RUNG loop=console-lane round=1 rung=2 bar=HIGH action=Continue cause=marginal-gain",
			"CONVERGENCE_NO_PROGRESS loop=console-lane round=1 mass_prev=16 mass=18 bar=MEDIUM",
			"CONVERGENCE_REPAIR_DAMAGE loop=console-lane round=1 damage=2 repairs=1",
			"CONVERGENCE_CONCENTRATION loop=console-lane component=X share=0.833 prev_share=0.800 window=1",
			"CONVERGENCE_NO_HEADROOM loop=console-lane role=fixer family=claude-tmux from=deep to=top",
			"CONVERGENCE_NO_HEADROOM loop=console-lane role=judge family=claude-tmux from=deep to=top",
			"CONVERGENCE_FILED loop=console-lane late=0 capability=0 info=1",
			"CONVERGENCE_ACCEPTED_LIMITS loop=console-lane component=X certificate=TestXFailsSafe",
			"CONVERGENCE_DEFERRED loop=console-lane count=5 followups=5",
		}},
		{stop, []string{
			"CONVERGENCE_RUNG loop=code-review round=1 rung=3 bar=HIGH action=Stop cause=final-round",
			"CONVERGENCE_NO_HEADROOM loop=code-review role=judge family=claude-tmux from=deep to=top",
			"CONVERGENCE_SPLIT loop=code-review component=X followup=X",
			"CONVERGENCE_STOP loop=code-review round=1 open=2 land_round=1",
		}},
	} {
		d := decide(t, tc.in)

		if !reflect.DeepEqual(d.Reasons, tc.want) {
			t.Fatalf("reasons\n got %q\nwant %q", d.Reasons, tc.want)
		}
	}
}
