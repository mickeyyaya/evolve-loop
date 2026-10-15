package channel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func waitForFeed(t *testing.T, path, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if body, err := os.ReadFile(path); err == nil && strings.Contains(string(body), want) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Errorf("feed %s never carried %q", path, want)
}

func appendRaw(t *testing.T, ws, name, line string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(ws, name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		t.Fatal(err)
	}
}

func TestChannel_EndToEnd(t *testing.T) {
	ws := t.TempDir()
	now := func() time.Time { return time.Unix(0, 0).UTC() }

	prod := NewProducer(ProducerConfig{
		Workspace: ws,
		Agent:     "build",
		Phase:     "build",
		Cycle:     1,
		PollEvery: time.Millisecond,
		Now:       now,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = prod.Run(ctx) }()

	sup := NewSupervisor(SupervisorConfig{
		Workspace: ws,
		Agent:     "build",
		Transport: "claude-tmux",
		Now:       now,
		NewID:     func() string { return "c1" },
		PollEvery: time.Millisecond,
		Timeout:   5 * time.Second,
	})

	go func() {
		appendRaw(t, ws, "build-stderr.log", `{"evolve_channel":"inject_applied","corr_id":"c1"}`)

		waitForFeed(t, FeedPath(ws, "build"), `"corr_id":"c1"`)

		appendRaw(t, ws, "build-stdout.log", `{"type":"assistant","message":{"content":[{"type":"text","text":"3 bullets: a,b,c"}]}}`)
		appendRaw(t, ws, "build-stderr.log", `{"evolve_channel":"idle_reached","corr_id":"c1"}`)
	}()

	ans, err := sup.Ask(context.Background(), "summarize")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !strings.Contains(ans.Text(), "3 bullets") {
		t.Fatalf("answer text = %q, want substring %q", ans.Text(), "3 bullets")
	}
}
