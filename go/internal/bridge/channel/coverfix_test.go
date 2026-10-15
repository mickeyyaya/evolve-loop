package channel

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type firstWriteSignal struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	once    sync.Once
	written chan struct{}
}

func (w *firstWriteSignal) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	w.once.Do(func() { close(w.written) })
	return n, err
}

func (w *firstWriteSignal) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func TestCoverfixProducerWarnsWhenTheFeedSinkRejectsWrites(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-stdout.log"),
		[]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"working"}]}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	saved := openFeed
	t.Cleanup(func() { openFeed = saved })
	openFeed = func(name string, _ int, perm os.FileMode) (*os.File, error) {
		if err := os.WriteFile(name, nil, perm); err != nil {
			return nil, err
		}
		return os.Open(name)
	}
	warn := &firstWriteSignal{written: make(chan struct{})}
	p := NewProducer(ProducerConfig{Workspace: ws, Agent: "build", Phase: "build", Cycle: 1,
		PollEvery: time.Millisecond, Warn: warn, Now: func() time.Time { return time.Unix(0, 0).UTC() }})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()
	select {
	case <-warn.written:
	case <-time.After(10 * time.Second):
		t.Fatal("producer never reported the sink failure")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run must absorb sink errors, got %v", err)
	}
	if !strings.Contains(warn.String(), "[channel] WARN poll:") {
		t.Fatalf("poll failure not surfaced, warn = %q", warn.String())
	}
}

func TestCoverfixAwaitReplyTimesOutBeforeFirstFeedRead(t *testing.T) {
	base := time.Unix(1000, 0)
	calls := 0
	now := func() time.Time {
		calls++
		return base.Add(time.Duration(calls) * time.Second)
	}
	s := NewSupervisor(SupervisorConfig{
		Workspace: t.TempDir(), Agent: "build", Transport: "claude-tmux",
		Now: now, NewID: func() string { return "c1" },
		PollEvery: time.Hour, Timeout: time.Nanosecond,
	})
	if _, err := s.Ask(context.Background(), "x"); !errors.Is(err, ErrResponseTimeout) {
		t.Fatalf("err = %v, want ErrResponseTimeout", err)
	}
}

func TestCoverfixAwaitReplyStopsWhenCancelledWhileWaitingForTheNextPoll(t *testing.T) {
	s := NewSupervisor(SupervisorConfig{
		Workspace: t.TempDir(), Agent: "build", Transport: "claude-tmux",
		NewID:     func() string { return "c1" },
		PollEvery: time.Hour, Timeout: time.Hour,
	})
	ctx, cancel := context.WithCancel(context.Background())
	timer := time.AfterFunc(30*time.Millisecond, cancel)
	defer timer.Stop()
	if _, err := s.Ask(ctx, "x"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestCoverfixTailLinesKeepsOffsetWhenStatOrSeekFail(t *testing.T) {
	path := FeedPath(t.TempDir(), "build")
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	saved := tailHooks
	t.Cleanup(func() { tailHooks = saved })

	tailHooks.stat = func(*os.File) (os.FileInfo, error) { return nil, errors.New("stat boom") }
	if lines, off := tailLines(path, 1); lines != nil || off != 1 {
		t.Errorf("stat failure: got (%v,%d), want (nil,1)", lines, off)
	}

	tailHooks = saved
	tailHooks.seek = func(*os.File, int64, int) (int64, error) { return 0, errors.New("seek boom") }
	if lines, off := tailLines(path, 1); lines != nil || off != 1 {
		t.Errorf("seek failure: got (%v,%d), want (nil,1)", lines, off)
	}
}

func TestCoverfixCollectSpanWarnsAndKeepsEventsBeforeAnOversizedLine(t *testing.T) {
	ws := t.TempDir()
	appendFeed(t, ws, "build",
		`{"seq":2,"kind":"assistant_text","data":{"text":"ok"}}`,
		strings.Repeat("x", 11*1024*1024),
	)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	savedErr := os.Stderr
	os.Stderr = w
	result := collectSpan(FeedPath(ws, "build"), 1, 10)
	os.Stderr = savedErr
	_ = w.Close()
	out, _ := io.ReadAll(r)
	if len(result) != 1 {
		t.Fatalf("events before the oversized line must survive, got %d", len(result))
	}
	if !strings.Contains(string(out), "WARN collectSpan scan") {
		t.Errorf("scan failure not surfaced, stderr = %q", out)
	}
}
