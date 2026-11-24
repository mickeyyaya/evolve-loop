package main

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

const (
	oneDiskLine  = "     15.34   1812  27.13  16  19  66   2.50  2.20  2.10"
	twoDiskLine  = "  4.00  3  0.01   8.00  2  0.02   5   7  88   1.00  1.00  1.00"
	iostatHeader = "              disk0       cpu    load average\n    KB/t  tps  MB/s  us sy id   1m   5m   15m"
)

func TestParseIostatCPU(t *testing.T) {
	cases := []struct {
		name string
		line string
		want float64
		ok   bool
	}{
		{"one disk", oneDiskLine, 34, true},
		{"two disks", twoDiskLine, 12, true},
		{"no disk listed", "5 7 88 1.00 1.00 1.00", 12, true},
		{"no disk header", "us sy id 1m 5m 15m", 0, false},
		{"idle clamp", "1 1 1 0 0 101 1 1 1", 0, true},
		{"busy clamp", "1 1 1 0 0 -3 1 1 1", 100, true},
		{"header", iostatHeader, 0, false},
		{"blank", "   ", 0, false},
		{"too short", "1 2 3 4 5", 0, false},
	}
	for _, c := range cases {
		got, ok := parseIostatCPU(c.line)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("%s: got %v,%v want %v,%v", c.name, got, ok, c.want, c.ok)
		}
	}
}

type clock struct {
	nanos atomic.Int64
}

func newClock(t time.Time) *clock {
	c := &clock{}
	c.nanos.Store(t.UnixNano())
	return c
}

func (c *clock) now() time.Time {
	return time.Unix(0, c.nanos.Load())
}

func (c *clock) advance(d time.Duration) {
	c.nanos.Add(int64(d))
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestCPUFeed_BlockedStreamServesNoStaleValue(t *testing.T) {
	clk := newClock(time.Unix(100, 0))
	pr, pw := io.Pipe()
	feed := startCPUFeed(func(context.Context, string, ...string) (io.ReadCloser, error) { return pr, nil }, clk.now, io.Discard)
	if _, ok := feed.latest(clk.now()); ok {
		t.Fatal("no line yet must give no value")
	}
	if _, err := pw.Write([]byte(iostatHeader + "\n" + oneDiskLine + "\n")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { _, ok := feed.latest(clk.now()); return ok })
	if v, _ := feed.latest(clk.now()); v != 34 {
		t.Fatalf("v=%v", v)
	}
	clk.advance(cpuLineMaxAge)
	if _, ok := feed.latest(clk.now()); !ok {
		t.Fatal("a line exactly at the limit is still fresh")
	}
	clk.advance(time.Millisecond)
	if v, ok := feed.latest(clk.now()); ok {
		t.Fatalf("stale value %v served from a blocked stream", v)
	}
	_ = pw.Close()
}

func TestStartCPUFeed_OpenFailureGivesNoCPU(t *testing.T) {
	feed := startCPUFeed(func(context.Context, string, ...string) (io.ReadCloser, error) { return nil, errors.New("no iostat") }, time.Now, io.Discard)
	if _, ok := feed.latest(time.Now()); ok {
		t.Fatal("want no value")
	}
}

func TestExecStream_ReadsChildStdoutAndReapsOnClose(t *testing.T) {
	rc, err := execStream(context.Background(), "sh", "-c", "printf 'a\\nb\\n'")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	if string(got) != "a\nb\n" {
		t.Fatalf("got %q", got)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestExecStream_StartFailureReturnsError(t *testing.T) {
	if _, err := execStream(context.Background(), "/nonexistent/iostat"); err == nil {
		t.Fatal("want error")
	}
}

func TestRun_FeedFromIostatStream_EveryRecordHasCPUBusyInRange(t *testing.T) {
	feed := startCPUFeed(func(ctx context.Context, _ string, _ ...string) (io.ReadCloser, error) {
		return execStream(ctx, "sh", "-c", "printf '"+iostatHeader+"\\n'; while :; do printf '"+oneDiskLine+"\\n'; sleep 0.01; done")
	}, time.Now, io.Discard)
	waitFor(t, func() bool { _, ok := feed.latest(time.Now()); return ok })
	read := func() (Counters, error) {
		return readMacOS(fakeRunner(map[string]string{}, "vm_stat"), time.Now, feed.latest), nil
	}
	out := filepath.Join(t.TempDir(), "s.ndjson")
	if code := run([]string{"-out", out, "-interval", "20ms", "-duration", "200ms"}, io.Discard, read, nil); code != 0 {
		t.Fatalf("code=%d", code)
	}
	samples := readSamples(t, out)
	if len(samples) < 3 {
		t.Fatalf("samples=%d", len(samples))
	}
	for i, s := range samples {
		if s.CPUBusyPct == nil || *s.CPUBusyPct < 0 || *s.CPUBusyPct > 100 {
			t.Fatalf("record %d cpu_busy_pct=%v", i, s.CPUBusyPct)
		}
	}
}

func TestReadMacOS_CPUFromFeed(t *testing.T) {
	cpu := func(time.Time) (float64, bool) { return 42, true }
	got := readMacOS(fakeRunner(map[string]string{}, "vm_stat"), time.Now, cpu)
	if !got.HasCPUPct || got.CPUPct != 42 || got.HasCPU {
		t.Fatalf("got %+v", got)
	}
}

func TestCPUBusyPct_DirectPercentNeedsNoPreviousReading(t *testing.T) {
	got := cpuBusyPct(Counters{}, Counters{HasCPUPct: true, CPUPct: 37})
	if got == nil || *got != 37 {
		t.Fatalf("got %v", got)
	}
	if cpuBusyPct(Counters{}, Counters{}) != nil {
		t.Fatal("no CPU at all must stay nil")
	}
}

func TestStartCPUFeed_RunsOneIostatWithOneSecondInterval(t *testing.T) {
	var gotName string
	var gotArgs []string
	startCPUFeed(func(_ context.Context, name string, args ...string) (io.ReadCloser, error) {
		gotName, gotArgs = name, args
		return nil, errors.New("stop")
	}, time.Now, io.Discard)
	if gotName != "iostat" || strings.Join(gotArgs, " ") != "-w 1" {
		t.Fatalf("ran %q %v", gotName, gotArgs)
	}
}

func TestExecStream_CloseStopsARunningChild(t *testing.T) {
	rc, err := execStream(context.Background(), "sleep", "30")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- rc.Close() }()
	select {
	case err := <-done:
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("Close=%v, want the exit error of the killed child", err)
		}
		if ws, ok := exit.Sys().(syscall.WaitStatus); !ok || !ws.Signaled() || ws.Signal() != syscall.SIGTERM {
			t.Fatalf("child status=%v, want a kill by SIGTERM", exit.Sys())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Close left the child running")
	}
}

type syncBuf struct {
	mu   sync.Mutex
	data []byte
}

func (b *syncBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	b.data = append(b.data, p...)
	b.mu.Unlock()
	return len(p), nil
}

func (b *syncBuf) String() string {
	b.mu.Lock()
	text := string(b.data)
	b.mu.Unlock()
	return text
}

type endingStream struct {
	io.Reader
	closeErr error
}

func (e endingStream) Close() error { return e.closeErr }

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read boom") }

func diagOf(t *testing.T, stream io.ReadCloser) string {
	t.Helper()
	var diag syncBuf
	startCPUFeed(func(context.Context, string, ...string) (io.ReadCloser, error) { return stream, nil }, time.Now, &diag)
	waitFor(t, func() bool { return diag.String() != "" })
	return diag.String()
}

func TestCPUFeed_StreamEndNamesTheScanAndCloseErrors(t *testing.T) {
	got := diagOf(t, endingStream{io.MultiReader(strings.NewReader(oneDiskLine+"\n"), failingReader{}), errors.New("close boom")})
	want := "hostsampler: iostat stream ended: scan=read boom close=close boom\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCPUFeed_CleanStreamEndWritesOneLineWithNilErrors(t *testing.T) {
	got := diagOf(t, endingStream{strings.NewReader(oneDiskLine + "\n"), nil})
	want := "hostsampler: iostat stream ended: scan=<nil> close=<nil>\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStartCPUFeed_OpenFailureWritesOneLine(t *testing.T) {
	var diag syncBuf
	startCPUFeed(func(context.Context, string, ...string) (io.ReadCloser, error) { return nil, errors.New("no iostat") }, time.Now, &diag)
	if want := "hostsampler: iostat did not start: no iostat\n"; diag.String() != want {
		t.Fatalf("got %q want %q", diag.String(), want)
	}
}

func TestExecStream_CloseReportsTheChildExitStatus(t *testing.T) {
	rc, err := execStream(context.Background(), "sh", "-c", "exit 3")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(rc)
	var exit *exec.ExitError
	if err := rc.Close(); !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("Close=%v", err)
	}
}
