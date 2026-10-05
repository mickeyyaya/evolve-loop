package bridge

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestEngineLaunch_BootTimeout_ConfigurableViaEnv(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	const bootPasses, finalScrollbackCaptures = 4, 1
	const wantPolls = bootPasses + finalScrollbackCaptures
	tmux := &fakeTmux{}
	eng := newTestEngine(Deps{
		Tmux:         tmux,
		Sleep:        func(time.Duration) {},
		BootTimeoutS: 4,
	})

	resp, _ := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: fx.profile, Model: "auto",
		Prompt: "do the thing", Workspace: fx.ws, ArtifactPath: fx.artifact,
	})
	if resp.ExitCode != ExitREPLBootTimeout {
		t.Fatalf("ExitCode=%d, want ExitREPLBootTimeout (%d)", resp.ExitCode, ExitREPLBootTimeout)
	}
	if got := len(tmux.captureScrollback); got != wantPolls {
		t.Fatalf("boot polled %d times, want %d: BootTimeoutS=4 at interval 1 must bound the loop at %d passes (not the hardcoded %ds default), "+
			"each pass capturing the pane once, plus %d final scrollback capture", got, wantPolls, bootPasses, tmuxREPLBootTimeoutS, finalScrollbackCaptures)
	}
}

func TestEngineLaunch_BootTimeout_RecordsStrike(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{}
	store := clihealth.NewStore(t.TempDir(), nil)
	eng := newTestEngine(Deps{
		Tmux:             tmux,
		Sleep:            func(time.Duration) {},
		BootTimeoutS:     4,
		BootTimeoutStore: store,
	})

	resp, _ := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: fx.profile, Model: "auto",
		Prompt: "do the thing", Workspace: fx.ws, ArtifactPath: fx.artifact,
	})
	if resp.ExitCode != ExitREPLBootTimeout {
		t.Fatalf("ExitCode=%d, want ExitREPLBootTimeout", resp.ExitCode)
	}

	benches, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load store entries: %v", err)
	}
	entry, ok := benches["claude-tmux"]
	if !ok {
		t.Fatal("expected strike entry to be recorded in store for 'claude-tmux'")
	}
	if entry.Strikes != 1 {
		t.Errorf("entry.Strikes = %d, want 1", entry.Strikes)
	}
}
