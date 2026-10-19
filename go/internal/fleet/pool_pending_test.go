package fleet

import (
	"context"
	"reflect"
	"strconv"
	"sync"
	"testing"
)

func TestRunPool_EveryTransitionCarriesTheTodosStillPending(t *testing.T) {
	sharedFile := []string{"go/internal/x/f.go"}
	backlog := []Todo{{ID: "A", Files: sharedFile}, {ID: "B", Files: sharedFile}, {ID: "C", Files: sharedFile}}
	var mu sync.Mutex
	var seen []PoolTransition
	var log launchLog

	RunPool(context.Background(), PoolConfig{Target: 4}, backlog, log.launch(0), func(tr PoolTransition) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, tr)
	})

	var got []string
	for _, tr := range seen {
		got = append(got, string(tr.Kind)+":"+tr.Todo+":"+strconv.Itoa(tr.Pending))
	}
	want := []string{
		"launched:A:2", "exited:A:2", "backfilled:B:1", "idle::1",
		"exited:B:1", "backfilled:C:0", "idle::0",
		"exited:C:0", "idle::0",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("transitions = %v, want %v: Pending counts the todos not yet launched after the transition", got, want)
	}
}
