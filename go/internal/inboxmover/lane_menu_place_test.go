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
