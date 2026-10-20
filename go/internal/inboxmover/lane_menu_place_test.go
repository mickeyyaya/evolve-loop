package inboxmover

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestPlaceOnLaneMenu_OneJudgmentPerItem(t *testing.T) {
	root := t.TempDir()
	writeTask(t, filepath.Join(root, ".evolve", "inbox"), "operator-work", nil)
	guarded := func(path string) bool { return path == "go/internal/guarded/x.go" }
	cases := []struct {
		item   inboxbatch.Item
		place  MenuPlace
		reason string
	}{
		{inboxbatch.Item{ID: "lane-work"}, MenuReady, ""},
		{inboxbatch.Item{ID: "operator-work", Route: "console-manual"}, MenuConsole, "route:console-manual"},
		{inboxbatch.Item{ID: "guarded", Files: []string{"go/internal/guarded/x.go"}}, MenuConsole, "protected fix surface: go/internal/guarded/x.go"},
		{inboxbatch.Item{ID: "blocked", Deps: []string{"operator-work"}}, MenuWaiting, "deps unmet: needs operator-work"},
		{inboxbatch.Item{ID: "operator-blocked", Route: "console-manual", Deps: []string{"operator-work"}}, MenuConsole, "route:console-manual"},
	}
	for _, c := range cases {
		if place, reason := PlaceOnLaneMenu(Options{ProjectRoot: root}, c.item, guarded); place != c.place || reason != c.reason {
			t.Errorf("%s: (%v, %q), want (%v, %q)", c.item.ID, place, reason, c.place, c.reason)
		}
	}
}

func TestPlaceOnLaneMenu_AnItemStaleDroppedTwiceWaitsForAVerifiedPremise(t *testing.T) {
	t.Parallel()
	opts := Options{InboxDir: filepath.Join(t.TempDir(), ".evolve", "inbox")}
	twice := inboxbatch.Item{ID: "twice", Route: "lane", StaleDrops: inboxbatch.StaleDropHold, LastStaleDropAt: "2026-10-01T00:00:00Z"}

	if place, reason := PlaceOnLaneMenu(opts, twice, nil); place != MenuConsole || reason != "stale-dropped 2 times since its premise was last verified; run `evolve inbox verify twice --evidence <text>` first" {
		t.Fatalf("PlaceOnLaneMenu = %v, %q; a third lane must not run before the console verifies the premise", place, reason)
	}
	twice.PremiseVerified = "2026-10-02T00:00:00Z"
	if place, _ := PlaceOnLaneMenu(opts, twice, nil); place != MenuReady {
		t.Fatalf("PlaceOnLaneMenu = %v; a premise verified after the last drop puts the item back on the menu", place)
	}
	if place, _ := PlaceOnLaneMenu(opts, inboxbatch.Item{ID: "once", StaleDrops: 1, LastStaleDropAt: "2026-10-01T00:00:00Z"}, nil); place != MenuReady {
		t.Fatalf("PlaceOnLaneMenu = %v; one stale drop holds nothing", place)
	}
}
