package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// TestPlanMismatch_TriggersOnlyOnMaterialDivergence pins WS2-S4 (ADR-0052) with a
// boundary table: "tester" inserts when scout.item_count >= 5. A mismatch exists
// only when the trigger FIRES on the measured signals AND the plan omits that
// phase. N=4 (below threshold) is not a mismatch; N=5 (fires) with tester
// unscheduled is; N=5 with tester already scheduled is not (need covered, no churn).
func TestPlanMismatch_TriggersOnlyOnMaterialDivergence(t *testing.T) {
	t.Parallel()
	in := func(itemCount int) RouteInput {
		return RouteInput{
			Cfg: config.RoutingConfig{Triggers: map[string]config.RoutingBlock{
				"tester": {InsertWhen: []config.Condition{{Field: "scout.item_count", Op: "gte", Value: 5}}},
			}},
			Signals: RoutingSignals{Scout: ScoutSignals{Present: true, ItemCount: itemCount}},
		}
	}
	planNoTester := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "scout", Run: true}, {Phase: "build", Run: true}}}
	planWithTester := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "scout", Run: true}, {Phase: "tester", Run: true}}}

	if PlanMismatch(in(4), planNoTester) {
		t.Error("item_count=4 (below threshold) must NOT be a mismatch")
	}
	if !PlanMismatch(in(5), planNoTester) {
		t.Error("item_count=5 (trigger fires) + tester unscheduled must be a mismatch")
	}
	if PlanMismatch(in(5), planWithTester) {
		t.Error("trigger fires but plan already runs tester ⇒ NOT a mismatch (no churn)")
	}
	if PlanMismatch(in(5), nil) {
		t.Error("nil plan ⇒ no mismatch")
	}
}
