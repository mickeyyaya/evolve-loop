package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func newStore(t *testing.T) (*FilesystemStorage, string) {
	t.Helper()
	dir := t.TempDir()
	evolveDir := filepath.Join(dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return New(evolveDir), evolveDir
}

// ReadState on a fresh repo (no state.json) returns the zero value and no error.
// Matches the bash behaviour of scripts/lifecycle/cycle-state.sh: missing
// state.json bootstraps an empty State.
func TestReadState_MissingFileReturnsZero(t *testing.T) {
	s, _ := newStore(t)
	got, err := s.ReadState(context.Background())
	if err != nil {
		t.Fatalf("ReadState missing: %v", err)
	}
	if got.LastCycleNumber != 0 || got.Version != 0 {
		t.Errorf("got %+v, want zero", got)
	}
}

func TestStateJSON_RoundTrip(t *testing.T) {
	s, _ := newStore(t)
	in := core.State{
		LastUpdated:     "2026-05-22T07:30:00Z",
		LastCycleNumber: 104,
		Version:         18,
		CurrentBatch:    core.BatchAccrual{CycleAccruedCostUSD: 3.21, GoalHash: "abc"},
		FailedAt: []core.FailedRecord{
			{Cycle: 100, Verdict: "FAIL", SHA256: "deadbeef"},
		},
		CarryoverTodos: []core.CarryoverTodo{
			{ID: "todo-1", Action: "investigate", Priority: "P1"},
		},
	}
	if err := s.WriteState(context.Background(), in); err != nil {
		t.Fatalf("WriteState: %v", err)
	}
	out, err := s.ReadState(context.Background())
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if out.LastCycleNumber != 104 || out.Version != 18 {
		t.Errorf("scalar lost: %+v", out)
	}
	if out.CurrentBatch.CycleAccruedCostUSD != 3.21 {
		t.Errorf("currentBatch lost: %+v", out.CurrentBatch)
	}
	if len(out.FailedAt) != 1 || out.FailedAt[0].SHA256 != "deadbeef" {
		t.Errorf("failedAt lost: %+v", out.FailedAt)
	}
	if len(out.CarryoverTodos) != 1 || out.CarryoverTodos[0].Priority != "P1" {
		t.Errorf("carryoverTodos lost: %+v", out.CarryoverTodos)
	}
}

func TestWriteState_AtomicNoPartialFile(t *testing.T) {
	s, dir := newStore(t)
	path := filepath.Join(dir, "state.json")
	// Pre-fill with a sentinel; the atomic write must replace it entirely
	// rather than ever leaving the file truncated to zero bytes mid-rename.
	if err := os.WriteFile(path, []byte(`{"lastCycleNumber": 999}`), 0o644); err != nil {
		t.Fatal(err)
	}
	in := core.State{LastCycleNumber: 5}
	if err := s.WriteState(context.Background(), in); err != nil {
		t.Fatalf("write: %v", err)
	}
	// After the write, the file must contain the new state, not the old.
	got, err := s.ReadState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.LastCycleNumber != 5 {
		t.Errorf("lastCycleNumber=%d, want 5", got.LastCycleNumber)
	}
	// No leftover .tmp files in the dir.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover tmp file: %s", e.Name())
		}
	}
}

func TestCycleStateJSON_RoundTrip(t *testing.T) {
	s, _ := newStore(t)
	in := core.CycleState{
		CycleID:         42,
		Phase:           "build",
		StartedAt:       "2026-05-22T08:00:00Z",
		PhaseStartedAt:  "2026-05-22T08:30:00Z",
		ActiveAgent:     "builder",
		ActiveWorktree:  "/tmp/worktree",
		CompletedPhases: []string{"scout", "triage", "tdd"},
		WorkspacePath:   "/tmp/x/.evolve/runs/cycle-42",
		IntentRequired:  true,
	}
	if err := s.WriteCycleState(context.Background(), in); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := s.ReadCycleState(context.Background())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if out.CycleID != 42 || out.ActiveAgent != "builder" {
		t.Errorf("scalar lost: %+v", out)
	}
	if len(out.CompletedPhases) != 3 || out.CompletedPhases[2] != "tdd" {
		t.Errorf("completed lost: %+v", out.CompletedPhases)
	}
	if !out.IntentRequired {
		t.Error("intent_required lost")
	}
}

func TestReadCycleState_MissingFile(t *testing.T) {
	s, _ := newStore(t)
	got, err := s.ReadCycleState(context.Background())
	if err != nil {
		t.Fatalf("ReadCycleState missing: %v", err)
	}
	if got.CycleID != 0 {
		t.Errorf("want zero, got %+v", got)
	}
}

// AcquireLock — exclusive, returns release fn.
func TestAcquireLock_ExclusiveWithinProcess(t *testing.T) {
	s, _ := newStore(t)
	rel, err := s.AcquireLock(context.Background())
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer rel()

	// Within the same process a second AcquireLock must fail fast.
	_, err = s.AcquireLock(context.Background())
	if !errors.Is(err, core.ErrLockHeld) {
		t.Errorf("second acquire: err=%v, want ErrLockHeld", err)
	}
}

func TestAcquireLock_ReleasedAfterCallback(t *testing.T) {
	s, _ := newStore(t)
	rel, err := s.AcquireLock(context.Background())
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := rel(); err != nil {
		t.Fatalf("release: %v", err)
	}
	// After release, a fresh acquire must succeed.
	rel2, err := s.AcquireLock(context.Background())
	if err != nil {
		t.Fatalf("re-acquire: %v", err)
	}
	rel2()
}

// Concurrent AcquireLock callers — exactly one must hold at a time.
func TestAcquireLock_ConcurrentExclusive(t *testing.T) {
	s, _ := newStore(t)

	var (
		held       int32
		maxHeld    int32
		successCnt int32
		wg         sync.WaitGroup
	)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rel, err := s.AcquireLock(context.Background())
			if err != nil {
				return // ErrLockHeld races are expected
			}
			cur := atomic.AddInt32(&held, 1)
			for {
				old := atomic.LoadInt32(&maxHeld)
				if cur <= old || atomic.CompareAndSwapInt32(&maxHeld, old, cur) {
					break
				}
			}
			atomic.AddInt32(&successCnt, 1)
			time.Sleep(5 * time.Millisecond)
			atomic.AddInt32(&held, -1)
			_ = rel()
		}()
	}
	wg.Wait()
	if maxHeld > 1 {
		t.Errorf("maxHeld=%d, want 1", maxHeld)
	}
	if successCnt == 0 {
		t.Error("no goroutine ever acquired the lock")
	}
}
