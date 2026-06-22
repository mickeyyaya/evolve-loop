package bridge

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestRunCmdBounded_KillsHungSubprocess is the regression for the cleanup-sweep
// hang (flag-campaign-8): a wedged tmux server made `tmux capture-pane`'s
// cmd.Run() block on read() forever, freezing the completion wait loop with the
// deliverable already on disk. execTmux.run used exec.CommandContext with NO
// per-call timeout, so a hung tmux could only be unblocked by ctx cancellation —
// which the wait loop checks only between iterations and which the 2h
// cycle-timeout failed to deliver. runCmdBounded must kill a subprocess that
// outlives the per-call timeout so a wedged tmux can never freeze the loop.
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

// TestRunCmdBounded_ReturnsOutputWhenFast: a normal (fast) command completes and
// its combined output is returned unchanged — the timeout is a backstop, never a
// throttle on healthy calls.
func TestRunCmdBounded_ReturnsOutputWhenFast(t *testing.T) {
	out, err := runCmdBounded(context.Background(), 5*time.Second, "echo", "hello-bounded")
	if err != nil {
		t.Fatalf("unexpected error on a fast command: %v", err)
	}
	if strings.TrimSpace(out) != "hello-bounded" {
		t.Fatalf("output = %q, want %q", strings.TrimSpace(out), "hello-bounded")
	}
}

// TestRunCmdBounded_HonorsParentCancellation: cancelling the parent ctx kills the
// subprocess before the (longer) per-call timeout — the per-call deadline layers
// on top of ctx, it does not replace it.
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
