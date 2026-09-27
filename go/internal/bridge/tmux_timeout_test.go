package bridge

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunCmdBounded_KillsHungSubprocess(t *testing.T) {
	start := time.Now()
	_, err := runCmdBounded(context.Background(), 150*time.Millisecond, "sleep", "30")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error from a subprocess that outlives the deadline, got nil")
	}
	if elapsed > 3*time.Second {
		t.Fatalf("runCmdBounded blocked %v on a hung subprocess — per-call timeout not enforced", elapsed)
	}
}

func TestRunCmdBounded_ReturnsOutputWhenFast(t *testing.T) {
	out, err := runCmdBounded(context.Background(), 5*time.Second, "echo", "hello-bounded")
	if err != nil {
		t.Fatalf("unexpected error on a fast command: %v", err)
	}
	if strings.TrimSpace(out) != "hello-bounded" {
		t.Fatalf("output = %q, want %q", strings.TrimSpace(out), "hello-bounded")
	}
}

func TestRunCmdBounded_HonorsParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()

	start := time.Now()
	_, err := runCmdBounded(ctx, 30*time.Second, "sleep", "30")
	if err == nil {
		t.Fatal("expected error when parent ctx cancelled, got nil")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("parent cancellation took %v to unblock — not honored", elapsed)
	}
}
