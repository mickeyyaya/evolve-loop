package channel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPolicy_StallPolicySatisfiesAndDecides(t *testing.T) {
	var p Policy = StallPolicy{Question: "Summarize progress + blockers in 3 bullets."}

	got := p.OnEvent(map[string]any{"kind": "stall"})
	if got == nil {
		t.Fatalf("OnEvent(stall) = nil, want a non-nil *AskAction")
	}
	want := AskAction{Question: "Summarize progress + blockers in 3 bullets."}
	if *got != want {
		t.Errorf("OnEvent(stall) = %+v, want %+v", *got, want)
	}
	if act := p.OnEvent(map[string]any{"kind": "assistant_text"}); act != nil {
		t.Errorf("OnEvent(non-stall) = %+v, want nil", act)
	}
}

func TestProducer_RunWritesFeed(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-stdout.log"),
		[]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var prod *Producer = NewProducer(ProducerConfig{
		Workspace: ws, Agent: "build", Phase: "build", Cycle: 1,
		PollEvery: time.Millisecond, Now: func() time.Time { return time.Unix(0, 0).UTC() },
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- prod.Run(ctx) }()

	feed := FeedPath(ws, "build")
	deadline := time.Now().Add(2 * time.Second)
	var data []byte
	for time.Now().Before(deadline) {
		data, _ = os.ReadFile(feed)
		if strings.Contains(string(data), `"kind":"assistant_text"`) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(string(data), `"kind":"assistant_text"`) {
		t.Errorf("Producer.Run did not write the content envelope to the feed within 2s:\n%s", data)
	}
}

func TestSupervisor_AskRefusesNonTmux(t *testing.T) {
	var sup *Supervisor = NewSupervisor(SupervisorConfig{
		Workspace: t.TempDir(), Agent: "build", Transport: "claude-p",
	})
	if _, err := sup.Ask(context.Background(), "x"); !errors.Is(err, ErrTransportNoInject) {
		t.Fatalf("Ask over non-tmux transport = %v, want ErrTransportNoInject", err)
	}
}
