package channel

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProducer_WritesContentAndCorrelation(t *testing.T) {
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "build-stdout.log"),
		[]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"working"}]}}`+"\n"), 0o644)
	os.WriteFile(filepath.Join(ws, "build-stderr.log"),
		[]byte(`{"evolve_channel":"inject_applied","corr_id":"c1"}`+"\n"), 0o644)

	p := NewProducer(ProducerConfig{Workspace: ws, Agent: "build", Phase: "build", Cycle: 1,
		PollEvery: time.Millisecond, Now: func() time.Time { return time.Unix(0, 0).UTC() }})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	<-done

	data, _ := os.ReadFile(FeedPath(ws, "build"))
	if !strings.Contains(string(data), `"kind":"assistant_text"`) {
		t.Fatalf("feed missing content envelope:\n%s", data)
	}
	if !strings.Contains(string(data), `"kind":"correlation"`) {
		t.Fatalf("feed missing correlation envelope:\n%s", data)
	}
}

// TestProducer_ExplicitSourcePaths asserts the producer tails the configured
// StdoutPath/StderrPath (the tmux .live pair) and IGNORES the legacy
// <phase>-stdout.log when explicit paths are set.
func TestProducer_ExplicitSourcePaths(t *testing.T) {
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "build-pane.live"),
		[]byte("tmux is a terminal multiplexer\n"), 0o644)
	os.WriteFile(filepath.Join(ws, "build-breadcrumbs.live"),
		[]byte(`{"evolve_channel":"inject_applied","corr_id":"c1"}`+"\n"), 0o644)
	// Legacy log present but must be ignored when explicit paths override.
	os.WriteFile(filepath.Join(ws, "build-stdout.log"), []byte("LEGACY-STDOUT-MUST-NOT-APPEAR\n"), 0o644)

	p := NewProducer(ProducerConfig{
		Workspace: ws, Agent: "build", Phase: "build", Cycle: 1,
		StdoutPath: filepath.Join(ws, "build-pane.live"),
		StderrPath: filepath.Join(ws, "build-breadcrumbs.live"),
		PollEvery:  time.Millisecond, Now: func() time.Time { return time.Unix(0, 0).UTC() },
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	<-done

	data, _ := os.ReadFile(FeedPath(ws, "build"))
	if !strings.Contains(string(data), "terminal multiplexer") {
		t.Errorf("feed missing pane.live content:\n%s", data)
	}
	if !strings.Contains(string(data), `"kind":"correlation"`) {
		t.Errorf("feed missing breadcrumbs.live correlation:\n%s", data)
	}
	if strings.Contains(string(data), "LEGACY-STDOUT") {
		t.Errorf("feed leaked legacy stdout.log despite explicit StdoutPath:\n%s", data)
	}
}

// TestProducer_WarnsWhenSourceNeverAppears asserts that a channel-on phase whose
// content source file never materializes (an agent/phase name mismatch or a
// mis-resolved CLI family pointing at a tmux driver's empty stdout.log) emits a
// loud WARN rather than silently producing an empty feed for the whole phase.
func TestProducer_WarnsWhenSourceNeverAppears(t *testing.T) {
	ws := t.TempDir()
	var warn bytes.Buffer
	p := NewProducer(ProducerConfig{
		Workspace: ws, Agent: "build", Phase: "build",
		StdoutPath: filepath.Join(ws, "build-pane.live"), // never created
		StderrPath: filepath.Join(ws, "build-breadcrumbs.live"),
		PollEvery:  time.Millisecond, Now: func() time.Time { return time.Unix(0, 0).UTC() },
		Warn: &warn,
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = p.Run(ctx); close(done) }()
	time.Sleep(60 * time.Millisecond) // >> the miss threshold at 1ms polls
	cancel()
	<-done // wait for Run to return so the buffer read does not race the writer

	if !strings.Contains(warn.String(), "source") {
		t.Errorf("expected a WARN that the channel source never appeared; got %q", warn.String())
	}
	// The WARN must fire exactly once (not every poll).
	if n := strings.Count(warn.String(), "never appeared"); n > 1 {
		t.Errorf("WARN fired %d times, want exactly 1", n)
	}
}

// TestProducer_NoWarnWhenSourceExists asserts the never-appeared WARN does NOT
// fire when the source file is present (the normal case, incl. a legitimately
// quiet phase whose file exists but is empty).
func TestProducer_NoWarnWhenSourceExists(t *testing.T) {
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "build-pane.live"), nil, 0o644) // exists, empty
	var warn bytes.Buffer
	p := NewProducer(ProducerConfig{
		Workspace: ws, Agent: "build", Phase: "build",
		StdoutPath: filepath.Join(ws, "build-pane.live"),
		PollEvery:  time.Millisecond, Now: func() time.Time { return time.Unix(0, 0).UTC() },
		Warn: &warn,
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = p.Run(ctx); close(done) }()
	time.Sleep(60 * time.Millisecond)
	cancel()
	<-done // wait for Run to return so the buffer read does not race the writer
	if strings.Contains(warn.String(), "never appeared") {
		t.Errorf("must not WARN when source file exists; got %q", warn.String())
	}
}

func TestProducer_FeedOpenError(t *testing.T) {
	// Workspace path that cannot be opened for the feed → Run returns error, no panic.
	p := NewProducer(ProducerConfig{Workspace: string([]byte{0}), Agent: "build", Phase: "build"})
	if err := p.Run(context.Background()); err == nil {
		t.Fatal("want error opening feed in bogus workspace, got nil")
	}
}
