package channel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// waitForFeed blocks until the feed file contains want, or fails the test after five seconds.
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

// appendRaw appends a single line (with a newline) to a file in ws.
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

// TestChannel_EndToEnd drives the real Producer + Supervisor against hand-written
// raw logs that simulate the driver, exercising the full ask→answer correlation
// path end-to-end.
//
// Seq mechanics (verified from classifier.go / correlation.go):
//
//	Poll 1 (stderr only):
//	  inject_applied → Stderr: c.corr.observe(line, seq=0) → atSeq=0
//	                   Emit(KindCorrelation) → seq=1, at_seq=0 (request)
//
//	Poll 2 (stdout then stderr):
//	  assistant     → Line: KindAssistantText → seq=2
//	  idle_reached  → Stderr: c.corr.observe(line, seq=2) → startSeq=1, endSeq=2
//	                  Emit(KindCorrelation) → seq=3, response_complete{start_seq:1,end_seq:2}
//
//	collectSpan([1,2]) collects seq=2 (assistant_text) → Text() contains "3 bullets".
//
// The second append waits for the first's envelope in the feed, so the
// Producer processes inject_applied in an earlier poll than the assistant +
// idle_reached pair and the assistant envelope takes a seq inside the span.
func TestChannel_EndToEnd(t *testing.T) {
	ws := t.TempDir()
	now := func() time.Time { return time.Unix(0, 0).UTC() }

	// Producer: polls the raw logs every 1 ms and writes normalized envelopes
	// to the channel feed.
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

	// Supervisor: injects questions via inbox and waits for the correlated
	// response_complete span in the feed.
	sup := NewSupervisor(SupervisorConfig{
		Workspace: ws,
		Agent:     "build",
		Transport: "claude-tmux",
		Now:       now,
		NewID:     func() string { return "c1" },
		PollEvery: time.Millisecond,
		Timeout:   5 * time.Second,
	})

	// Driver-side simulation: write breadcrumbs and content in two distinct
	// poll windows so the Normalizer assigns the assistant envelope a seq that
	// falls inside the [startSeq, endSeq] span.
	go func() {
		// Tick 1: inject_applied arrives in stderr.
		// Normalizer processes stdout (empty) then stderr → seq=1 request correlation.
		appendRaw(t, ws, "build-stderr.log", `{"evolve_channel":"inject_applied","corr_id":"c1"}`)

		// Tick 2 must land in a later poll than tick 1 whatever the host's load, so wait for the
		// feed to carry tick 1's envelope instead of assuming the ticker fired in between.
		waitForFeed(t, FeedPath(ws, "build"), `"corr_id":"c1"`)

		// Tick 2: assistant content in stdout + idle_reached in stderr.
		// Normalizer: stdout first → seq=2 assistant_text; stderr → seq=3 response_complete{start:1,end:2}.
		// collectSpan([1,2]) captures seq=2.
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
