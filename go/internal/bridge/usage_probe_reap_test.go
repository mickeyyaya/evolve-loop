package bridge

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func markerFrames(n int) []string {
	frames := make([]string, n)
	for i := range frames {
		frames[i] = "quota healthy\n❯"
	}
	return frames
}

func probeConfig(t *testing.T, session string) *Config {
	return &Config{CLI: "claude-tmux", Workspace: t.TempDir(), Agent: "usage-probe", SessionName: session,
		Realization: RealizeFor("claude-tmux", LaunchIntent{})}
}

func TestCaptureControl_ReapsTheEphemeralProbeSession(t *testing.T) {
	fake := &FakeTmuxController{CaptureFrames: markerFrames(40)}
	deps := covDeps()
	deps.Tmux = fake
	if _, err := captureControl(context.Background(), probeConfig(t, ""), deps, "claude-tmux", "/usage", helpCaptureSettleTicks); err != nil {
		t.Fatalf("captureControl: %v", err)
	}
	if len(fake.KilledSessions) != 1 || !strings.HasPrefix(fake.KilledSessions[0], "evolve-recipe-") {
		t.Errorf("killed %v, want exactly the probe's own evolve-recipe- session", fake.KilledSessions)
	}
}

func TestCaptureControl_ReapsTheProbeSessionWhenBootFails(t *testing.T) {
	fake := &FakeTmuxController{NewSessionErr: errors.New("tmux: server exited")}
	deps := covDeps()
	deps.Tmux = fake
	if _, err := captureControl(context.Background(), probeConfig(t, ""), deps, "claude-tmux", "/usage", helpCaptureSettleTicks); err == nil {
		t.Fatal("want the boot error")
	}
	if len(fake.KilledSessions) != 1 {
		t.Errorf("killed %v after a failed boot, want the half-made session reaped", fake.KilledSessions)
	}
}

func TestCaptureControl_NeverKillsANamedSession(t *testing.T) {
	fake := &FakeTmuxController{Existing: map[string]bool{"evolve-bridge-named-keep": true}, CaptureFrames: markerFrames(40)}
	deps := covDeps()
	deps.Tmux = fake
	if _, err := captureControl(context.Background(), probeConfig(t, "keep"), deps, "claude-tmux", "/usage", helpCaptureSettleTicks); err != nil {
		t.Fatalf("captureControl: %v", err)
	}
	if len(fake.KilledSessions) != 0 {
		t.Errorf("killed the operator's named session: %v", fake.KilledSessions)
	}
}

type killFailTmux struct{ *FakeTmuxController }

func (k killFailTmux) KillSession(context.Context, string) error {
	return errors.New("tmux: no server")
}

func TestCaptureControl_AFailedReapIsLoud(t *testing.T) {
	var stderr bytes.Buffer
	deps := covDeps()
	deps.Stderr = &stderr
	deps.Tmux = killFailTmux{&FakeTmuxController{CaptureFrames: markerFrames(40)}}
	if _, err := captureControl(context.Background(), probeConfig(t, ""), deps, "claude-tmux", "/usage", helpCaptureSettleTicks); err != nil {
		t.Fatalf("captureControl: %v", err)
	}
	if !strings.Contains(stderr.String(), "not reaped") {
		t.Errorf("a probe session that could not be killed must be named on stderr; got %q", stderr.String())
	}
}
