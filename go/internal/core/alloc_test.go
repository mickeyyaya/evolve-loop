package core

import (
	"context"
	"sync"
	"testing"
)

// alloc_test.go — CA.4 (concurrency-factory plan, Track C-A): cycle-number
// allocation lease. `lastAllocatedCycleNumber` (≠ lastCompleted) is bumped
// through the serialized UpdateState RMW, so two concurrent allocators can
// never mint the same cycle number; a crashed run burns its number (gap),
// and resume reuses the run record (RunCycleFromPhase) — it never
// re-allocates.

// memUpdater is an in-memory StateUpdater for allocation-semantics tests
// (the cross-process acceptance runs against the real FilesystemStorage in
// adapters/storage).
type memUpdater struct {
	mu sync.Mutex
	st State
}

func (m *memUpdater) UpdateState(_ context.Context, mutate func(*State)) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mutate(&m.st)
	m.st.StateRevision++
	return m.st, nil
}

func TestAllocateCycleNumber_SingleModeEquivalence(t *testing.T) {
	// Fresh lease (never allocated): identical to the legacy
	// LastCycleNumber+1 — the single-mode byte/behavior-stability bar.
	m := &memUpdater{st: State{LastCycleNumber: 5}}
	n, err := AllocateCycleNumber(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	if n != 6 {
		t.Errorf("allocated %d, want 6 (legacy equivalence)", n)
	}
	if m.st.LastAllocatedCycleNumber != 6 {
		t.Errorf("lease not persisted: %+v", m.st)
	}
}

func TestAllocateCycleNumber_CrashBurnsNumber(t *testing.T) {
	// A prior run allocated 6 and crashed (LastCycleNumber still 5).
	// The next allocation must burn 6 and mint 7 — never reuse a number a
	// crashed run may have left artifacts under.
	m := &memUpdater{st: State{LastCycleNumber: 5, LastAllocatedCycleNumber: 6}}
	n, err := AllocateCycleNumber(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	if n != 7 {
		t.Errorf("allocated %d, want 7 (crashed allocation must burn)", n)
	}
}

func TestAllocateCycleNumber_ConcurrentAllocatorsDistinct(t *testing.T) {
	m := &memUpdater{st: State{LastCycleNumber: 10}}
	const G = 16
	got := make(chan int, G)
	var wg sync.WaitGroup
	for i := 0; i < G; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := AllocateCycleNumber(context.Background(), m)
			if err != nil {
				t.Error(err)
				return
			}
			got <- n
		}()
	}
	wg.Wait()
	close(got)
	seen := map[int]bool{}
	for n := range got {
		if seen[n] {
			t.Fatalf("duplicate cycle number allocated: %d", n)
		}
		seen[n] = true
	}
	for n := 11; n <= 10+G; n++ {
		if !seen[n] {
			t.Errorf("cycle number %d never allocated (gap without a crash)", n)
		}
	}
}

// TestOrchestratorAllocateCycle_LegacyStorageFallsBack — a storage without
// UpdateState (every existing fake / pre-CA.3 adapter) keeps the exact
// legacy LastCycleNumber+1 path: byte-identical single-mode behavior.
func TestOrchestratorAllocateCycle_LegacyStorageFallsBack(t *testing.T) {
	o := &Orchestrator{storage: &fakeStorage{}}
	st := State{LastCycleNumber: 41}
	n, err := o.allocateCycle(context.Background(), &st)
	if err != nil {
		t.Fatal(err)
	}
	if n != 42 {
		t.Errorf("legacy fallback allocated %d, want 42", n)
	}
}

// fakeUpdaterStorage upgrades fakeStorage with the StateUpdater capability.
type fakeUpdaterStorage struct {
	fakeStorage
	mem memUpdater
}

func (f *fakeUpdaterStorage) UpdateState(ctx context.Context, mutate func(*State)) (State, error) {
	return f.mem.UpdateState(ctx, mutate)
}

// TestPersistCycleEndState_NeverRollsBackLease — the reviewer-named clobber:
// run A allocates 8, run B allocates 9, run A's cycle-end persist (carrying
// its stale in-memory lease=8) must NOT roll the on-disk lease back to 8 —
// otherwise the next allocator re-mints 9, B's number.
func TestPersistCycleEndState_NeverRollsBackLease(t *testing.T) {
	f := &fakeUpdaterStorage{}
	f.mem.st = State{LastCycleNumber: 7}
	o := &Orchestrator{storage: f}

	stA := State{LastCycleNumber: 7}
	nA, err := o.allocateCycle(context.Background(), &stA) // A leases 8
	if err != nil {
		t.Fatal(err)
	}
	stB := State{LastCycleNumber: 7}
	if _, err := o.allocateCycle(context.Background(), &stB); err != nil { // B leases 9
		t.Fatal(err)
	}

	stA.LastCycleNumber = nA // A completes its cycle and persists
	if err := o.persistCycleEndState(context.Background(), stA); err != nil {
		t.Fatal(err)
	}

	stC := State{LastCycleNumber: nA}
	nC, err := o.allocateCycle(context.Background(), &stC)
	if err != nil {
		t.Fatal(err)
	}
	if nC != 10 {
		t.Errorf("post-persist allocation = %d, want 10 (lease rolled back — B's 9 would be re-minted)", nC)
	}
	// A's own outcome fields must still have landed.
	if f.mem.st.LastCycleNumber != nA {
		t.Errorf("cycle-end persist lost LastCycleNumber: %+v", f.mem.st)
	}
}

// TestPersistCycleEndState_LegacyStorageFallsBack — no StateUpdater ⇒ the
// plain WriteState, byte-identical to the pre-CA.4 cycle end.
func TestPersistCycleEndState_LegacyStorageFallsBack(t *testing.T) {
	fs := &fakeStorage{}
	o := &Orchestrator{storage: fs}
	if err := o.persistCycleEndState(context.Background(), State{LastCycleNumber: 42}); err != nil {
		t.Fatal(err)
	}
	if fs.state.LastCycleNumber != 42 {
		t.Errorf("legacy persist did not WriteState: %+v", fs.state)
	}
}

func TestOrchestratorAllocateCycle_LeaseWhenSupported(t *testing.T) {
	f := &fakeUpdaterStorage{}
	f.mem.st = State{LastCycleNumber: 7, LastAllocatedCycleNumber: 8} // 8 crashed
	o := &Orchestrator{storage: f}
	st := State{LastCycleNumber: 7}
	n, err := o.allocateCycle(context.Background(), &st)
	if err != nil {
		t.Fatal(err)
	}
	if n != 9 {
		t.Errorf("lease path allocated %d, want 9 (burn 8)", n)
	}
	if st.LastAllocatedCycleNumber != 9 {
		t.Errorf("in-memory state not synced with lease: %+v", st)
	}
}
