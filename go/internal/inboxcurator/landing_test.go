package inboxcurator

import (
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func testsIn(names ...string) func(string) bool {
	return func(name string) bool { return slices.Contains(names, name) }
}

var landedAt = time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)

func poolOpenItem() inboxbatch.Item {
	it := item("rolling-pool", "A free lane takes the next ready task", []string{"go/internal/fleet/pool.go"},
		"`TestOrchestrator_AFreeSlotTakesTheNextAdmissibleTodoAtOnce` passes: a ship.landed event backfills the slot.",
		"`TestLease_ALanePastItsDeadlineStops` passes.")
	it.Path = "2026-10-17T03-40-00Z-rolling-pool.json"
	return it
}

func TestNamedTests_ListsEachTestNameAndWhetherEveryCriterionNamesOne(t *testing.T) {
	names, every := NamedTests(poolOpenItem())
	if want := []string{"TestOrchestrator_AFreeSlotTakesTheNextAdmissibleTodoAtOnce", "TestLease_ALanePastItsDeadlineStops"}; !slices.Equal(names, want) || !every {
		t.Errorf("NamedTests = %v, %v; want %v, true", names, every, want)
	}
	mixed := item("m", "", nil, "TestA passes", "the utilization is at least 0.85", "TestA still passes", "Testing is fun")
	if names, every := NamedTests(mixed); !slices.Equal(names, []string{"TestA"}) || every {
		t.Errorf("NamedTests(mixed) = %v, %v; want [TestA], false", names, every)
	}
}

func TestMatchLanding_ProposesADeterministicConsumeWhenTheSurfaceChangedAndEveryNamedTestExists(t *testing.T) {
	change := Change{Commit: "abc123", Paths: []string{"go/internal/fleet/pool.go", "go/internal/fleet/pool_test.go"}, Tests: []string{"TestLease_ALanePastItsDeadlineStops"}, When: landedAt}
	all := testsIn("TestOrchestrator_AFreeSlotTakesTheNextAdmissibleTodoAtOnce", "TestLease_ALanePastItsDeadlineStops")

	got := MatchLanding(change, []inboxbatch.Item{poolOpenItem()}, all, 0.5)

	if len(got) != 1 || got[0].Kind != ProposeConsume || got[0].ID != "rolling-pool" || !got[0].Deterministic || got[0].Trigger != TriggerLanding {
		t.Fatalf("MatchLanding = %+v, want one deterministic consume of rolling-pool", got)
	}
	if !slices.Contains(got[0].Evidence, "commit abc123") || got[0].Path != poolOpenItem().Path {
		t.Errorf("the proposal names its commit and path, got %+v", got[0])
	}
}

func TestMatchLanding_AHalfMetItemIsAProposalForTheOperatorOnly(t *testing.T) {
	change := Change{Commit: "abc123", Paths: []string{"go/internal/fleet/pool.go"}}

	got := MatchLanding(change, []inboxbatch.Item{poolOpenItem()}, testsIn("TestOrchestrator_AFreeSlotTakesTheNextAdmissibleTodoAtOnce"), 0.5)

	if len(got) != 1 || got[0].Deterministic {
		t.Fatalf("MatchLanding = %+v, want one consume proposal that is not deterministic", got)
	}
}

func TestMatchLanding_ProposesNothingBelowTheShareOrOffTheSurface(t *testing.T) {
	for name, tc := range map[string]struct {
		change Change
		tests  func(string) bool
	}{
		"no named test exists":       {Change{Commit: "a", Paths: []string{"go/internal/fleet/pool.go"}}, testsIn()},
		"the surface did not change": {Change{Commit: "a", Paths: []string{"docs/x.md"}}, testsIn("TestOrchestrator_AFreeSlotTakesTheNextAdmissibleTodoAtOnce", "TestLease_ALanePastItsDeadlineStops")},
		"an item that names no test": {Change{Commit: "a", Paths: []string{"go/internal/fleet/pool.go"}}, testsIn("TestX")},
	} {
		open := []inboxbatch.Item{poolOpenItem()}
		if name == "an item that names no test" {
			open = []inboxbatch.Item{item("plain", "x", []string{"go/internal/fleet/pool.go"}, "the pool is faster")}
		}
		if got := MatchLanding(tc.change, open, tc.tests, 0.5); len(got) != 0 {
			t.Errorf("%s: MatchLanding = %+v, want none", name, got)
		}
	}
}

func TestMatchLanding_TheChangeTextNamingAnItemIsAProposal(t *testing.T) {
	change := Change{Commit: "c1", Paths: []string{"docs/a.md"}, Text: "feat: wire the pool (closes rolling-pool)"}

	got := MatchLanding(change, []inboxbatch.Item{poolOpenItem()}, testsIn(), 0.5)

	if len(got) != 1 || got[0].Deterministic || got[0].ID != "rolling-pool" {
		t.Fatalf("MatchLanding = %+v, want one consume proposal from the text", got)
	}
}

func TestMatchLanding_ARootCopyOfAnItemTheChangeConsumedIsADeterministicConsume(t *testing.T) {
	change := Change{Commit: "c2", Paths: []string{".evolve/inbox/consumed/2026-10-17T03-40-00Z-rolling-pool.json"}}
	pool := item("pool", "other work", []string{"go/x.go"})
	pool.Path = "2026-10-18T00-00-00Z-pool.json"

	got := MatchLanding(change, []inboxbatch.Item{poolOpenItem(), pool}, testsIn(), 0.5)

	if len(got) != 1 || !got[0].Deterministic || got[0].Kind != ProposeConsume || got[0].ID != "rolling-pool" {
		t.Fatalf("MatchLanding = %+v, want one deterministic consume of rolling-pool and none of pool", got)
	}
}

func TestSweep_ProposesEveryItemWhoseNamedTestsExistAndNeverAppliesOne(t *testing.T) {
	stale := poolOpenItem()
	fresh := item("fresh", "x", []string{"go/a.go"}, "TestNotYetWritten passes")
	all := testsIn("TestOrchestrator_AFreeSlotTakesTheNextAdmissibleTodoAtOnce", "TestLease_ALanePastItsDeadlineStops")

	got := Sweep([]inboxbatch.Item{stale, fresh}, all, 0.5)

	if len(got) != 1 || got[0].ID != "rolling-pool" || got[0].Deterministic || got[0].Trigger != TriggerSweep {
		t.Fatalf("Sweep = %+v, want one proposal for rolling-pool that is not deterministic", got)
	}
}

func TestMatchLanding_ADeterministicConsumeNeedsANamedTestTheChangeDefines(t *testing.T) {
	all := testsIn("TestOrchestrator_AFreeSlotTakesTheNextAdmissibleTodoAtOnce", "TestLease_ALanePastItsDeadlineStops")
	surface := []string{"go/internal/fleet/pool.go"}

	for name, c := range map[string]Change{
		"the change defines no named test": {Commit: "a", Paths: surface},
		"the change predates the item":     {Commit: "b", Paths: surface, Tests: []string{"TestLease_ALanePastItsDeadlineStops"}, When: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
	} {
		got := MatchLanding(c, []inboxbatch.Item{poolOpenItem()}, all, 0.5)
		if len(got) != 1 || got[0].Deterministic {
			t.Errorf("%s: MatchLanding = %+v, want a proposal for the operator only", name, got)
		}
	}
}
