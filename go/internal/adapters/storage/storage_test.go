package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/test/fixtures"
)

func newStore(t *testing.T) (*FilesystemStorage, string) {
	t.Helper()
	// The temp-project + .evolve/ layout is built by the shared harness so this
	// (previously copy-pasted across ~20 packages) lives in exactly one place.
	ws := fixtures.NewWorkspace(t).Build()
	return New(ws.EvolveDir), ws.EvolveDir
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
			{Cycle: 100, Verdict: "FAIL", AuditReportSHA256: "deadbeef"},
		},
		CarryoverTodos: []core.CarryoverTodo{
			{ID: "todo-1", Action: "investigate", Priority: "P1"},
		},
		TriageThroughput: []core.TriageThroughputEntry{
			{Cycle: 281, Floors: 5},
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
	if len(out.FailedAt) != 1 || out.FailedAt[0].AuditReportSHA256 != "deadbeef" {
		t.Errorf("failedAt lost: %+v", out.FailedAt)
	}
	if len(out.CarryoverTodos) != 1 || out.CarryoverTodos[0].Priority != "P1" {
		t.Errorf("carryoverTodos lost: %+v", out.CarryoverTodos)
	}
	if len(out.TriageThroughput) != 1 || out.TriageThroughput[0].Floors != 5 {
		t.Errorf("triageThroughput lost: %+v", out.TriageThroughput)
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

func TestReadState_InvalidJSON(t *testing.T) {
	s, dir := newStore(t)
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadState(context.Background()); err == nil {
		t.Fatal("invalid JSON must error")
	}
}

func TestReadCycleState_InvalidJSON(t *testing.T) {
	s, dir := newStore(t)
	if err := os.WriteFile(filepath.Join(dir, "cycle-state.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadCycleState(context.Background()); err == nil {
		t.Fatal("invalid JSON must error")
	}
}

func TestReadState_EmptyFile(t *testing.T) {
	s, dir := newStore(t)
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadState(context.Background())
	if err != nil {
		t.Fatalf("empty file: %v", err)
	}
	if got.LastCycleNumber != 0 {
		t.Errorf("zero state expected, got %+v", got)
	}
}

func TestWriteState_NonExistentParent(t *testing.T) {
	// New(...) under a path that doesn't exist yet — WriteState must
	// create the parent dir.
	dir := t.TempDir()
	s := New(filepath.Join(dir, "deep", "nested", ".evolve"))
	if err := s.WriteState(context.Background(), core.State{LastCycleNumber: 1}); err != nil {
		t.Fatalf("write into nested: %v", err)
	}
	got, _ := s.ReadState(context.Background())
	if got.LastCycleNumber != 1 {
		t.Errorf("did not persist: %+v", got)
	}
}

func TestAcquireLock_ReleaseIdempotent(t *testing.T) {
	s, _ := newStore(t)
	rel, err := s.AcquireLock(context.Background())
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := rel(); err != nil {
		t.Errorf("first release: %v", err)
	}
	if err := rel(); err != nil {
		t.Errorf("second release should be no-op, got %v", err)
	}
}

func TestWriteState_MkdirParentFailsViaBlocker(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(filepath.Join(blocker, "child")) // child is under a regular file
	err := s.WriteState(context.Background(), core.State{})
	if err == nil {
		t.Fatal("expected error writing into a path under a regular file")
	}
}

func TestWriteState_TmpCreateFailsInReadOnlyDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission bits")
	}
	dir := t.TempDir()
	ro := filepath.Join(dir, "ro")
	if err := os.MkdirAll(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	// Make absolutely sure we can't write in it.
	if err := os.Chmod(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })

	s := New(ro)
	err := s.WriteState(context.Background(), core.State{})
	if err == nil {
		t.Fatal("expected error writing into a read-only dir")
	}
}

func TestWriteJSONAtomic_MarshalError(t *testing.T) {
	s, _ := newStore(t)
	withHooks(ioHooks{
		marshal: func(any) ([]byte, error) { return nil, errors.New("forced marshal fail") },
	}, func() {
		err := s.WriteState(context.Background(), core.State{})
		if err == nil || !strings.Contains(err.Error(), "marshal") {
			t.Errorf("got %v, want marshal error", err)
		}
	})
}

func TestWriteJSONAtomic_WriteError(t *testing.T) {
	s, _ := newStore(t)
	withHooks(ioHooks{
		write: func(*os.File, []byte) (int, error) { return 0, errors.New("forced write fail") },
	}, func() {
		err := s.WriteState(context.Background(), core.State{})
		if err == nil || !strings.Contains(err.Error(), "write tmp") {
			t.Errorf("got %v, want write tmp error", err)
		}
	})
}

func TestWriteJSONAtomic_SyncError(t *testing.T) {
	s, _ := newStore(t)
	withHooks(ioHooks{
		sync: func(*os.File) error { return errors.New("forced sync fail") },
	}, func() {
		err := s.WriteState(context.Background(), core.State{})
		if err == nil || !strings.Contains(err.Error(), "sync") {
			t.Errorf("got %v, want sync error", err)
		}
	})
}

func TestWriteJSONAtomic_CloseError(t *testing.T) {
	s, _ := newStore(t)
	withHooks(ioHooks{
		closeF: func(*os.File) error { return errors.New("forced close fail") },
	}, func() {
		err := s.WriteState(context.Background(), core.State{})
		if err == nil || !strings.Contains(err.Error(), "close") {
			t.Errorf("got %v, want close error", err)
		}
	})
}

func TestWriteJSONAtomic_RenameError(t *testing.T) {
	s, _ := newStore(t)
	withHooks(ioHooks{
		rename: func(_, _ string) error { return errors.New("forced rename fail") },
	}, func() {
		err := s.WriteState(context.Background(), core.State{})
		if err == nil || !strings.Contains(err.Error(), "rename") {
			t.Errorf("got %v, want rename error", err)
		}
	})
}

func TestAcquireLock_FailsWhenLockPathUnwritable(t *testing.T) {
	// Point storage at a non-existent root with a path component that
	// can't be created (e.g. a file in place of a directory).
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(filepath.Join(blocker, "child"))
	if _, err := s.AcquireLock(context.Background()); err == nil {
		t.Fatal("acquire on un-mkdir-able path must error")
	}
}

func TestAcquireLock_OpenFileErrorWhenPathIsADirectory(t *testing.T) {
	// Make the lock path itself a directory; OpenFile with O_CREATE on a
	// dir-named path returns EISDIR — exercises the OpenFile error
	// branch in acquire() (separate from the MkdirAll branch).
	dir := t.TempDir()
	s := New(dir) // lockPath = <dir>/.lock
	if err := os.MkdirAll(filepath.Join(dir, ".lock"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := s.AcquireLock(context.Background())
	if err == nil {
		t.Fatal("expected open error when lock path is a directory")
	}
}

// readJSON: non-ENOENT read error path.
func TestReadJSON_NonENOENTError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission bits")
	}
	dir := t.TempDir()
	evolveDir := filepath.Join(dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(evolveDir, "state.json")
	if err := os.WriteFile(path, []byte(`{"lastCycleNumber":1}`), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	s := New(evolveDir)
	if _, err := s.ReadState(context.Background()); err == nil {
		t.Fatal("expected permission error on 0o000 state.json")
	}
}

func TestAcquireLock_FlockHeldExternally(t *testing.T) {
	s, _ := newStore(t)
	withLockHooks(lockHooks{
		flock: func(_, _ int) error { return syscall.EWOULDBLOCK },
	}, func() {
		_, err := s.AcquireLock(context.Background())
		if !errors.Is(err, core.ErrLockHeld) {
			t.Errorf("got %v, want ErrLockHeld", err)
		}
	})
}

func TestAcquireLock_FlockOtherError(t *testing.T) {
	s, _ := newStore(t)
	withLockHooks(lockHooks{
		flock: func(_, _ int) error { return errors.New("synthetic flock fail") },
	}, func() {
		_, err := s.AcquireLock(context.Background())
		if err == nil || strings.Contains(err.Error(), "held externally") {
			t.Errorf("got %v, want non-EWOULDBLOCK flock error", err)
		}
	})
}

func TestReleaseLock_CloseError(t *testing.T) {
	s, _ := newStore(t)
	withLockHooks(lockHooks{
		closeF: func(*os.File) error { return errors.New("synthetic close fail") },
	}, func() {
		rel, err := s.AcquireLock(context.Background())
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		relErr := rel()
		if relErr == nil || !strings.Contains(relErr.Error(), "close lock") {
			t.Errorf("got %v, want close lock error", relErr)
		}
	})
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
