package phasespec

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func harnessMakes(fields ...string) func(string) bool {
	set := map[string]bool{}
	for _, f := range fields {
		set[f] = true
	}
	return func(field string) bool { return set[field] }
}

func insertWhen(fields ...string) config.RoutingBlock {
	var conds []config.Condition
	for _, f := range fields {
		conds = append(conds, config.Condition{Field: f, Op: "==", Value: "x"})
	}
	return config.RoutingBlock{InsertWhen: conds}
}

func TestUnproducedTriggerFields_AFieldNothingProducesNamesThePhaseAndTheField(t *testing.T) {
	t.Parallel()
	cfg := config.RoutingConfig{
		Order:    []string{"scout", "build", "release-notes"},
		Triggers: map[string]config.RoutingBlock{"release-notes": insertWhen("ship.flavor")},
	}
	got := UnproducedTriggerFields(cfg, nil, harnessMakes("scout.goal_type"))
	want := []string{"phase release-notes insert_when field ship.flavor has no producer: no typed signal, and no phase before release-notes declares it in outputs.signals"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UnproducedTriggerFields = %q, want %q", got, want)
	}
}

func TestUnproducedTriggerFields_ATypedSignalIsAProducer(t *testing.T) {
	t.Parallel()
	cfg := config.RoutingConfig{
		Order:    []string{"scout", "release-notes"},
		Triggers: map[string]config.RoutingBlock{"release-notes": insertWhen("scout.goal_type")},
	}
	if got := UnproducedTriggerFields(cfg, nil, harnessMakes("scout.goal_type")); len(got) != 0 {
		t.Fatalf("a typed signal is produced by the harness, got %q", got)
	}
}

func TestUnproducedTriggerFields_OnlyAnEarlierPhaseOutputIsAProducer(t *testing.T) {
	t.Parallel()
	specs := []PhaseSpec{
		{Name: "amplify-tests", Outputs: IO{Signals: []string{"amplify.tests_added", "ratio"}}},
		{Name: "late-check", Outputs: IO{Signals: []string{"late.score"}}},
	}
	cfg := config.RoutingConfig{
		Order: []string{"scout", "amplify-tests", "mutation-check", "late-check"},
		Triggers: map[string]config.RoutingBlock{
			"mutation-check": insertWhen("amplify.tests_added", "amplify-tests.ratio", "late.score"),
		},
	}
	got := UnproducedTriggerFields(cfg, specs, harnessMakes())
	want := []string{"phase mutation-check insert_when field late.score has no producer: no typed signal, and no phase before mutation-check declares it in outputs.signals"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UnproducedTriggerFields = %q, want %q (a dotted output is verbatim, a bare one is phase-prefixed, a later phase does not count)", got, want)
	}
}

func TestUnproducedTriggerFields_ChecksSkipWhenAndAPhaseOutsideTheOrder(t *testing.T) {
	t.Parallel()
	specs := []PhaseSpec{{Name: "scout", Outputs: IO{Signals: []string{"scout.risk"}}}}
	cfg := config.RoutingConfig{
		Order: []string{"scout", "build"},
		Triggers: map[string]config.RoutingBlock{
			"build":    {SkipWhen: []config.Condition{{Field: "build.ghost", Op: "==", Value: true}}},
			"unplaced": insertWhen("scout.risk"),
		},
	}
	got := UnproducedTriggerFields(cfg, specs, harnessMakes())
	want := []string{
		"phase build skip_when field build.ghost has no producer: no typed signal, and no phase before build declares it in outputs.signals",
		"phase unplaced insert_when field scout.risk has no producer: no typed signal, and no phase before unplaced declares it in outputs.signals",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UnproducedTriggerFields = %q, want %q", got, want)
	}
}
