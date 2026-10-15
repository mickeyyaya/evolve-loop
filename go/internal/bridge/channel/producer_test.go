package channel

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestProducer_ExplicitSourcePaths(t *testing.T) {
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "build-pane.live"),
		[]byte("tmux is a terminal multiplexer\n"), 0o644)
	os.WriteFile(filepath.Join(ws, "build-breadcrumbs.live"),
		[]byte(`{"evolve_channel":"inject_applied","corr_id":"c1"}`+"\n"), 0o644)
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

func TestProducer_WarnsWhenSourceNeverAppears(t *testing.T) {
	ws := t.TempDir()
	warn := &lockedBuf{}
	p := NewProducer(ProducerConfig{
		Workspace: ws, Agent: "build", Phase: "build",
		StdoutPath: filepath.Join(ws, "build-pane.live"),
		StderrPath: filepath.Join(ws, "build-breadcrumbs.live"),
		PollEvery:  time.Millisecond, Now: func() time.Time { return time.Unix(0, 0).UTC() },
		Warn: warn,
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = p.Run(ctx); close(done) }()
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(warn.String(), "source") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done

	if !strings.Contains(warn.String(), "source") {
		t.Errorf("expected a WARN that the channel source never appeared; got %q", warn.String())
	}
	if n := strings.Count(warn.String(), "never appeared"); n > 1 {
		t.Errorf("WARN fired %d times, want exactly 1", n)
	}
}

func TestProducer_NoWarnWhenSourceExists(t *testing.T) {
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "build-pane.live"), nil, 0o644)
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
	<-done
	if strings.Contains(warn.String(), "never appeared") {
		t.Errorf("must not WARN when source file exists; got %q", warn.String())
	}
}

func TestProducer_FeedOpenError(t *testing.T) {
	p := NewProducer(ProducerConfig{Workspace: string([]byte{0}), Agent: "build", Phase: "build"})
	if err := p.Run(context.Background()); err == nil {
		t.Fatal("want error opening feed in bogus workspace, got nil")
	}
}

type lockedBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
