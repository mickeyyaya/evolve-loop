package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

type scriptedDirtyReader struct {
	mu    sync.Mutex
	reads atomic.Int32
	paths []string
	err   error
}

func (s *scriptedDirtyReader) read(_ context.Context, _ string) ([]string, error) {
	s.reads.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	return append([]string(nil), s.paths...), nil
}

func (s *scriptedDirtyReader) write(paths ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paths = paths
}

func snapshotCycleRun(t *testing.T, root string, reader *scriptedDirtyReader) *cycleRun {
	t.Helper()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithGitDirtyPaths(reader.read))
	return &cycleRun{o: o, ctx: context.Background(), req: CycleRequest{ProjectRoot: root}, cycle: 13,
		cs: CycleState{CycleID: 13, WorkspacePath: t.TempDir()}}
}

func checkoutRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestSnapshotMainTree_ANonCheckoutReadsTheStatusOnceAndNeverRetries(t *testing.T) {
	t.Parallel()
	reader := &scriptedDirtyReader{err: errors.New("fatal: not a git repository")}
	cr := snapshotCycleRun(t, t.TempDir(), reader)

	guard, before, err := cr.snapshotMainTree(PhaseBuild)

	if guard != nil || before != nil || err != nil {
		t.Fatalf("a failed read in a non-checkout is no guard and no error: guard=%v before=%v err=%v", guard, before, err)
	}
	if got := reader.reads.Load(); got != 1 {
		t.Fatalf("status reads = %d, want 1: a retry cannot make a non-checkout readable", got)
	}
}

func TestSnapshotMainTree_ANonCheckoutWithAReadableStatusKeepsItsGuard(t *testing.T) {
	t.Parallel()
	reader := &scriptedDirtyReader{paths: []string{"docs/a.md"}}
	cr := snapshotCycleRun(t, t.TempDir(), reader)

	guard, before, err := cr.snapshotMainTree(PhaseBuild)

	if err != nil || guard == nil || !reflect.DeepEqual(before, []string{"docs/a.md"}) {
		t.Fatalf("a readable status keeps the guard and the snapshot: guard=%v before=%v err=%v", guard, before, err)
	}
	if got := reader.reads.Load(); got != 1 {
		t.Fatalf("status reads = %d, want 1", got)
	}
}

func TestSnapshotMainTree_ACheckoutStillRetriesAFailingStatus(t *testing.T) {
	t.Parallel()
	reader := &scriptedDirtyReader{err: errors.New("index.lock exists")}
	cr := snapshotCycleRun(t, checkoutRoot(t), reader)

	guard, _, err := cr.snapshotMainTree(PhaseBuild)

	if guard != nil || !errors.Is(err, errPrePhaseSnapshot) {
		t.Fatalf("a checkout whose status keeps failing aborts the phase: guard=%v err=%v", guard, err)
	}
	if got := reader.reads.Load(); got != snapshotAttempts {
		t.Fatalf("status reads = %d, want %d: a checkout keeps its retries", got, snapshotAttempts)
	}
}

func TestSnapshotMainTree_AGuardReadsTheStatusAgainAfterAWrite(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		written   []string
		wantLeaks []string
	}{
		{"no write between the reads", []string{"docs/a.md"}, nil},
		{"a write adds a path", []string{"docs/a.md", "go/leak.go"}, []string{"go/leak.go"}},
		{"a write on a clean tree", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reader := &scriptedDirtyReader{paths: []string{"docs/a.md"}}
			cr := snapshotCycleRun(t, checkoutRoot(t), reader)
			guard, before, err := cr.snapshotMainTree(PhaseBuild)
			if err != nil || guard == nil {
				t.Fatalf("snapshot: guard=%v err=%v", guard, err)
			}

			reader.write(tc.written...)
			res := guard.Check(context.Background(), cr.req.ProjectRoot, before)

			if !reflect.DeepEqual(res.Leaked, tc.wantLeaks) {
				t.Fatalf("leaked = %v, want %v: the check must read the status after the write, not reuse the snapshot", res.Leaked, tc.wantLeaks)
			}
			if got := reader.reads.Load(); got != 2 {
				t.Fatalf("status reads = %d, want 2: one before the write and one after", got)
			}
		})
	}
}
