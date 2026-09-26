package swarm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// withTmuxRunStub swaps a package variable, so tests that use it must not call t.Parallel().
func withTmuxRunStub(t *testing.T, stub func(ctx context.Context, args ...string) error) *[][]string {
	t.Helper()
	var calls [][]string
	orig := tmuxRun
	tmuxRun = func(ctx context.Context, args ...string) error {
		calls = append(calls, args)
		if stub != nil {
			return stub(ctx, args...)
		}
		return nil
	}
	t.Cleanup(func() { tmuxRun = orig })
	return &calls
}

func TestExecTmuxKill_EmptySessionRefusedBeforeExec(t *testing.T) {
	calls := withTmuxRunStub(t, nil)

	err := ExecTmuxKill(context.Background(), "")

	if err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("ExecTmuxKill(\"\") must refuse with error, got %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("ExecTmuxKill(\"\") must never exec tmux, got calls %v", *calls)
	}
}

func TestExecTmuxKill_NamedSessionKillArgs(t *testing.T) {
	t.Setenv(bridge.TmuxSocketEnv, "")
	calls := withTmuxRunStub(t, nil)

	if err := ExecTmuxKill(context.Background(), "sess-w0"); err != nil {
		t.Fatalf("ExecTmuxKill(named) = %v, want nil", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("want exactly 1 tmux exec, got %d: %v", len(*calls), *calls)
	}
	got := strings.Join((*calls)[0], " ")
	want := "-L " + bridge.TmuxSocket + " kill-session -t sess-w0"
	if got != want {
		t.Fatalf("tmux args = %q, want %q", got, want)
	}
}

func TestExecTmuxKill_RunnerErrorIsBestEffort(t *testing.T) {
	withTmuxRunStub(t, func(context.Context, ...string) error {
		return errors.New("no server running")
	})

	if err := ExecTmuxKill(context.Background(), "already-dead"); err != nil {
		t.Fatalf("ExecTmuxKill must be best-effort on runner error, got %v", err)
	}
}
