package shipwindow

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestStaleHolder_OnStaleBreakReportsALiveHolderBrokenAtTheTTL(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	if _, err := Acquire(context.Background(), dir, Options{Now: func() time.Time { return t0 }, OwnerPID: 4242}); err != nil {
		t.Fatal(err)
	}
	var broken []StaleHolder
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	l, err := Acquire(ctx, dir, Options{
		Now:          func() time.Time { return t0.Add(DefaultTTL + time.Second) },
		Alive:        func(int) bool { return true },
		Poll:         2 * time.Millisecond,
		OnStaleBreak: func(h StaleHolder) { broken = append(broken, h) },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Release() }()
	if len(broken) != 1 || broken[0].OwnerPID != 4242 || broken[0].HeartbeatAt != t0.Format(time.RFC3339Nano) {
		t.Fatalf("stale breaks reported = %+v, want one for the live pid 4242 with its heartbeat", broken)
	}
}

func TestOnStaleBreak_IsSilentForADeadHolder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := Acquire(context.Background(), dir, Options{OwnerPID: 4243}); err != nil {
		t.Fatal(err)
	}
	called := false
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	l, err := Acquire(ctx, dir, Options{
		Alive:        func(pid int) bool { return pid == os.Getpid() },
		Poll:         2 * time.Millisecond,
		OnStaleBreak: func(StaleHolder) { called = true },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Release() }()
	if called {
		t.Fatal("a dead holder is crash debris, not a hung holder, and must not be reported")
	}
}

func TestHeld_WithHeldMarksTheContextOfTheHolder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	l, err := Acquire(context.Background(), dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Release() }()
	if Held(context.Background()) {
		t.Fatal("a bare context claims a held ship window")
	}
	if !Held(WithHeld(context.Background(), l)) {
		t.Fatal("WithHeld did not mark the context")
	}
	if Held(WithHeld(context.Background(), nil)) {
		t.Fatal("a nil lease marks the context as held")
	}
}

func TestStaleHolder_ReasonAndFieldsNameTheBrokenHolder(t *testing.T) {
	t.Parallel()
	h := StaleHolder{OwnerPID: 77, HeartbeatAt: "2026-10-19T01:13:10Z"}
	if got := h.Reason(); got != "ship-window lease of pid 77 broken at the TTL (heartbeat 2026-10-19T01:13:10Z)" {
		t.Fatalf("Reason = %q", got)
	}
	if got := h.Fields(); len(got) != 2 || got["owner_pid"] != "77" || got["heartbeat_at"] != "2026-10-19T01:13:10Z" {
		t.Fatalf("Fields = %v", got)
	}
}
