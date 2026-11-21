package fleet

import (
	"context"
	"sync"
	"testing"
)

type signalingSupply struct {
	once   sync.Once
	called chan struct{}
	batch  []Todo
}

func newSignalingSupply(batch ...Todo) *signalingSupply {
	return &signalingSupply{called: make(chan struct{}), batch: batch}
}

func (s *signalingSupply) next([]Todo) []Todo {
	var out []Todo
	s.once.Do(func() {
		out = s.batch
		close(s.called)
	})
	return out
}

func drainStarted(h *heldLaunch) []string {
	var out []string
	for {
		select {
		case id := <-h.started:
			out = append(out, id)
		default:
			return out
		}
	}
}

func TestPoolResupply_RelaunchesAParkedLeadAfterItsLaneExited(t *testing.T) {
	supply := &resupplier{batches: [][]Todo{{{ID: "A", Resume: true}, {ID: "B"}}}}
	var log launchLog

	RunPool(context.Background(), PoolConfig{Target: 2, Resupply: supply.next}, []Todo{{ID: "A"}}, log.launch(0), nil)

	if want := []string{"A", "A", "B"}; !sameIDs(log.got(), want) {
		t.Errorf("launches = %v, want %v: the resumed lead A runs a second time beside B", log.got(), want)
	}
}

func TestPoolResupply_KeepsAnUnmarkedKnownTodoDeduped(t *testing.T) {
	supply := &resupplier{batches: [][]Todo{{{ID: "A"}, {ID: "B"}}}}
	var log launchLog

	RunPool(context.Background(), PoolConfig{Target: 2, Resupply: supply.next}, []Todo{{ID: "A"}}, log.launch(0), nil)

	if want := []string{"A", "B"}; !sameIDs(log.got(), want) {
		t.Errorf("launches = %v, want %v: an unmarked A that already launched is not launched again", log.got(), want)
	}
}

func TestPoolResupply_NeverRelaunchesAMarkedTodoWhileItsLaneRuns(t *testing.T) {
	supply := newSignalingSupply(Todo{ID: "A", Resume: true})
	h := newHeldLaunch("X")
	done := make(chan []Result, 1)
	go func() {
		done <- RunPool(context.Background(), PoolConfig{Target: 2, Resupply: supply.next}, []Todo{{ID: "A"}, {ID: "X"}}, h.launch, nil)
	}()
	if got := h.await(t, 2); !sameIDs(got, []string{"A", "X"}) {
		t.Fatalf("first launches = %v, want A and X", got)
	}
	<-supply.called
	close(h.hold)
	res := <-done

	if extra := drainStarted(h); len(extra) != 0 {
		t.Errorf("later launches = %v, want none: the lane of A still ran when the resupply named it", extra)
	}
	if len(res) != 2 {
		t.Errorf("results = %d, want 2", len(res))
	}
}

func TestPoolResupply_NeverRelaunchesAMarkedTodoWhileItsLaneIsPending(t *testing.T) {
	supply := newSignalingSupply(Todo{ID: "A", Resume: true})
	h := newHeldLaunch("C")
	backlog := []Todo{{ID: "B", Files: []string{"x.go"}}, {ID: "A", Files: []string{"x.go"}}, {ID: "C", Files: []string{"y.go"}}}
	done := make(chan []Result, 1)
	go func() {
		done <- RunPool(context.Background(), PoolConfig{Target: 3, Resupply: supply.next}, backlog, h.launch, nil)
	}()
	if got := h.await(t, 2); !sameIDs(got, []string{"B", "C"}) {
		t.Fatalf("first launches = %v, want B and C: A waits on the file of B", got)
	}
	<-supply.called
	close(h.hold)
	res := <-done

	if later := drainStarted(h); !sameIDs(later, []string{"A"}) {
		t.Errorf("later launches = %v, want A exactly once: it was pending when the resupply named it", later)
	}
	if len(res) != 3 {
		t.Errorf("results = %d, want 3", len(res))
	}
}
