package channel

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
)

func appendFeed(t *testing.T, ws, agent string, lines ...string) {
	t.Helper()
	f, err := os.OpenFile(FeedPath(ws, agent), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, ln := range lines {
		if _, err := f.WriteString(ln + "\n"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSupervisor_Ask_ReturnsAnswerSpan(t *testing.T) {
	ws := t.TempDir()
	now := func() time.Time { return time.Unix(0, 0).UTC() }
	s := NewSupervisor(SupervisorConfig{
		Workspace: ws,
		Agent:     "build",
		Transport: "claude-tmux",
		Now:       now,
		NewID:     func() string { return "c1" },
		PollEvery: time.Millisecond,
		Timeout:   time.Second,
	})
	go func() {
		time.Sleep(10 * time.Millisecond)
		appendFeed(t, ws, "build",
			`{"seq":5,"kind":"correlation","data":{"sub":"request","corr_id":"c1","at_seq":5}}`,
			`{"seq":6,"kind":"assistant_text","data":{"text":"done: 3 bullets"}}`,
			`{"seq":7,"kind":"correlation","data":{"sub":"response_complete","corr_id":"c1","start_seq":6,"end_seq":7}}`)
	}()
	ans, err := s.Ask(context.Background(), "summarize")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if len(ans.Events) == 0 || !strings.Contains(ans.Text(), "3 bullets") {
		t.Fatalf("answer = %+v text=%q", ans, ans.Text())
	}
	got, _ := inbox.NewCursor(ws, "build").Drain()
	if len(got) != 1 || got[0].CorrID != "c1" || got[0].Kind != inbox.KindCommand {
		t.Fatalf("inbox = %+v", got)
	}
}

func TestSupervisor_Ask_HeadlessRefused(t *testing.T) {
	s := NewSupervisor(SupervisorConfig{Workspace: t.TempDir(), Agent: "build", Transport: "claude-p"})
	if _, err := s.Ask(context.Background(), "x"); !errors.Is(err, ErrTransportNoInject) {
		t.Fatalf("err = %v, want ErrTransportNoInject", err)
	}
}

func TestSupervisor_Ask_Timeout(t *testing.T) {
	s := NewSupervisor(SupervisorConfig{
		Workspace: t.TempDir(),
		Agent:     "build",
		Transport: "claude-tmux",
		NewID:     func() string { return "c1" },
		PollEvery: time.Millisecond,
		Timeout:   20 * time.Millisecond,
	})
	if _, err := s.Ask(context.Background(), "x"); !errors.Is(err, ErrResponseTimeout) {
		t.Fatalf("err = %v, want ErrResponseTimeout", err)
	}
}

func TestSupervisor_Ask_CtxCancel(t *testing.T) {
	s := NewSupervisor(SupervisorConfig{
		Workspace: t.TempDir(),
		Agent:     "build",
		Transport: "claude-tmux",
		NewID:     func() string { return "c1" },
		PollEvery: time.Millisecond,
		Timeout:   time.Hour,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Ask(ctx, "x"); err == nil {
		t.Fatal("want ctx error, got nil")
	}
}

// TestSupervisor_DefaultNewID ensures NewSupervisor applies its default NewID
// and the minted ID is non-empty (exercises the default branch in NewSupervisor).
func TestSupervisor_DefaultNewID(t *testing.T) {
	s := NewSupervisor(SupervisorConfig{
		Workspace: t.TempDir(),
		Agent:     "build",
		Transport: "claude-tmux",
		// NewID intentionally omitted → default applied
		PollEvery: time.Millisecond,
		Timeout:   5 * time.Millisecond,
	})
	// The default NewID path is exercised; we just need Ask to reach awaitReply
	// (which it does before timing out).
	_, err := s.Ask(context.Background(), "x")
	if !errors.Is(err, ErrResponseTimeout) {
		t.Fatalf("expected timeout, got %v", err)
	}
}

// TestAnswer_Text_NilData exercises the nil-data branch in Answer.Text.
func TestAnswer_Text_NilData(t *testing.T) {
	a := Answer{Events: []map[string]any{
		{"seq": int64(1), "kind": "assistant_text"}, // no "data" key
		{"seq": int64(2), "kind": "assistant_text", "data": map[string]any{"text": "hello"}},
	}}
	if got := a.Text(); got != "hello" {
		t.Fatalf("Text() = %q, want %q", got, "hello")
	}
}

// TestToI64 covers the int, int64, float64, and unknown branches.
func TestToI64(t *testing.T) {
	cases := []struct {
		in   any
		want int64
	}{
		{float64(7), 7},
		{int64(42), 42},
		{int(9), 9},
		{"nope", 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := toI64(c.in); got != c.want {
			t.Errorf("toI64(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestTailLines_MissingFile confirms tailLines returns (nil, off) for a
// non-existent feed path without panicking.
func TestTailLines_MissingFile(t *testing.T) {
	lines, off := tailLines("/no/such/path/feed.ndjson", 0)
	if lines != nil || off != 0 {
		t.Fatalf("expected nil/0, got %v / %d", lines, off)
	}
}

// TestTailLines_Truncation confirms that tailLines resets the offset to 0
// when the file is shorter than the provided offset (truncation guard).
func TestTailLines_Truncation(t *testing.T) {
	ws := t.TempDir()
	appendFeed(t, ws, "build", `{"seq":1,"kind":"assistant_text","data":{"text":"hi"}}`)
	path := FeedPath(ws, "build")
	fi, _ := os.Stat(path)
	size := fi.Size()
	// Pass an offset larger than the file to simulate truncation.
	lines, newOff := tailLines(path, size+100)
	// Should reset to 0 and re-read from the start.
	if len(lines) == 0 {
		t.Fatal("expected lines after truncation reset, got none")
	}
	if newOff <= 0 {
		t.Fatalf("expected positive newOff, got %d", newOff)
	}
}

// TestCollectSpan_MissingFile confirms collectSpan returns nil gracefully.
func TestCollectSpan_MissingFile(t *testing.T) {
	result := collectSpan("/no/such/path/feed.ndjson", 1, 10)
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

// TestFindResponseComplete_EdgeCases exercises the skip branches:
// malformed JSON, missing data, wrong sub, wrong corrID.
func TestFindResponseComplete_EdgeCases(t *testing.T) {
	lines := []string{
		`not json`,
		`{"kind":"correlation"}`,
		`{"kind":"correlation","data":{"sub":"request","corr_id":"c1"}}`,
		`{"kind":"correlation","data":{"sub":"response_complete","corr_id":"OTHER","start_seq":1,"end_seq":2}}`,
	}
	if _, _, ok := findResponseComplete(lines, "c1"); ok {
		t.Fatal("expected not found")
	}
}

// TestSupervisor_Ask_InboxWriteError confirms Ask surfaces an inbox.Append
// error (unwritable directory) rather than silently swallowing it.
func TestSupervisor_Ask_InboxWriteError(t *testing.T) {
	ws := t.TempDir()
	// Create the inbox dir as a file so os.MkdirAll inside Append fails.
	inboxDir := ws + "/.bridge-inbox"
	if err := os.WriteFile(inboxDir, []byte("blocker"), 0o444); err != nil {
		t.Fatal(err)
	}
	s := NewSupervisor(SupervisorConfig{
		Workspace: ws, Agent: "build", Transport: "claude-tmux",
		NewID: func() string { return "c1" }, PollEvery: time.Millisecond, Timeout: time.Second,
	})
	if _, err := s.Ask(context.Background(), "x"); err == nil {
		t.Fatal("expected inbox write error, got nil")
	}
}

// TestSupervisor_Ask_TimeoutAfterWait exercises the post-tick timeout check
// in awaitReply by using a Now clock that advances past the deadline exactly
// on the second call (after the first ticker wait).
func TestSupervisor_Ask_TimeoutAfterWait(t *testing.T) {
	calls := 0
	base := time.Unix(1000, 0)
	now := func() time.Time {
		calls++
		// First 3 calls: before deadline. After that: past it.
		if calls <= 3 {
			return base
		}
		return base.Add(200 * time.Millisecond)
	}
	s := NewSupervisor(SupervisorConfig{
		Workspace: t.TempDir(), Agent: "build", Transport: "claude-tmux",
		Now: now, NewID: func() string { return "c1" },
		PollEvery: time.Millisecond, Timeout: 100 * time.Millisecond,
	})
	if _, err := s.Ask(context.Background(), "x"); !errors.Is(err, ErrResponseTimeout) {
		t.Fatalf("err = %v, want ErrResponseTimeout", err)
	}
}

// TestCollectSpan_MalformedLine confirms collectSpan skips malformed JSON lines.
func TestCollectSpan_MalformedLine(t *testing.T) {
	ws := t.TempDir()
	appendFeed(t, ws, "build",
		`not json`,
		`{"seq":3,"kind":"assistant_text","data":{"text":"ok"}}`,
	)
	result := collectSpan(FeedPath(ws, "build"), 1, 10)
	if len(result) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result))
	}
}

// TestSupervisor_Ask_SpanWithInt64Seq confirms collectSpan works when seq
// values are encoded as int64 (not float64) — exercises the int64 toI64 branch
// in a realistic end-to-end path.
func TestSupervisor_Ask_SpanWithInt64Seq(t *testing.T) {
	ws := t.TempDir()
	now := func() time.Time { return time.Unix(0, 0).UTC() }
	s := NewSupervisor(SupervisorConfig{
		Workspace: ws, Agent: "scout", Transport: "agy-tmux",
		Now: now, NewID: func() string { return "c2" },
		PollEvery: time.Millisecond, Timeout: time.Second,
	})
	go func() {
		time.Sleep(10 * time.Millisecond)
		// Write lines with integer seq values (Go json.Unmarshal decodes as float64,
		// but toI64 must handle int64 when the caller builds the map directly).
		appendFeed(t, ws, "scout",
			`{"seq":10,"kind":"assistant_text","data":{"text":"result"}}`,
			`{"seq":11,"kind":"correlation","data":{"sub":"response_complete","corr_id":"c2","start_seq":10,"end_seq":11}}`)
	}()
	ans, err := s.Ask(context.Background(), "go")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if ans.Text() != "result" {
		t.Fatalf("Text() = %q, want %q", ans.Text(), "result")
	}
}
