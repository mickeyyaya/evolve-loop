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
	// CapturePane always returns "", so the boot loop polls to its deadline
	// before returning ExitREPLBootTimeout; claude-tmux also ticks the
	// auto-responder during boot, so each iteration captures the pane twice.
	const wantPolls = 4*2 + 1 // 4 boot-poll iterations × 2 captures (loop + tick) + 1 deferred tmuxCleanup capture
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
		t.Fatalf("boot polled %d times, want %d (BootTimeoutS=4, interval=1) — "+
			"the typed field must bound the loop, not the hardcoded %ds default", got, wantPolls, tmuxREPLBootTimeoutS)
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
