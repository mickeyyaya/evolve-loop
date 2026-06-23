package bridge

// driver_tmux_repl_boottimeout_test.go — the REPL boot deadline must be
// overridable via Deps.BootTimeoutS (BridgePolicy.BootTimeoutS; default
// tmuxREPLBootTimeoutS=60). On a loaded CI runner the fake-CLI/tmux handshake
// intermittently exceeds the fixed budget ("REPL prompt never appeared after
// 60s"); raising the typed field gives the poll loop more iterations so the
// integration tier stops flaking.

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

func TestEngineLaunch_BootTimeout_ConfigurableViaEnv(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	// CapturePane always "" → the marker is never seen → the boot loop polls to
	// its deadline then returns ExitREPLBootTimeout. captureScrollback records
	// one entry per poll iteration PLUS one from the deferred tmuxCleanup
	// final-scrollback capture (it fires on every exit path, including the
	// boot-timeout return), so its length == BootTimeoutS / bootIntervalS
	// + 1 (claude bootIntervalS=1). With the field at 4 that is 5; without the
	// field it would be 61 (the hardcoded 60s default), so an exact 5 proves
	// the typed field bounds the loop.
	const wantPolls = 4 + 1 // 4 boot-poll iterations (env) + 1 deferred tmuxCleanup capture
	tmux := &fakeTmux{}
	eng := NewEngine(Deps{
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
