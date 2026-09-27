package triagecap

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func TestReadInboxBacklog_OffersExactlyTheLaneMenusReadyItemsThatCarryAnID(t *testing.T) {
	evolveDir := t.TempDir()
	writeLifecycleTodo(t, evolveDir, inboxmover.StatePending, "free")
	writeLifecycleTodo(t, evolveDir, inboxmover.StatePending, "operator-work")
	writeLifecycleTodo(t, evolveDir, inboxmover.StatePending, "waits", "operator-work")
	writeLifecycleTodo(t, evolveDir, inboxmover.StateProcessed, "done")
	writeLifecycleTodo(t, evolveDir, inboxmover.StatePending, "after-done", "done")
	fromMenu, fromBacklog := menuReadyIDs(t, evolveDir), backlogIDs(evolveDir)
	if !reflect.DeepEqual(fromMenu, fromBacklog) {
		t.Errorf("the planner's backlog %v must be the lane menu's ready set %v", fromBacklog, fromMenu)
	}
}

func TestReadInboxBacklog_DropsAnIDLessItemTheLaneMenuStillOffers(t *testing.T) {
	evolveDir := t.TempDir()
	inbox := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "no-id.json"), []byte(`{"weight":0.5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := menuReadyIDs(t, evolveDir); !reflect.DeepEqual(got, []string{"no-id"}) {
		t.Fatalf("the lane menu names an id-less item by its file, got %v", got)
	}
	if got := backlogIDs(evolveDir); len(got) != 0 {
		t.Errorf("the planner's backlog drops an id-less item, got %v", got)
	}
}

func menuReadyIDs(t *testing.T, evolveDir string) []string {
	t.Helper()
	inbox := filepath.Join(evolveDir, "inbox")
	items, _, err := inboxbatch.LoadDir(inbox)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, it := range inboxmover.PartitionLaneMenu(inboxmover.Options{InboxDir: inbox}, items, noProtectedSurface).Ready {
		ids = append(ids, it.ID)
	}
	sort.Strings(ids)
	return ids
}

func backlogIDs(evolveDir string) []string {
	var ids []string
	for _, c := range ReadInboxBacklog(evolveDir, noProtectedSurface) {
		ids = append(ids, c.ID)
	}
	sort.Strings(ids)
	return ids
}

func noProtectedSurface(string) bool { return false }
