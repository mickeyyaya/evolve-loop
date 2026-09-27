package inboxmover

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestPartitionLaneMenu_HoldsBackItemsWaitingOnADependency(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeTask(t, inbox, "dep-pending", nil)
	writeTask(t, filepath.Join(inbox, "processed", "cycle-9"), "dep-done", nil)
	items := []inboxbatch.Item{
		{ID: "free"},
		{ID: "waits", Deps: []string{"dep-pending"}},
		{ID: "after-done", Deps: []string{"dep-done"}},
	}
	menu := PartitionLaneMenu(Options{ProjectRoot: root}, items, nil)
	if got := idsOf(menu.Ready); !reflect.DeepEqual(got, []string{"free", "after-done"}) {
		t.Errorf("ready = %v", got)
	}
	if got := idsOf(menu.Waiting); !reflect.DeepEqual(got, []string{"waits"}) {
		t.Errorf("waiting = %v", got)
	}
	if !reflect.DeepEqual(menu.WaitingReasons, []string{"waits: deps unmet: needs dep-pending"}) {
		t.Errorf("reasons = %v", menu.WaitingReasons)
	}
}

func idsOf(items []inboxbatch.Item) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func TestPartitionLaneMenu_ADependentWaitsEvenWhenItsDependencyIsOnTheSameMenu(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeTask(t, inbox, "first", nil)
	writeTask(t, inbox, "second", []string{"first"})
	items := []inboxbatch.Item{{ID: "first"}, {ID: "second", Deps: []string{"first"}}}
	menu := PartitionLaneMenu(Options{ProjectRoot: root}, items, nil)
	if got := idsOf(menu.Ready); !reflect.DeepEqual(got, []string{"first"}) {
		t.Errorf("ready = %v: only the dependency is offered", got)
	}
	if got := idsOf(menu.Waiting); !reflect.DeepEqual(got, []string{"second"}) {
		t.Errorf("waiting = %v: the dependent waits until its dependency lands, never batched with it", got)
	}
}

func TestPartitionLaneMenu_SplitsReadyConsoleOwnedAndWaiting(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeTask(t, inbox, "operator-work", nil)
	items := []inboxbatch.Item{
		{ID: "lane-work"},
		{ID: "operator-work", Route: "console-manual"},
		{ID: "guarded", Files: []string{"go/internal/guarded/x.go"}},
		{ID: "blocked", Deps: []string{"operator-work"}},
	}
	guarded := func(path string) bool { return path == "go/internal/guarded/x.go" }
	var menu LaneMenu = PartitionLaneMenu(Options{ProjectRoot: root}, items, guarded)
	if got := idsOf(menu.Ready); !reflect.DeepEqual(got, []string{"lane-work"}) {
		t.Errorf("ready = %v", got)
	}
	if got := idsOf(menu.Console); !reflect.DeepEqual(got, []string{"operator-work", "guarded"}) || len(menu.ConsoleReasons) != 2 {
		t.Errorf("console = %v %v", got, menu.ConsoleReasons)
	}
	if got := idsOf(menu.Waiting); !reflect.DeepEqual(got, []string{"blocked"}) || !reflect.DeepEqual(menu.WaitingReasons, []string{"blocked: deps unmet: needs operator-work"}) {
		t.Errorf("waiting = %v %v", got, menu.WaitingReasons)
	}
}
