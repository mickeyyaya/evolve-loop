package inboxcurator

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestStaleClaims_ProposeAJudgmentQuarantineOnlyForTheItemTheHookNames(t *testing.T) {
	items := []inboxbatch.Item{{ID: "stale"}, {ID: "fresh"}}

	got := StaleClaims(items, func(it inboxbatch.Item) string {
		if it.ID == "stale" {
			return "the fix is on main"
		}
		return ""
	})

	if len(got) != 1 || got[0].ID != "stale" || got[0].Kind != ProposeQuarantine || got[0].Deterministic {
		t.Fatalf("StaleClaims = %+v, want one non-deterministic quarantine for stale", got)
	}
	if !slices.Equal(got[0].Evidence, []string{"the fix is on main"}) {
		t.Errorf("evidence = %v", got[0].Evidence)
	}
	if StaleClaims(items, nil) != nil {
		t.Error("a nil hook must propose nothing")
	}
}

func TestReroutes_WithoutAProtectedPredicateTheTextCheckStaysOffAndTheItemIsRerouted(t *testing.T) {
	it := inboxbatch.Item{ID: "x", Kind: "bug", Route: "console-manual", Files: []string{"go/internal/fleet/pool.go"}}

	got := Reroutes([]inboxbatch.Item{it}, RerouteOptions{BuildableKinds: []string{"bug"}})

	if len(got) != 1 || got[0].Target != RouteLaneTarget {
		t.Fatalf("Reroutes = %+v, want one lane reroute", got)
	}
}
