package observer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestCoreAdapter_SpawnsProducer_WhenChannelOn(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "build-stdout.log"),
		[]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`+"\n"), 0o644)
	a := &CoreAdapter{RecoveryStage: "enforce"}
	cancel := a.Start(context.Background(), "build", core.PhaseRequest{Workspace: ws, Cycle: 1})
	time.Sleep(40 * time.Millisecond)
	cancel()
	if _, err := os.Stat(channel.FeedPath(ws, "build")); err != nil {
		t.Fatalf("feed not produced: %v", err)
	}
}

func TestCoreAdapter_NoProducer_WhenChannelOff(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "build-stdout.log"), []byte("{}\n"), 0o644)
	a := &CoreAdapter{}
	cancel := a.Start(context.Background(), "build", core.PhaseRequest{Workspace: ws, Cycle: 1})
	time.Sleep(20 * time.Millisecond)
	cancel()
	if _, err := os.Stat(channel.FeedPath(ws, "build")); !os.IsNotExist(err) {
		t.Fatalf("feed should not exist when channel off, err=%v", err)
	}
}

func TestChannelSourcePaths_TmuxFamily(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	a := &CoreAdapter{}

	cases := []struct {
		name string
		env  map[string]string
	}{
		{"default (unset → claude-tmux)", nil},
		{"explicit claude-tmux", map[string]string{"EVOLVE_BUILD_CLI": "claude-tmux"}},
		{"codex-tmux", map[string]string{"EVOLVE_BUILD_CLI": "codex-tmux"}},
		{"global EVOLVE_CLI agy-tmux", map[string]string{"EVOLVE_CLI": "agy-tmux"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := core.PhaseRequest{Workspace: ws, Env: tc.env}
			stdout, stderr := a.channelSourcePaths(req, "build")
			if stdout != filepath.Join(ws, "build-pane.live") {
				t.Errorf("stdout=%q, want build-pane.live", stdout)
			}
			if stderr != filepath.Join(ws, "build-breadcrumbs.live") {
				t.Errorf("stderr=%q, want build-breadcrumbs.live", stderr)
			}
		})
	}
}

func TestChannelSourcePaths_Headless(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	a := &CoreAdapter{}
	req := core.PhaseRequest{Workspace: ws, Env: map[string]string{"EVOLVE_BUILD_CLI": "claude-p"}}
	stdout, stderr := a.channelSourcePaths(req, "build")
	if stdout != "" || stderr != "" {
		t.Errorf("headless must return empty paths (producer legacy), got stdout=%q stderr=%q", stdout, stderr)
	}
}

// Process env is never read at all; TestCoreAdapter_HasNoStderrLinesAndNoEnvReads
// pins that. This test covers the request-scoped key and its hyphen mapping.
func TestChannelSourcePaths_PerAgentKeyIgnoresProcessEnv(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	a := &CoreAdapter{}
	req := core.PhaseRequest{
		Workspace: ws,
		Env:       map[string]string{"EVOLVE_TDD_ENGINEER_CLI": "claude-p"},
	}
	stdout, stderr := a.channelSourcePaths(req, "tdd-engineer")
	if stdout != "" || stderr != "" {
		t.Errorf("request-scoped claude-p must select headless legacy, got stdout=%q stderr=%q", stdout, stderr)
	}
}
