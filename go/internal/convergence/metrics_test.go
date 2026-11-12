package convergence_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

func lateHigh(id string, mods ...mod) convergence.Finding {
	return finding(id, convergence.SeverityHigh, append([]mod{late}, mods...)...)
}

func consoleLaneThreeRounds() convergence.Input {
	return inputFor(convergence.LoopConsoleLane,
		[]convergence.Finding{
			finding("d1", convergence.SeverityMedium),
			finding("d2", convergence.SeverityMedium),
			lateHigh("h1"),
		},
		[]convergence.Finding{
			finding("d1", convergence.SeverityMedium, withStatus(convergence.StatusDeferred)),
			finding("d2", convergence.SeverityMedium, withStatus(convergence.StatusDeferred)),
			lateHigh("h1", survived),
		},
		[]convergence.Finding{
			finding("d1", convergence.SeverityMedium, fixed),
			finding("d2", convergence.SeverityMedium, withStatus(convergence.StatusDeferred)),
			lateHigh("h1", survived, fixed),
		},
	)
}

func consoleLaneTwoRounds() convergence.Input {
	return inputFor(convergence.LoopConsoleLane,
		[]convergence.Finding{
			lateHigh("x"),
			lateHigh("y"),
			lateHigh("z"),
		},
		[]convergence.Finding{
			lateHigh("x", refuted, withStatus(convergence.StatusFiled)),
			lateHigh("y", refuted, withStatus(convergence.StatusFiled)),
			lateHigh("z", survived),
		},
	)
}

func codeReviewOneRound() convergence.Input {
	return inputFor(convergence.LoopCodeReview,
		[]convergence.Finding{finding("a", convergence.SeverityLow)},
		[]convergence.Finding{finding("a", convergence.SeverityLow, fixed)},
	)
}

func TestMeasure_RoundsToLandAreGroupedPerLoopWithMedianAndMax(t *testing.T) {
	got := convergence.Measure([]convergence.Input{consoleLaneThreeRounds(), consoleLaneTwoRounds(), codeReviewOneRound()})

	want := []convergence.LoopRounds{
		{Loop: convergence.LoopCodeReview, Runs: 1, Median: 1, Max: 1},
		{Loop: convergence.LoopConsoleLane, Runs: 2, Median: 1.5, Max: 2},
	}
	if !reflect.DeepEqual(got.RoundsToLand, want) {
		t.Fatalf("RoundsToLand = %+v, want %+v", got.RoundsToLand, want)
	}
}

func TestMeasure_AnOddRunCountTakesTheMiddleRound(t *testing.T) {
	three := inputFor(convergence.LoopCycle, []convergence.Finding{}, []convergence.Finding{}, []convergence.Finding{}, []convergence.Finding{})
	one := inputFor(convergence.LoopCycle, []convergence.Finding{})
	two := inputFor(convergence.LoopCycle, []convergence.Finding{}, []convergence.Finding{}, []convergence.Finding{})

	got := convergence.Measure([]convergence.Input{three, one, two})

	want := []convergence.LoopRounds{{Loop: convergence.LoopCycle, Runs: 3, Median: 2, Max: 3}}
	if !reflect.DeepEqual(got.RoundsToLand, want) {
		t.Fatalf("RoundsToLand = %+v, want %+v", got.RoundsToLand, want)
	}
}

func TestMeasure_TheDeferredThenClosedShareCountsADeferralClosedByALaterFix(t *testing.T) {
	got := convergence.Measure([]convergence.Input{consoleLaneThreeRounds(), consoleLaneTwoRounds()})

	want := convergence.DeferredClosed{Deferred: 2, Closed: 1, Share: 0.5}
	if got.DeferredClosed != want {
		t.Fatalf("DeferredClosed = %+v, want %+v", got.DeferredClosed, want)
	}
}

func TestMeasure_ADeferralThatVanishesOrIsOnlyFiledIsNotClosed(t *testing.T) {
	vanishes := inputFor(convergence.LoopConsoleLane,
		[]convergence.Finding{finding("gone", convergence.SeverityMedium, withStatus(convergence.StatusDeferred))},
		[]convergence.Finding{},
	)
	filedOnly := inputFor(convergence.LoopConsoleLane,
		[]convergence.Finding{finding("moved", convergence.SeverityLow, withStatus(convergence.StatusDeferred))},
		[]convergence.Finding{finding("moved", convergence.SeverityLow, withStatus(convergence.StatusFiled))},
	)
	fixedBeforeDeferral := inputFor(convergence.LoopConsoleLane,
		[]convergence.Finding{finding("early", convergence.SeverityLow, fixed)},
		[]convergence.Finding{finding("early", convergence.SeverityLow, withStatus(convergence.StatusDeferred))},
	)

	got := convergence.Measure([]convergence.Input{vanishes, filedOnly, fixedBeforeDeferral})

	want := convergence.DeferredClosed{Deferred: 3, Closed: 0, Share: 0}
	if got.DeferredClosed != want {
		t.Fatalf("DeferredClosed = %+v, want %+v", got.DeferredClosed, want)
	}
}

func TestMeasure_AFindingDeferredInTwoRoundsCountsOnce(t *testing.T) {
	in := inputFor(convergence.LoopConsoleLane,
		[]convergence.Finding{finding("twice", convergence.SeverityLow, withStatus(convergence.StatusDeferred))},
		[]convergence.Finding{finding("twice", convergence.SeverityLow, withStatus(convergence.StatusDeferred))},
		[]convergence.Finding{finding("twice", convergence.SeverityLow, fixed)},
	)

	got := convergence.Measure([]convergence.Input{in})

	want := convergence.DeferredClosed{Deferred: 1, Closed: 1, Share: 1}
	if got.DeferredClosed != want {
		t.Fatalf("DeferredClosed = %+v, want %+v", got.DeferredClosed, want)
	}
}

func TestMeasure_TheFalsificationDropRateIsRefutedOverChecked(t *testing.T) {
	got := convergence.Measure([]convergence.Input{consoleLaneThreeRounds(), consoleLaneTwoRounds()})

	want := convergence.FalsificationDrop{Checked: 4, Refuted: 2, Rate: 0.5}
	if got.Falsification != want {
		t.Fatalf("Falsification = %+v, want %+v", got.Falsification, want)
	}
}

func TestMeasure_AFindingIsCheckedOnceByItsLatestResult(t *testing.T) {
	in := inputFor(convergence.LoopConsoleLane,
		[]convergence.Finding{lateHigh("flip", survived)},
		[]convergence.Finding{lateHigh("flip", refuted, withStatus(convergence.StatusFiled))},
	)

	got := convergence.Measure([]convergence.Input{in})

	want := convergence.FalsificationDrop{Checked: 1, Refuted: 1, Rate: 1}
	if got.Falsification != want {
		t.Fatalf("Falsification = %+v, want %+v", got.Falsification, want)
	}
}

func TestMeasure_NoInputYieldsZeroRatesNotNaN(t *testing.T) {
	got := convergence.Measure(nil)

	if len(got.RoundsToLand) != 0 {
		t.Fatalf("RoundsToLand = %+v, want none", got.RoundsToLand)
	}
	if got.DeferredClosed != (convergence.DeferredClosed{}) || got.Falsification != (convergence.FalsificationDrop{}) {
		t.Fatalf("DeferredClosed = %+v, Falsification = %+v, want zero values", got.DeferredClosed, got.Falsification)
	}
}

func TestMeasure_TheMetricsEncodeUnderTheDocumentedKeys(t *testing.T) {
	m := convergence.Measure([]convergence.Input{consoleLaneThreeRounds(), consoleLaneTwoRounds()})

	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var rounds struct {
		RoundsToLand []map[string]any `json:"rounds_to_land"`
	}
	if err := json.Unmarshal(raw, &rounds); err != nil {
		t.Fatal(err)
	}
	if len(rounds.RoundsToLand) != 1 {
		t.Fatalf("rounds_to_land = %+v in %s", rounds.RoundsToLand, raw)
	}
	for _, key := range []string{"loop", "runs", "median", "max"} {
		if _, ok := rounds.RoundsToLand[0][key]; !ok {
			t.Errorf("rounds_to_land entry lacks key %q: %s", key, raw)
		}
	}
	var shape struct {
		DeferredClosed map[string]any `json:"deferred_closed"`
		Falsification  map[string]any `json:"falsification"`
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"deferred", "closed", "share"} {
		if _, ok := shape.DeferredClosed[key]; !ok {
			t.Errorf("deferred_closed lacks key %q: %s", key, raw)
		}
	}
	for _, key := range []string{"checked", "refuted", "drop_rate"} {
		if _, ok := shape.Falsification[key]; !ok {
			t.Errorf("falsification lacks key %q: %s", key, raw)
		}
	}
}
