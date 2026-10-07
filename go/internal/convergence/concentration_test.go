package convergence_test

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

type spread map[string]int

func roundOf(k int, counts spread, sev convergence.Severity) []convergence.Finding {
	var out []convergence.Finding
	for _, component := range []string{"U", "V", "W", "X", "Y", "Z", "alpha", "beta"} {
		for i := 0; i < counts[component]; i++ {
			out = append(out, finding(fmt.Sprintf("r%d-%s-%d-%s", k, component, i, sev), sev, ofComponent(component)))
		}
	}
	return out
}

func spreads(counts ...spread) []convergence.Judgment {
	rounds := make([][]convergence.Finding, len(counts))
	for k, c := range counts {
		rounds[k] = roundOf(k, c, convergence.SeverityLow)
	}
	return judgments(rounds...)
}

func TestConcentration_Rule5_FiresAtTheThresholdOnTwoConsecutiveWindows(t *testing.T) {
	at := spread{"X": 3, "Y": 2}
	in := inputOf(convergence.LoopConsoleLane, spreads(at, at, at))

	d := decide(t, in)

	requireReason(t, d, "CONVERGENCE_CONCENTRATION", "component=X", "share=0.600", "prev_share=0.600", "window=2")
	if !reflect.DeepEqual(d.Redesign, []string{"X"}) {
		t.Fatalf("redesign %q, want the hot component's follow-up", d.Redesign)
	}

	below := spread{"X": 5, "Y": 4}
	refuseReason(t, decide(t, inputOf(convergence.LoopConsoleLane, spreads(below, below, below))), "CONVERGENCE_CONCENTRATION")
}

func TestConcentration_Rule6_OneWindowAtTheThresholdIsNotEnough(t *testing.T) {
	js := spreads(spread{"U": 1, "V": 1, "W": 1, "Y": 1, "Z": 1}, spread{"X": 3, "Y": 1, "Z": 1}, spread{"X": 5}, spread{"X": 5})

	refuseReason(t, decide(t, inputOf(convergence.LoopConsoleLane, js[:3])), "CONVERGENCE_CONCENTRATION")
	requireReason(t, decide(t, inputOf(convergence.LoopConsoleLane, js)), "CONVERGENCE_CONCENTRATION", "component=X", "share=1.000", "prev_share=0.800")
}

func TestConcentration_Rule6_TheWindowSpansConcentrationWindowRounds(t *testing.T) {
	js := spreads(spread{"X": 5}, spread{"X": 5})
	twoRounds := inputOf(convergence.LoopConsoleLane, js)
	oneRound := inputOf(convergence.LoopConsoleLane, js)
	oneRound.Config.ConcentrationWindow = 1

	refuseReason(t, decide(t, twoRounds), "CONVERGENCE_CONCENTRATION")
	requireReason(t, decide(t, oneRound), "CONVERGENCE_CONCENTRATION", "component=X", "window=1")
}

func TestConcentration_Rule5_AWindowBelowTheMinimumCountIsUndefined(t *testing.T) {
	short := spreads(spread{"X": 2}, spread{"X": 2}, spread{"X": 2})
	enough := spreads(spread{"X": 2}, spread{"X": 3}, spread{"X": 2})

	refuseReason(t, decide(t, inputOf(convergence.LoopConsoleLane, short)), "CONVERGENCE_CONCENTRATION")
	requireReason(t, decide(t, inputOf(convergence.LoopConsoleLane, enough)), "CONVERGENCE_CONCENTRATION", "component=X")
}

func TestConcentration_Rule5_INFOIsExcluded(t *testing.T) {
	rounds := make([][]convergence.Finding, 3)
	for k := range rounds {
		rounds[k] = join(roundOf(k, spread{"X": 2, "Y": 1, "Z": 1, "W": 1}, convergence.SeverityLow), roundOf(k, spread{"X": 3}, convergence.SeverityInfo))
	}

	d := decide(t, inputOf(convergence.LoopConsoleLane, judgments(rounds...)))

	refuseReason(t, d, "CONVERGENCE_CONCENTRATION")
}

func TestConcentration_Rule5_ATieGoesToTheLexicallySmallestComponent(t *testing.T) {
	tie := spread{"beta": 3, "alpha": 3}
	in := inputOf(convergence.LoopConsoleLane, spreads(tie, tie, tie))
	in.Config.ConcentrationThreshold = 0.5

	requireReason(t, decide(t, in), "CONVERGENCE_CONCENTRATION", "component=alpha", "share=0.500")
}

func TestConcentration_TwoWindowsHotOnDifferentComponentsDoNotFire(t *testing.T) {
	js := spreads(spread{"Y": 5}, spread{"Y": 1, "X": 4}, spread{"X": 5})

	refuseReason(t, decide(t, inputOf(convergence.LoopConsoleLane, js)), "CONVERGENCE_CONCENTRATION")
}

func TestConcentration_AFindingOpenInBothRoundsCountsOncePerWindow(t *testing.T) {
	persistent := many("x", 3, convergence.SeverityLow, ofComponent("X"))
	js := judgments(
		roundOf(0, spread{"Z": 5}, convergence.SeverityLow),
		join(persistent, roundOf(1, spread{"Y": 2}, convergence.SeverityLow)),
		join(persistent, roundOf(2, spread{"Y": 2}, convergence.SeverityLow)),
		join(persistent, roundOf(3, spread{"Y": 2}, convergence.SeverityLow)))

	refuseReason(t, decide(t, inputOf(convergence.LoopConsoleLane, js)), "CONVERGENCE_CONCENTRATION")
}

func hotRounds(lastY convergence.Severity) []convergence.Judgment {
	x := func(k int, sev convergence.Severity) []convergence.Finding {
		return many(fmt.Sprintf("x%d", k), 3, sev, ofComponent("X"))
	}
	y := func(k, n int, sev convergence.Severity) []convergence.Finding {
		return many(fmt.Sprintf("y%d", k), n, sev, ofComponent("Y"))
	}
	return judgments(
		join(x(0, convergence.SeverityCritical), y(0, 2, convergence.SeverityCritical)),
		join(asFixed(join(x(0, convergence.SeverityCritical), y(0, 2, convergence.SeverityCritical))), x(1, convergence.SeverityHigh), y(1, 2, convergence.SeverityCritical)),
		join(asFixed(join(x(1, convergence.SeverityHigh), y(1, 2, convergence.SeverityCritical))), x(2, convergence.SeverityHigh), y(2, 1, lastY)))
}

func hotInput(loop convergence.Loop, facts convergence.ComponentFacts, lastY convergence.Severity) convergence.Input {
	in := inputOf(loop, hotRounds(lastY))
	in.Config = withBudget(4)
	in.Components = map[string]convergence.ComponentFacts{"X": facts}
	return in
}

func TestConcentration_SplitsASeparableHotComponentEarlyAndTheRestContinues(t *testing.T) {
	d := decide(t, hotInput(convergence.LoopConsoleLane, convergence.ComponentFacts{Separable: true}, convergence.SeverityHigh))

	requireOutcome(t, d, convergence.ActionContinue, 1)
	if d.SplitComponent != "X" || len(d.Redesign) != 0 || d.LandRound != 2 {
		t.Fatalf("%+v, want X split out (its own item, no redesign row) and round 2 kept", d)
	}
	requireReason(t, d, "CONVERGENCE_CONCENTRATION", "component=X", "share=0.667", "prev_share=0.600")
	requireReason(t, d, "CONVERGENCE_SPLIT", "component=X")
}

func TestConcentration_TheHotComponentSplitsEarlyWhenTheNextRoundIsTheFinalOne(t *testing.T) {
	in := hotInput(convergence.LoopConsoleLane, convergence.ComponentFacts{Separable: true}, convergence.SeverityHigh)
	in.Config = withBudget(3)

	d := decide(t, in)

	requireOutcome(t, d, convergence.ActionContinue, 2)
	if d.SplitComponent != "X" {
		t.Fatalf("%+v, want X split before the final round", d)
	}
}

func earlyAcceptRounds(first []convergence.Finding, hotHigh convergence.Finding) []convergence.Judgment {
	lows := many("p", 5, convergence.SeverityLow, ofComponent("A"))
	return judgments(join(first, lows), join(asFixed(first), []convergence.Finding{hotHigh}, lows))
}

func earlyAcceptInput(js []convergence.Judgment) convergence.Input {
	in := inputOf(convergence.LoopConsoleLane, js)
	in.Config = withBudget(4)
	in.Config.ConcentrationWindow = 1
	in.Components = map[string]convergence.ComponentFacts{"A": {FailSafeCertificate: "TestAFailsSafe"}}
	return in
}

func TestConcentration_AnEarlyAcceptLandsARoundWithoutTheCritical(t *testing.T) {
	js := earlyAcceptRounds([]convergence.Finding{finding("c1", convergence.SeverityCritical, ofComponent("A"))},
		finding("h1", convergence.SeverityHigh, ofComponent("A")))

	d := decide(t, earlyAcceptInput(js))

	requireOutcome(t, d, convergence.ActionLand, 0)
	if d.LandRound != 1 || !reflect.DeepEqual(d.Defer, []string{"h1"}) {
		t.Fatalf("land round %d defer %q, want round 1 with h1 accepted: round 0 holds c1 open", d.LandRound, d.Defer)
	}
}

func TestConcentration_AnEarlyAcceptKeepsTheRoundItsExemptionMakesBest(t *testing.T) {
	js := earlyAcceptRounds([]convergence.Finding{finding("b1", convergence.SeverityHigh, ofComponent("B"))},
		finding("h1", convergence.SeverityHigh, ofComponent("A")))

	d := decide(t, earlyAcceptInput(js))

	requireOutcome(t, d, convergence.ActionLand, 2)
	if d.LandRound != 1 || !slices.Contains(d.Defer, "h1") {
		t.Fatalf("land round %d defer %q, want round 1: with A accepted it holds nothing strict, against round 0's b1", d.LandRound, d.Defer)
	}
}

func TestConcentration_AnEarlySplitThatLeavesNothingStrictLandsTheRest(t *testing.T) {
	in := hotInput(convergence.LoopConsoleLane, convergence.ComponentFacts{Separable: true}, convergence.SeverityLow)

	d := decide(t, in)

	requireOutcome(t, d, convergence.ActionLand, 1)
	if d.SplitComponent != "X" {
		t.Fatalf("split %q, want X", d.SplitComponent)
	}
}

func TestConcentration_AcceptsACertifiedHotComponentEarly(t *testing.T) {
	d := decide(t, hotInput(convergence.LoopConsoleLane, convergence.ComponentFacts{FailSafeCertificate: "TestXFailsSafe"}, convergence.SeverityHigh))

	requireOutcome(t, d, convergence.ActionContinue, 1)
	if !reflect.DeepEqual(d.Defer, []string{"x2a", "x2b", "x2c"}) || !reflect.DeepEqual(d.Redesign, []string{"X"}) {
		t.Fatalf("defer %q redesign %q, want X's strict findings deferred with the certificate and a redesign row", d.Defer, d.Redesign)
	}
	requireReason(t, d, "CONVERGENCE_ACCEPTED_LIMITS", "component=X", "certificate=TestXFailsSafe")
}

func TestConcentration_SignalsWhenNeitherExitFitsAndTheLadderContinues(t *testing.T) {
	d := decide(t, hotInput(convergence.LoopConsoleLane, convergence.ComponentFacts{}, convergence.SeverityHigh))

	requireOutcome(t, d, convergence.ActionContinue, 1)
	if d.SplitComponent != "" || len(d.Defer) != 0 || !reflect.DeepEqual(d.Redesign, []string{"X"}) {
		t.Fatalf("%+v, want only the signal and the redesign follow-up", d)
	}
	requireReason(t, d, "CONVERGENCE_CONCENTRATION", "component=X")
}

func TestConcentration_NeverStopsAConvergingCycleLoop(t *testing.T) {
	d := decide(t, hotInput(convergence.LoopCodeReview, convergence.ComponentFacts{Separable: true}, convergence.SeverityHigh))

	requireOutcome(t, d, convergence.ActionContinue, 1)
	if d.SplitComponent != "" || !reflect.DeepEqual(d.Redesign, []string{"X"}) {
		t.Fatalf("%+v, want no early split (a cycle's split is a Stop) and the redesign follow-up", d)
	}
}

func TestConcentration_AnOpenCriticalRefusesTheEarlyExits(t *testing.T) {
	for _, facts := range []convergence.ComponentFacts{{Separable: true}, {FailSafeCertificate: "TestXFailsSafe"}} {
		d := decide(t, hotInput(convergence.LoopConsoleLane, facts, convergence.SeverityCritical))

		requireOutcome(t, d, convergence.ActionContinue, 1)
		if d.SplitComponent != "" || len(d.Defer) != 0 {
			t.Fatalf("%+v: %+v, want no split and no accept beside an open CRITICAL", facts, d)
		}
	}
}
