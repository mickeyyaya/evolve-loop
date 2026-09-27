package triagecap

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func TestReadInboxBacklog_AdmitsOnlyItemsALaneMayTakeNow(t *testing.T) {
	evolveDir := t.TempDir()
	writeLifecycleTodo(t, evolveDir, inboxmover.StatePending, "blocker")
	writeLifecycleTodo(t, evolveDir, inboxmover.StateProcessed, "done")
	writeLifecycleTodo(t, evolveDir, inboxmover.StatePending, "waits-on-blocker", "blocker")
	writeLifecycleTodo(t, evolveDir, inboxmover.StatePending, "after-done", "done")
	writeLifecycleTodo(t, evolveDir, inboxmover.StatePending, "free")
	got := map[string]bool{}
	for _, c := range ReadInboxBacklog(evolveDir, nil) {
		got[c.ID] = true
	}
	want := map[string]bool{"blocker": true, "after-done": true, "free": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("backlog = %v, want %v: an item waiting on a pending dependency is not lane material", got, want)
	}
}
