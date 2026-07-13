package shipwindow

// shipwindow_edge_test.go — Builder-added edge coverage on top of the TDD
// contract in shipwindow_test.go (which is the spec and stays unmodified):
// break-safety of Release, corrupt-lock recovery, dead-waiter queue sweep,
// and the enqueue error path.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// PathIn is built from FileName — the exported identity gc/operators grep.
func TestFileName_IsPathInBasename(t *testing.T) {
	if got := filepath.Base(PathIn("/x/.evolve")); got != FileName {
		t.Errorf("PathIn basename = %q, want FileName %q", got, FileName)
	}
}

// Release must be a no-op (not a removal) when the lease was broken and
// re-acquired by another lane: the token no longer matches.
func TestRelease_TokenMismatchLeavesForeignLease(t *testing.T) {
	dir := t.TempDir()
	l, err := Acquire(context.Background(), dir, Options{})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	foreign := []byte(`{"owner_pid":1,"heartbeat_at":"2026-07-13T12:00:00Z","token":"someone-else"}`)
	if err := os.WriteFile(PathIn(dir), foreign, 0o644); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if err := l.Release(); err != nil {
		t.Fatalf("Release on foreign lease: %v", err)
	}
	if _, err := os.Stat(PathIn(dir)); err != nil {
		t.Errorf("Release removed a lease it no longer owned: %v", err)
	}
}

// A corrupt lock is crash debris: Release treats it as already-broken (no-op,
// no error), and a waiter's Acquire breaks it as stale and proceeds.
func TestCorruptLock_ReleasedQuietlyAndBrokenByAcquire(t *testing.T) {
	dir := t.TempDir()
	l, err := Acquire(context.Background(), dir, Options{})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := os.WriteFile(PathIn(dir), []byte("not json"), 0o644); err != nil {
		t.Fatalf("corrupt: %v", err)
	}
	if err := l.Release(); err != nil {
		t.Fatalf("Release on corrupt lease: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	l2, err := Acquire(ctx, dir, Options{Poll: 2 * time.Millisecond})
	if err != nil {
		t.Fatalf("Acquire over corrupt lock: %v", err)
	}
	if err := l2.Release(); err != nil {
		t.Errorf("Release: %v", err)
	}
}

// A crashed WAITER's ticket (dead pid) must not wedge FIFO: later waiters
// sweep it and proceed.
func TestQueue_DeadWaiterTicketSwept(t *testing.T) {
	dir := t.TempDir()
	queueDir := filepath.Join(dir, queueDirName)
	if err := os.MkdirAll(queueDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Lexicographically FIRST ticket, owned by a dead pid.
	stale := filepath.Join(queueDir, "00000000000000000001-000000001-4194000")
	if err := os.WriteFile(stale, []byte("4194000\n"), 0o644); err != nil {
		t.Fatalf("stale ticket: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	self := os.Getpid()
	l, err := Acquire(ctx, dir, Options{
		Poll:  2 * time.Millisecond,
		Alive: func(pid int) bool { return pid == self },
	})
	if err != nil {
		t.Fatalf("Acquire behind dead-waiter ticket: %v", err)
	}
	if err := l.Release(); err != nil {
		t.Errorf("Release: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("dead waiter's ticket not swept (stat err=%v)", err)
	}
}

// A queue entry with no parsable pid is fail-safe: never swept, so it holds
// the head and a live waiter blocks until its context expires.
func TestQueue_UnparsableTicketIsNeverSwept(t *testing.T) {
	dir := t.TempDir()
	queueDir := filepath.Join(dir, queueDirName)
	if err := os.MkdirAll(queueDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	junk := filepath.Join(queueDir, "0000junk")
	if err := os.WriteFile(junk, nil, 0o644); err != nil {
		t.Fatalf("junk ticket: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if l, err := Acquire(ctx, dir, Options{Poll: 5 * time.Millisecond}); err == nil {
		_ = l.Release()
		t.Fatalf("Acquire succeeded past an unparsable head ticket; want fail-safe block")
	}
	if _, err := os.Stat(junk); err != nil {
		t.Errorf("unparsable ticket was swept: %v", err)
	}
}

// pidAlive: non-positive pids are never alive; the running test process is.
func TestPidAliveBounds(t *testing.T) {
	if pidAlive(0) || pidAlive(-1) {
		t.Error("pidAlive(non-positive) = true, want false")
	}
	if !pidAlive(os.Getpid()) {
		t.Error("pidAlive(self) = false, want true")
	}
}

// ticketPID: unparsable names yield 0 (fail-safe: never swept).
func TestTicketPIDParsing(t *testing.T) {
	for name, want := range map[string]int{"nodash": 0, "1-notanumber": 0, "1-2-345": 345} {
		if got := ticketPID(name); got != want {
			t.Errorf("ticketPID(%q) = %d, want %d", name, got, want)
		}
	}
}

// Release surfaces a genuine lock-read I/O failure (path readable as neither
// absent nor a lease) instead of guessing.
func TestRelease_ReadErrorSurfaces(t *testing.T) {
	l := &Lease{path: t.TempDir(), token: "x"} // a directory: ReadFile errors, not ENOENT
	if err := l.Release(); err == nil {
		t.Fatal("Release on unreadable lock path: want error, got nil")
	}
}

// tryAcquire fails loudly when the lease cannot be staged (read-only dir).
func TestTryAcquire_StageWriteFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores file modes")
	}
	dir := t.TempDir()
	queueDir := filepath.Join(dir, queueDirName)
	if err := os.MkdirAll(queueDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ticket := "00000000000000000001-000000001-1"
	if err := os.WriteFile(filepath.Join(queueDir, ticket), nil, 0o644); err != nil {
		t.Fatalf("ticket: %v", err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	_, err := tryAcquire(dir, queueDir, ticket, Options{}.normalized())
	if err == nil || !strings.Contains(err.Error(), "stage lease") {
		t.Fatalf("tryAcquire err = %v, want stage-lease failure", err)
	}
}

// The enqueue path fails loudly when the queue directory cannot be created
// (evolveDir path occupied by a regular file).
func TestAcquire_QueueDirCreateFailure(t *testing.T) {
	parent := t.TempDir()
	notADir := filepath.Join(parent, "evolve-as-file")
	if err := os.WriteFile(notADir, nil, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	_, err := Acquire(context.Background(), notADir, Options{})
	if err == nil || !strings.Contains(err.Error(), "queue dir") {
		t.Fatalf("Acquire err = %v, want queue-dir creation failure", err)
	}
}
