package inboxcurator

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestReroutes_AGateFileItemOnTheLaneRouteIsADeterministicRerouteToTheConsole(t *testing.T) {
	gate := func(p string) bool { return p == "go/internal/guards/ship.go" }
	onLane := inboxbatch.Item{ID: "gate", Kind: "feature", Route: inboxbatch.RouteLaneValue, Files: []string{"go/internal/guards/ship.go"}}
	underived := inboxbatch.Item{ID: "plain", Kind: "feature", Route: "console-manual", Files: []string{"go/internal/fleet/pool.go"}}
	derived := inboxbatch.Item{ID: "pipe", Kind: "pipeline-repair", Route: "console", Files: []string{"go/internal/fleet/pool.go"}}
	fine := inboxbatch.Item{ID: "lane", Kind: "feature", Route: inboxbatch.RouteLaneValue, Files: []string{"go/internal/fleet/pool.go"}}

	got := Reroutes([]inboxbatch.Item{onLane, underived, derived, fine}, gate)

	if len(got) != 2 {
		t.Fatalf("Reroutes = %+v, want two proposals", got)
	}
	if got[0].ID != "gate" || got[0].Target != RouteConsoleTarget || !got[0].Deterministic || got[0].Kind != ProposeReroute || got[0].Trigger != TriggerRouting {
		t.Errorf("first = %+v, want a deterministic reroute of gate to the console", got[0])
	}
	if got[1].ID != "plain" || got[1].Target != RouteLaneTarget || got[1].Deterministic {
		t.Errorf("second = %+v, want a reroute of plain to a lane for the operator", got[1])
	}
}

func TestTouchesRouting_IsTrueForThePolicyTheManifestAndTheProfiles(t *testing.T) {
	for _, paths := range [][]string{{".evolve/policy.json"}, {"go/internal/guards/integrity_surface.go"}, {"docs/a.md", ".evolve/profiles/builder.json"}} {
		if !TouchesRouting(paths) {
			t.Errorf("TouchesRouting(%v) = false, want true", paths)
		}
	}
	if TouchesRouting([]string{"go/internal/fleet/pool.go", ".evolve/policy.json.bak"}) {
		t.Error("TouchesRouting of other paths = true, want false")
	}
}

func TestDuplicates_ProposesAMergeOfTheNewerItemIntoTheOlderForTheOperator(t *testing.T) {
	files := []string{"go/a.go"}
	older := item("older", "alpha beta gamma", files)
	older.CreatedAt = "2026-10-01T00:00:00Z"
	newer := item("newer", "alpha beta gamma", files)
	newer.CreatedAt = "2026-10-02T00:00:00Z"
	other := item("other", "delta", files)

	got := Duplicates([]inboxbatch.Item{newer, older, other}, Thresholds{Refuse: 0.5})

	if len(got) != 1 || got[0].ID != "newer" || got[0].Target != "older" || got[0].Kind != ProposeMerge || got[0].Deterministic || got[0].Trigger != TriggerSweep {
		t.Fatalf("Duplicates = %+v, want one merge of newer into older", got)
	}
	if !slices.Contains(got[0].Evidence, "similarity 1.00 on go/a.go") {
		t.Errorf("evidence = %v, want the similarity and the surface", got[0].Evidence)
	}
}

func TestReroutes_AConsoleRouteWithARecordedReasonIsExplained(t *testing.T) {
	declined := inboxbatch.Item{ID: "kept", Kind: "feature", Route: "console-manual", RoutedReason: json.RawMessage(`"the operator keeps it"`), Files: []string{"go/internal/fleet/pool.go"}}

	if got := Reroutes([]inboxbatch.Item{declined}, func(string) bool { return false }); len(got) != 0 {
		t.Errorf("Reroutes = %+v, want none: route-console recorded the reason", got)
	}
}
