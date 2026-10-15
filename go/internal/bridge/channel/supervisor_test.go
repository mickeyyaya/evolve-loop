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

func TestSupervisor_DefaultNewID(t *testing.T) {
	s := NewSupervisor(SupervisorConfig{
		Workspace: t.TempDir(),
		Agent:     "build",
		Transport: "claude-tmux",
		PollEvery: time.Millisecond,
		Timeout:   5 * time.Millisecond,
	})
	_, err := s.Ask(context.Background(), "x")
	if !errors.Is(err, ErrResponseTimeout) {
		t.Fatalf("expected timeout, got %v", err)
	}
}

func TestAnswer_Text_NilData(t *testing.T) {
	a := Answer{Events: []map[string]any{
		{"seq": int64(1), "kind": "assistant_text"},
		{"seq": int64(2), "kind": "assistant_text", "data": map[string]any{"text": "hello"}},
	}}
	if got := a.Text(); got != "hello" {
		t.Fatalf("Text() = %q, want %q", got, "hello")
	}
}

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

func TestTailLines_MissingFile(t *testing.T) {
	lines, off := tailLines("/no/such/path/feed.ndjson", 0)
	if lines != nil || off != 0 {
		t.Fatalf("expected nil/0, got %v / %d", lines, off)
	}
}

func TestTailLines_Truncation(t *testing.T) {
	ws := t.TempDir()
	appendFeed(t, ws, "build", `{"seq":1,"kind":"assistant_text","data":{"text":"hi"}}`)
	path := FeedPath(ws, "build")
	fi, _ := os.Stat(path)
	size := fi.Size()
	lines, newOff := tailLines(path, size+100)
	if len(lines) == 0 {
		t.Fatal("expected lines after truncation reset, got none")
	}
	if newOff <= 0 {
		t.Fatalf("expected positive newOff, got %d", newOff)
	}
}

func TestCollectSpan_MissingFile(t *testing.T) {
	result := collectSpan("/no/such/path/feed.ndjson", 1, 10)
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

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

func TestSupervisor_Ask_InboxWriteError(t *testing.T) {
	ws := t.TempDir()
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

func TestSupervisor_Ask_TimeoutAfterWait(t *testing.T) {
	calls := 0
	base := time.Unix(1000, 0)
	now := func() time.Time {
		calls++
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

func TestCollectSpan_RecoversLineOver64KB(t *testing.T) {
	ws := t.TempDir()
	big := strings.Repeat("x", 100*1024)
	appendFeed(t, ws, "build",
		`{"seq":5,"kind":"assistant_text","data":{"text":"`+big+`"}}`,
	)
	result := collectSpan(FeedPath(ws, "build"), 1, 10)
	if len(result) != 1 {
		t.Fatalf("expected the >64KB line recovered (1 event), got %d", len(result))
	}
	data, _ := result[0]["data"].(map[string]any)
	if txt, _ := data["text"].(string); len(txt) != len(big) {
		t.Errorf("answer text truncated: got %d bytes, want %d", len(txt), len(big))
	}
}

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
