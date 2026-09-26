package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
)

// lockedBuffer lets the test poll String() while the follow loop writes from
// its own goroutine without a data race.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func TestRenderFeedLine_Correlation(t *testing.T) {
	e := map[string]any{
		"kind": "correlation",
		"data": map[string]any{"sub": "request", "corr_id": "c1"},
	}
	got := renderFeedLine(e)
	if !strings.Contains(got, "correlation") || !strings.Contains(got, "request") || !strings.Contains(got, "c1") {
		t.Fatalf("render = %q; want to contain 'correlation', 'request', 'c1'", got)
	}
	if !strings.Contains(got, "corr_id=c1") {
		t.Fatalf("render = %q; want corr_id=c1", got)
	}
}

func TestRenderFeedLine_CorrelationMissingData(t *testing.T) {
	e := map[string]any{"kind": "correlation"}
	got := renderFeedLine(e)
	if !strings.Contains(got, "correlation") {
		t.Fatalf("render = %q; want 'correlation'", got)
	}
}

func TestRenderFeedLine_CorrelationWrongDataType(t *testing.T) {
	e := map[string]any{"kind": "correlation", "data": "not-a-map"}
	got := renderFeedLine(e)
	if !strings.Contains(got, "correlation") {
		t.Fatalf("render = %q; want 'correlation'", got)
	}
}

func TestRenderFeedLine_AssistantText(t *testing.T) {
	e := map[string]any{
		"seq":  float64(1),
		"kind": "assistant_text",
		"data": map[string]any{"text": "hello world"},
	}
	got := renderFeedLine(e)
	if !strings.Contains(got, "assistant_text") || !strings.Contains(got, "hello") {
		t.Fatalf("render = %q; want assistant_text and 'hello'", got)
	}
	if !strings.Contains(got, "seq=1") {
		t.Fatalf("render = %q; want seq=1", got)
	}
}

func TestRenderFeedLine_AssistantTextTruncation(t *testing.T) {
	longText := strings.Repeat("x", 200)
	e := map[string]any{
		"kind": "assistant_text",
		"data": map[string]any{"text": longText},
	}
	got := renderFeedLine(e)
	if len(got) > 200 {
		t.Fatalf("render not truncated: len=%d, got=%q", len(got), got[:50])
	}
}

func TestRenderFeedLine_NoData(t *testing.T) {
	e := map[string]any{"kind": "tool_use"}
	got := renderFeedLine(e)
	if !strings.Contains(got, "tool_use") {
		t.Fatalf("render = %q; want 'tool_use'", got)
	}
}

func TestRenderFeedLine_SeqPresent(t *testing.T) {
	e := map[string]any{"seq": float64(42), "kind": "ping"}
	got := renderFeedLine(e)
	if !strings.Contains(got, "seq=42") {
		t.Fatalf("render = %q; want seq=42", got)
	}
}

func TestRunBridgeWatchOnce_RendersFeed(t *testing.T) {
	ws := t.TempDir()
	feedContent := `{"seq":1,"kind":"assistant_text","data":{"text":"hello"}}` + "\n" +
		`{"seq":2,"kind":"correlation","data":{"sub":"request","corr_id":"c1","at_seq":2}}` + "\n"
	if err := os.WriteFile(channel.FeedPath(ws, "build"), []byte(feedContent), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	var out bytes.Buffer
	if err := runBridgeWatchOnce(&out, ws, "build"); err != nil {
		t.Fatalf("watch: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "assistant_text") || !strings.Contains(s, "hello") {
		t.Fatalf("missing assistant_text/hello in:\n%s", s)
	}
	if !strings.Contains(s, "request") || !strings.Contains(s, "corr_id=c1") {
		t.Fatalf("missing 'request corr_id=c1' in:\n%s", s)
	}
}

func TestRunBridgeWatchOnce_MissingFeedNoError(t *testing.T) {
	if err := runBridgeWatchOnce(&bytes.Buffer{}, t.TempDir(), "build"); err != nil {
		t.Fatalf("missing feed should return nil err, got %v", err)
	}
}

func TestRunBridgeWatchOnce_MalformedLinesSkipped(t *testing.T) {
	ws := t.TempDir()
	feedContent := "not-json\n" +
		`{"seq":1,"kind":"ping"}` + "\n" +
		"{broken\n"
	if err := os.WriteFile(channel.FeedPath(ws, "agent"), []byte(feedContent), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	var out bytes.Buffer
	if err := runBridgeWatchOnce(&out, ws, "agent"); err != nil {
		t.Fatalf("malformed lines should not error: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "ping") {
		t.Fatalf("valid line 'ping' missing in:\n%s", s)
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 rendered line, got %d:\n%s", len(lines), s)
	}
}

func TestRunBridgeWatchOnce_EmptyFeedNoOutput(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(channel.FeedPath(ws, "scout"), []byte(""), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	var out bytes.Buffer
	if err := runBridgeWatchOnce(&out, ws, "scout"); err != nil {
		t.Fatalf("empty feed: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("empty feed should produce no output, got %q", out.String())
	}
}

func TestRunBridgeWatchOnce_ReadError(t *testing.T) {
	// A directory at the feed path is a read error other than ErrNotExist.
	ws := t.TempDir()
	feedPath := channel.FeedPath(ws, "build")
	if err := os.MkdirAll(feedPath, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	err := runBridgeWatchOnce(&bytes.Buffer{}, ws, "build")
	if err == nil {
		t.Fatal("expected error when feed path is a directory, got nil")
	}
}

func TestRenderFeedLine_SeqIntType(t *testing.T) {
	// JSON decodes numbers as float64; an int seq comes only from in-process callers.
	e := map[string]any{"seq": 7, "kind": "ping"}
	got := renderFeedLine(e)
	if !strings.Contains(got, "seq=7") {
		t.Fatalf("render = %q; want seq=7", got)
	}
}

func TestRunBridge_Watch_FeedReadError(t *testing.T) {
	ws := t.TempDir()
	feedPath := channel.FeedPath(ws, "build")
	if err := os.MkdirAll(feedPath, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	var out, errb bytes.Buffer
	code := runBridge([]string{"watch", "--workspace=" + ws, "--agent=build"}, nil, &out, &errb)
	if code != 1 {
		t.Fatalf("expected exit 1 on feed read error, got %d; stderr=%q", code, errb.String())
	}
}

func TestRunBridge_Watch_MissingWorkspace(t *testing.T) {
	var out, errb bytes.Buffer
	code := runBridge([]string{"watch", "--agent=build"}, nil, &out, &errb)
	if code != 10 {
		t.Fatalf("missing --workspace: exit=%d, want 10; stderr=%q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "workspace") {
		t.Errorf("stderr should mention workspace; got %q", errb.String())
	}
}

func TestRunBridge_Watch_MissingAgent(t *testing.T) {
	var out, errb bytes.Buffer
	code := runBridge([]string{"watch", "--workspace=/tmp/x"}, nil, &out, &errb)
	if code != 10 {
		t.Fatalf("missing --agent: exit=%d, want 10; stderr=%q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "agent") {
		t.Errorf("stderr should mention agent; got %q", errb.String())
	}
}

func TestRunBridge_Watch_NoFollowRendersAndReturns(t *testing.T) {
	ws := t.TempDir()
	feedContent := `{"seq":1,"kind":"ping","data":{}}` + "\n"
	if err := os.WriteFile(channel.FeedPath(ws, "build"), []byte(feedContent), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	var out, errb bytes.Buffer
	code := runBridge([]string{"watch", "--workspace=" + ws, "--agent=build"}, nil, &out, &errb)
	if code != 0 {
		t.Fatalf("watch exit=%d, want 0; stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), "ping") {
		t.Fatalf("output missing 'ping': %q", out.String())
	}
}

func TestRunBridge_Watch_HelpFlag(t *testing.T) {
	var out, errb bytes.Buffer
	code := runBridge([]string{"watch", "--help"}, nil, &out, &errb)
	if code != 0 {
		t.Fatalf("--help exit=%d, want 0", code)
	}
	if !strings.Contains(out.String(), "workspace") {
		t.Errorf("help should mention workspace; got %q", out.String())
	}
}

func TestRunBridge_Watch_UnknownFlag(t *testing.T) {
	var out, errb bytes.Buffer
	code := runBridge([]string{"watch", "--workspace=/tmp/x", "--agent=b", "--bogus=x"}, nil, &out, &errb)
	if code != 10 {
		t.Fatalf("unknown flag: exit=%d, want 10", code)
	}
}

func TestRenderFeedLine_MissingKind(t *testing.T) {
	if got := renderFeedLine(map[string]any{}); !strings.Contains(got, "unknown") {
		t.Fatalf("render = %q; want 'unknown'", got)
	}
}

func TestRunBridgeWatchFollow_CancelledCtxExits(t *testing.T) {
	ws := t.TempDir()
	os.WriteFile(channel.FeedPath(ws, "scout"), []byte(`{"seq":3,"kind":"tick_event"}`+"\n"), 0o644)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errb bytes.Buffer
	if code := runBridgeWatchFollow(ctx, &out, &errb, ws, "scout"); code != 0 {
		t.Fatalf("exit=%d want 0", code)
	}
}

func TestRunBridgeWatchFollow_TailsNewLines(t *testing.T) {
	orig := watchFollowInterval
	watchFollowInterval = 20 * time.Millisecond
	t.Cleanup(func() { watchFollowInterval = orig })

	ws := t.TempDir()
	path := channel.FeedPath(ws, "scout")
	os.WriteFile(path, []byte(`{"seq":1,"kind":"assistant_text","data":{"text":"first"}}`+"\n"), 0o644)
	// The loop seeds its offset asynchronously and skips an earlier append, so
	// the test re-appends each tick and waits on the rendered line.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out := &lockedBuffer{}
	var errb bytes.Buffer
	done := make(chan int, 1)
	go func() { done <- runBridgeWatchFollow(ctx, out, &errb, ws, "scout") }()

	deadline := time.Now().Add(20 * time.Second)
	for !strings.Contains(out.String(), "second") && time.Now().Before(deadline) {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatalf("open feed for append: %v", err)
		}
		f.WriteString(`{"seq":2,"kind":"assistant_text","data":{"text":"second"}}` + "\n")
		f.Close()
		time.Sleep(watchFollowInterval)
	}
	cancel()
	code := <-done
	if code != 0 {
		t.Fatalf("exit=%d want 0", code)
	}
	if !strings.Contains(out.String(), "second") {
		t.Fatalf("expected tailed line in output:\n%s", out.String())
	}
}

func TestRunBridgeWatchFollow_NoGrowthNoOutput(t *testing.T) {
	orig := watchFollowInterval
	watchFollowInterval = 20 * time.Millisecond
	t.Cleanup(func() { watchFollowInterval = orig })

	ws := t.TempDir()
	os.WriteFile(channel.FeedPath(ws, "scout"), []byte(`{"seq":1,"kind":"ping"}`+"\n"), 0o644)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	var out, errb bytes.Buffer
	if code := runBridgeWatchFollow(ctx, &out, &errb, ws, "scout"); code != 0 {
		t.Fatalf("exit=%d want 0", code)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no output for non-growing feed, got: %q", out.String())
	}
}

func TestRunBridgeWatchFollow_MissingFeedContinues(t *testing.T) {
	orig := watchFollowInterval
	watchFollowInterval = 20 * time.Millisecond
	t.Cleanup(func() { watchFollowInterval = orig })

	ws := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	var out, errb bytes.Buffer
	if code := runBridgeWatchFollow(ctx, &out, &errb, ws, "scout"); code != 0 {
		t.Fatalf("exit=%d want 0 (ErrNotExist must not be fatal)", code)
	}
}

func TestRunBridgeWatchFollow_SkipsMalformedAndEmptyLines(t *testing.T) {
	orig := watchFollowInterval
	watchFollowInterval = 20 * time.Millisecond
	t.Cleanup(func() { watchFollowInterval = orig })

	ws := t.TempDir()
	path := channel.FeedPath(ws, "scout")
	// One byte makes the seeded offset non-zero, which forces a seek.
	os.WriteFile(path, []byte("\n"), 0o644)
	// The loop seeds its offset asynchronously and skips an earlier append, so
	// the test re-appends each tick and waits on the rendered line.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out := &lockedBuffer{}
	var errb bytes.Buffer
	done := make(chan int, 1)
	go func() { done <- runBridgeWatchFollow(ctx, out, &errb, ws, "scout") }()

	deadline := time.Now().Add(20 * time.Second)
	for !strings.Contains(out.String(), "ping") && time.Now().Before(deadline) {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatalf("open feed for append: %v", err)
		}
		f.WriteString("\nnot-json\n" + `{"seq":5,"kind":"ping"}` + "\n")
		f.Close()
		time.Sleep(watchFollowInterval)
	}
	cancel()
	code := <-done
	if code != 0 {
		t.Fatalf("exit=%d want 0", code)
	}
	if !strings.Contains(out.String(), "ping") {
		t.Fatalf("expected 'ping' in output, got: %q", out.String())
	}
	if strings.Contains(out.String(), "not-json") {
		t.Fatalf("malformed line leaked into output: %q", out.String())
	}
}

func TestCmdBridgeWatch_FollowFlagParsed(t *testing.T) {
	orig := watchFollowInterval
	watchFollowInterval = 20 * time.Millisecond
	t.Cleanup(func() { watchFollowInterval = orig })

	ws := t.TempDir()
	// A SIGINT to this process is the only way to end the follow loop that
	// cmdBridgeWatch wires to the signal context.
	go func() {
		time.Sleep(60 * time.Millisecond)
		syscall.Kill(syscall.Getpid(), syscall.SIGINT) //nolint:errcheck
	}()
	var out, errb bytes.Buffer
	code := cmdBridgeWatch([]string{"--workspace=" + ws, "--agent=scout", "--follow"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit=%d want 0 after SIGINT", code)
	}
}
