package fleet

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRunPool_AHoldAtTheStartAndAfterAKickEmitsOneHeldTransitionPerState(t *testing.T) {
	t.Parallel()
	kicks := make(chan struct{})
	release := make(chan struct{})
	started := make(chan string, 4)
	var armed atomic.Bool
	cfg := PoolConfig{Target: 4, Kick: kicks, Next: func(inFlight, _ []Todo) int {
		if len(inFlight) == 0 || armed.CompareAndSwap(true, false) {
			return 0
		}
		return -1
	}}
	launch := func(_ context.Context, spec CycleSpec) (int, error) {
		started <- scopeID(spec)
		<-release
		return 0, nil
	}
	var mu sync.Mutex
	var held [][2]int
	heldSeen := make(chan struct{}, 8)
	done := make(chan struct{})
	go func() {
		RunPool(context.Background(), cfg, []Todo{{ID: "A"}, {ID: "B"}, {ID: "C"}, {ID: "D"}}, launch, func(tr PoolTransition) {
			if tr.Kind == PoolHeld {
				mu.Lock()
				held = append(held, [2]int{tr.Live, tr.Pending})
				mu.Unlock()
				heldSeen <- struct{}{}
			}
		})
		close(done)
	}()
	<-started
	<-heldSeen
	armed.Store(true)
	kicks <- struct{}{}
	<-started
	<-heldSeen
	kicks <- struct{}{}
	kicks <- struct{}{}
	close(release)
	<-done
	mu.Lock()
	defer mu.Unlock()
	if want := [][2]int{{1, 3}, {2, 2}}; !reflect.DeepEqual(held, want) {
		t.Fatalf("held (live, pending) = %v, want %v: one transition at the start hold and one after the kick that changed the state, none for the kicks that changed nothing", held, want)
	}
}

func TestRunPool_AFullPoolEmitsNoHeldTransition(t *testing.T) {
	t.Parallel()
	cfg := PoolConfig{Target: 2, Next: func(_, _ []Todo) int { return 0 }}
	var held atomic.Int64
	var log launchLog
	RunPool(context.Background(), cfg, []Todo{{ID: "A"}, {ID: "B"}}, log.launch(0), func(tr PoolTransition) {
		if tr.Kind == PoolHeld {
			held.Add(1)
		}
	})
	if n := held.Load(); n != 0 {
		t.Fatalf("held transitions = %d, want 0: admission refused nothing", n)
	}
}
