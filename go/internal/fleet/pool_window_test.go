package fleet

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type heldLanes struct {
	started chan string
	mu      sync.Mutex
	release map[string]chan struct{}
}

func newHeldLanes() *heldLanes {
	return &heldLanes{started: make(chan string, 16), release: map[string]chan struct{}{}}
}

func (h *heldLanes) gate(id string) chan struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.release[id]; !ok {
		h.release[id] = make(chan struct{})
	}
	return h.release[id]
}

func (h *heldLanes) launch(ctx context.Context, spec CycleSpec) (int, error) {
	id := scopeID(spec)
	gate := h.gate(id)
	h.started <- id
	select {
	case <-gate:
		return 0, nil
	case <-ctx.Done():
		return 143, nil
	}
}

func (h *heldLanes) finish(id string) { close(h.gate(id)) }

func (h *heldLanes) next(t *testing.T) string {
	t.Helper()
	select {
	case id := <-h.started:
		return id
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a lane launch")
		return ""
	}
}

type transitionLog struct {
	mu   sync.Mutex
	list []PoolTransition
}

func (l *transitionLog) record(pt PoolTransition) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list = append(l.list, pt)
}

func (l *transitionLog) lives(kind PoolTransitionKind) []int {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []int
	for _, pt := range l.list {
		if pt.Kind == kind {
			out = append(out, pt.Live)
		}
	}
	return out
}

func (l *transitionLog) peakLive() int {
	peak := 0
	for _, kind := range []PoolTransitionKind{PoolLaunched, PoolBackfilled} {
		for _, live := range l.lives(kind) {
			peak = max(peak, live)
		}
	}
	return peak
}

func runWindowed(cfg PoolConfig, h *heldLanes, log *transitionLog, ids ...string) chan []Result {
	backlog := make([]Todo, len(ids))
	for i, id := range ids {
		backlog[i] = Todo{ID: id}
	}
	done := make(chan []Result, 1)
	go func() { done <- RunPool(context.Background(), cfg, backlog, h.launch, log.record) }()
	return done
}

func TestRunPool_TheWindowHookBoundsAdmissionAndAShrinkNeverStopsALaneInFlight(t *testing.T) {
	var window atomic.Int32
	window.Store(4)
	h, log := newHeldLanes(), &transitionLog{}
	done := runWindowed(PoolConfig{Target: 4, Window: func() int { return int(window.Load()) }}, h, log, "a", "b", "c", "d", "e", "f")
	for range 4 {
		h.next(t)
	}

	window.Store(1)
	for _, id := range []string{"a", "b", "c", "d"} {
		h.finish(id)
	}
	if got := h.next(t); got != "e" {
		t.Fatalf("next launch = %q, want e after the last of the 4 lanes in flight exited", got)
	}
	h.finish("e")
	if got := h.next(t); got != "f" {
		t.Fatalf("next launch = %q, want f", got)
	}
	h.finish("f")
	for i, r := range <-done {
		if r.ExitCode != 0 || r.Err != nil {
			t.Errorf("result[%d] = %+v, want ok: a smaller window never stops a lane", i, r)
		}
	}
	if got := log.lives(PoolBackfilled); !reflect.DeepEqual(got, []int{1, 1}) {
		t.Errorf("live lanes at each backfill = %v, want [1 1]: the exits at 3, 2 and 1 lanes in flight were outside the window", got)
	}
}

func TestRunPool_AWindowOfZeroOrLessStillAdmitsOneLaneAtATime(t *testing.T) {
	for _, width := range []int{0, -3} {
		h, log := newHeldLanes(), &transitionLog{}
		done := runWindowed(PoolConfig{Target: 3, Window: func() int { return width }}, h, log, "a", "b")
		if got := h.next(t); got != "a" {
			t.Fatalf("window %d: first launch = %q, want a: a window of nothing would hang the run", width, got)
		}
		h.finish("a")
		if got := h.next(t); got != "b" {
			t.Fatalf("window %d: second launch = %q, want b", width, got)
		}
		h.finish("b")
		<-done
		if got := log.lives(PoolLaunched); !reflect.DeepEqual(got, []int{1}) {
			t.Errorf("window %d: live lanes at the first fill = %v, want [1]", width, got)
		}
	}
}

func TestRunPool_AWindowAboveTheTargetNeverRaisesIt(t *testing.T) {
	h, log := newHeldLanes(), &transitionLog{}
	done := runWindowed(PoolConfig{Target: 2, Concurrency: 5, Window: func() int { return 9 }}, h, log, "a", "b", "c")
	h.next(t)
	h.next(t)
	h.finish("a")
	h.finish("b")
	h.next(t)
	h.finish("c")
	<-done
	if peak := log.peakLive(); peak != 2 {
		t.Errorf("peak lanes live = %d, want the target 2: a window of 9 does not raise it", peak)
	}
}

func TestRunPool_AWindowNeverLiftsTheConcurrencyCap(t *testing.T) {
	h, log := newHeldLanes(), &transitionLog{}
	done := runWindowed(PoolConfig{Target: 4, Concurrency: 2, Window: func() int { return 4 }}, h, log, "a", "b", "c", "d")
	h.next(t)
	h.next(t)
	h.finish("a")
	h.next(t)
	h.finish("b")
	h.next(t)
	h.finish("c")
	h.finish("d")
	<-done
	if peak := log.peakLive(); peak != 2 {
		t.Errorf("peak lanes live = %d, want the concurrency cap 2: a window of 4 does not lift it", peak)
	}
}

func TestRunPool_ASlotTheWindowWithholdsIsNotReportedIdleButAnEmptySupplyIs(t *testing.T) {
	var window atomic.Int32
	window.Store(3)
	h, log := newHeldLanes(), &transitionLog{}
	done := runWindowed(PoolConfig{Target: 3, Window: func() int { return int(window.Load()) }}, h, log, "a", "b", "c", "d")
	for range 3 {
		h.next(t)
	}

	window.Store(1)
	h.finish("a")
	h.finish("b")
	h.finish("c")
	if got := h.next(t); got != "d" {
		t.Fatalf("next launch = %q, want d once the window had room", got)
	}
	h.finish("d")
	<-done
	if got := log.lives(PoolIdle); !reflect.DeepEqual(got, []int{0}) {
		t.Errorf("idle transitions (live lanes at each) = %v, want [0]: the exits of a, b and c left slots that the window held on purpose, and only the last exit found the supply empty", got)
	}
}
