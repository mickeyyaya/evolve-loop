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

// TestPolicy_StallPolicySatisfiesAndDecides binds both the Policy interface and
// the *AskAction return type to their real producer StallPolicy.OnEvent.
//   - var _ Policy = StallPolicy{} pins that StallPolicy implements Policy
//     (the seam a future LLM-driven policy must also satisfy — ADR-0037).
//   - On a "stall" envelope OnEvent must return a non-nil *AskAction whose
//     Question field is exactly the configured prompt (full-field assertion).
//   - On any non-stall envelope it must return nil (no spurious injection).
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

// TestProducer_RunWritesFeed binds the *Producer type to its constructor
// NewProducer and exercises Run end-to-end: a Producer over a phase's raw logs
// must write the normalized content envelope into the canonical FeedPath. This
// asserts the single-writer Producer's core contract (it is the SOLE writer of
// the feed) rather than merely naming the type.
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

	// Poll the feed until the envelope appears (or a generous cap) rather than
	// a fixed sleep — robust under slow CI.
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

// TestSupervisor_AskRefusesNonTmux binds the *Supervisor type to its
// constructor NewSupervisor and exercises Ask's transport guard: a non-tmux
// transport cannot receive live injection, so Ask must return
// ErrTransportNoInject (a real behavioral contract, not a tautology).
func TestSupervisor_AskRefusesNonTmux(t *testing.T) {
	var sup *Supervisor = NewSupervisor(SupervisorConfig{
		Workspace: t.TempDir(), Agent: "build", Transport: "claude-p",
	})
	if _, err := sup.Ask(context.Background(), "x"); !errors.Is(err, ErrTransportNoInject) {
		t.Fatalf("Ask over non-tmux transport = %v, want ErrTransportNoInject", err)
	}
}
