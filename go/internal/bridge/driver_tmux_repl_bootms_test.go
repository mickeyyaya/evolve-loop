package bridge

// driver_tmux_repl_bootms_test.go — A0 cold-boot instrumentation (ADR-0043).
// Proves the full chain driver→Deps.OnBoot→BridgeResponse.BootMS: the tmux-REPL
// driver reports the cold-boot wait (the 2 fixed readiness sleeps + the marker
// poll), the Engine captures it onto the response, and a launch that never
// completes a cold boot reports BootMS=0.

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestEngineLaunch_BootMS_CapturedOnColdBoot(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	// Pre-seed the artifact so the post-boot wait loop exits on its first check
	// and the launch reaches ExitOK — isolating the boot window we assert on.
	if err := os.WriteFile(fx.artifact, []byte("<!-- challenge-token: "+fx.token+" -->\nDONE\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	// paneSeq=[marker] → the REPL prompt appears on the first CapturePane, i.e.
	// the first poll iteration. claude-tmux bootIntervalS=1, so the modelled
	// boot = 2000ms (two fixed readiness sleeps) + 1×1000ms (one poll) = 3000ms.
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	var onBoot []int64
	eng := NewEngine(Deps{
		Tmux:      tmux,
		Sleep:     func(time.Duration) {},
		LookupEnv: mapLookup(nil),
		OnBoot:    func(ms int64) { onBoot = append(onBoot, ms) },
	})

	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI:          "claude-tmux",
		Profile:      fx.profile,
		Model:        "auto",
		Prompt:       "do the thing",
		Workspace:    fx.ws,
		ArtifactPath: fx.artifact,
	})
	if err != nil {
		t.Fatalf("Launch returned error: %v", err)
	}
	if resp.ExitCode != ExitOK {
		t.Fatalf("ExitCode = %d, want ExitOK (%d)", resp.ExitCode, ExitOK)
	}
	const wantBoot = int64(3000)
	if resp.BootMS != wantBoot {
		t.Errorf("BridgeResponse.BootMS = %d, want %d (2000 fixed + 1×1000 poll)", resp.BootMS, wantBoot)
	}
	// The Engine chains the pre-wired OnBoot, so the caller's callback fires once
	// with the same value (single cold boot per Launch).
	if len(onBoot) != 1 || onBoot[0] != wantBoot {
		t.Errorf("OnBoot calls = %v, want exactly [%d]", onBoot, wantBoot)
	}
}

func TestEngineLaunch_BootMS_ZeroWhenBootNeverCompletes(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	// CapturePane always "" → the prompt marker is never seen → the driver
	// returns ExitREPLBootTimeout BEFORE reporting OnBoot. No completed cold
	// boot ⇒ BootMS stays 0 (the same signal a warm/resumed named session gives).
	tmux := &fakeTmux{}
	var onBoot []int64
	eng := NewEngine(Deps{
		Tmux:      tmux,
		Sleep:     func(time.Duration) {},
		LookupEnv: mapLookup(nil),
		OnBoot:    func(ms int64) { onBoot = append(onBoot, ms) },
	})

	resp, _ := eng.Launch(context.Background(), core.BridgeRequest{
		CLI:          "claude-tmux",
		Profile:      fx.profile,
		Model:        "auto",
		Prompt:       "do the thing",
		Workspace:    fx.ws,
		ArtifactPath: fx.artifact,
	})
	if resp.ExitCode != ExitREPLBootTimeout {
		t.Fatalf("ExitCode = %d, want ExitREPLBootTimeout (%d)", resp.ExitCode, ExitREPLBootTimeout)
	}
	if resp.BootMS != 0 {
		t.Errorf("BootMS = %d, want 0 (boot never completed)", resp.BootMS)
	}
	if len(onBoot) != 0 {
		t.Errorf("OnBoot must not fire on a failed boot; got %v", onBoot)
	}
}

func TestEngineLaunch_BootMS_ZeroOnWarmNamedSession(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	if err := os.WriteFile(fx.artifact, []byte("<!-- challenge-token: "+fx.token+" -->\nDONE\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	// A pre-existing named session → namedExists=true → the driver RESUMEs and
	// skips the entire cold-boot block, so OnBoot is never reached and BootMS
	// stays 0. This is the ADR-0043 contract ("warm named session → 0") that the
	// boot-timeout test only covers by proxy; here it is exercised directly, so a
	// refactor that moved the OnBoot call out of the `if !namedExists` block fails.
	const sessName = "warm1"
	tmux := &fakeTmux{
		existing: map[string]bool{NamedSessionName(sessName): true},
		paneSeq:  []string{tmuxPromptMarkerDefault},
	}
	var onBoot []int64
	eng := NewEngine(Deps{
		Tmux:      tmux,
		Sleep:     func(time.Duration) {},
		LookupEnv: mapLookup(nil),
		OnBoot:    func(ms int64) { onBoot = append(onBoot, ms) },
	})

	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI:          "claude-tmux",
		Profile:      fx.profile,
		Model:        "auto",
		Prompt:       "do the thing",
		Workspace:    fx.ws,
		ArtifactPath: fx.artifact,
		SessionName:  sessName,
	})
	if err != nil {
		t.Fatalf("Launch returned error: %v", err)
	}
	if resp.ExitCode != ExitOK {
		t.Fatalf("ExitCode = %d, want ExitOK (%d)", resp.ExitCode, ExitOK)
	}
	if resp.BootMS != 0 {
		t.Errorf("BootMS = %d, want 0 (warm named session skips cold boot)", resp.BootMS)
	}
	if len(onBoot) != 0 {
		t.Errorf("OnBoot must not fire on a warm/resumed session; got %v", onBoot)
	}
}
