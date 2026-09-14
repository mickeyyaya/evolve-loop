package usageprobe

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// 2026-09-15 wave-4 boundary: the loop ignored SIGINT ×2 and SIGTERM for
// over a minute while `[loop] usage-probe: checking …` ran — the probe was
// handed context.Background() and its WaitGroup waited on a bridge session
// that does not return on cancel. The loop's interrupt must win: a cancelled
// context skips the probe entirely, and a cancel that lands mid-probe returns
// promptly even when a probe ignores it, naming what was abandoned.
func TestProber_Run_ReturnsPromptlyOnCancelEvenWhenAProbeIgnoresIt(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	var log bytes.Buffer
	p := &Prober{
		Families: []string{"codex"},
		Probe:    func(context.Context, string) (string, error) { <-block; return "", nil }, // ignores ctx, like a stuck pane
		Classify: bridge.ClassifyExhausted,
		Store:    newStore(t),
		Log:      &log,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.Run(ctx); close(done) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run must return promptly after cancel even though the probe never returns")
	}
	if !strings.Contains(log.String(), "cancelled") || !strings.Contains(log.String(), "1 in-flight") {
		t.Errorf("the abandonment is one readable line naming the count: %q", log.String())
	}
}

func TestProber_Run_SkipsEverythingWhenAlreadyCancelled(t *testing.T) {
	rec := &recordingProbe{pane: map[string]string{"codex": "ok"}}
	var log bytes.Buffer
	p := &Prober{Families: []string{"codex", "claude"}, Probe: rec.probe, Classify: bridge.ClassifyExhausted, Store: newStore(t), Log: &log}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p.Run(ctx)
	if got := rec.probedFamilies(); len(got) != 0 {
		t.Errorf("a cancelled context launches no probe, got %v", got)
	}
	if !strings.Contains(log.String(), "cancelled") {
		t.Errorf("the skip is logged: %q", log.String())
	}
}

func TestProber_Run_AbandonmentCountsOnlyProbesStillRunning(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	var log bytes.Buffer
	p := &Prober{
		Families: []string{"claude", "codex"},
		Probe: func(_ context.Context, family string) (string, error) {
			if family == "codex" {
				<-block // stuck pane
			}
			return "healthy", nil
		},
		Classify: bridge.ClassifyExhausted,
		Store:    newStore(t),
		Log:      &log,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.Run(ctx); close(done) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done
	if !strings.Contains(log.String(), "abandoning 1 in-flight") {
		t.Errorf("the count names probes still running, not probes launched: %q", log.String())
	}
}
