package channel

import (
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

func TestProducer_FeedOpenError(t *testing.T) {
	// Workspace path that cannot be opened for the feed → Run returns error, no panic.
	p := NewProducer(ProducerConfig{Workspace: string([]byte{0}), Agent: "build", Phase: "build"})
	if err := p.Run(context.Background()); err == nil {
		t.Fatal("want error opening feed in bogus workspace, got nil")
	}
}
