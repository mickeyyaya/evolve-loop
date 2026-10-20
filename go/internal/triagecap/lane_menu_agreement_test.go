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

func TestInboxReaders_GiveEachItemOneID(t *testing.T) {
	evolveDir := t.TempDir()
	inbox := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	fixtures := []struct{ id, file, body string }{
		{"no-id", "no-id.json", `{"weight":0.5}`},
		{"bell item", "bell.json", `{"id":"bell\u0007item","weight":0.5}`},
	}
	for _, f := range fixtures {
		if err := os.WriteFile(filepath.Join(inbox, f.file), []byte(f.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"bell item", "no-id"}
	if got := menuReadyIDs(t, evolveDir); !reflect.DeepEqual(got, want) {
		t.Errorf("the lane menu names the items %q, want %q", got, want)
	}
	if got := backlogIDs(evolveDir); !reflect.DeepEqual(got, want) {
		t.Errorf("the planner's backlog names the items %q, want %q", got, want)
	}
	for _, f := range fixtures {
		if loc, err := inboxmover.Locate(inbox, f.id); err != nil || loc.Path != filepath.Join(inbox, f.file) {
			t.Errorf("the lifecycle finds %q by the loader's id: %+v %v", f.id, loc, err)
		}
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
