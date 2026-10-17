package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func fakeReader() reader {
	var n atomic.Int64
	return func() (Counters, error) {
		i := n.Add(1)
		return Counters{At: time.Unix(i, 0), HasCPU: true, CPUBusy: uint64(i) * 10, CPUTotal: uint64(i) * 20, HasMem: true, MemUsedMB: 1}, nil
	}
}

func readSamples(t *testing.T, path string) []Sample {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []Sample
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var s Sample
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			t.Fatalf("bad line %q: %v", line, err)
		}
		out = append(out, s)
	}
	return out
}

func TestRun_DurationBoundedRunWritesSamples(t *testing.T) {
	out := filepath.Join(t.TempDir(), "s.ndjson")
	var se bytes.Buffer
	code := run([]string{"-out", out, "-interval", "10ms", "-duration", "100ms"}, &se, fakeReader(), nil)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, se.String())
	}
	got := readSamples(t, out)
	if len(got) < 3 {
		t.Fatalf("samples=%d", len(got))
	}
	if got[0].CPUBusyPct == nil || *got[0].CPUBusyPct != 50 {
		t.Fatalf("pct=%v", got[0].CPUBusyPct)
	}
}

func TestRun_StopSignalEndsRunAndClosesFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "s.ndjson")
	stop := make(chan os.Signal, 1)
	go func() {
		time.Sleep(80 * time.Millisecond)
		stop <- syscall.SIGTERM
	}()
	if code := run([]string{"-out", out, "-interval", "10ms"}, io.Discard, fakeReader(), stop); code != 0 {
		t.Fatalf("code=%d", code)
	}
	if len(readSamples(t, out)) == 0 {
		t.Fatal("no samples")
	}
}

func TestRun_UsageErrors(t *testing.T) {
	for _, args := range [][]string{{}, {"-out", "x", "-interval", "0s"}, {"-bogus"}} {
		var se bytes.Buffer
		if code := run(args, &se, fakeReader(), nil); code != 2 {
			t.Errorf("args=%v code=%d", args, code)
		}
	}
}

func TestRun_OpenFailureExits1(t *testing.T) {
	var se bytes.Buffer
	missing := filepath.Join(t.TempDir(), "nodir", "s.ndjson")
	if code := run([]string{"-out", missing}, &se, fakeReader(), nil); code != 1 || !strings.Contains(se.String(), "hostsampler:") {
		t.Fatalf("code=%d stderr=%q", code, se.String())
	}
}

func TestRun_BaselineReadFailureExits1(t *testing.T) {
	bad := func() (Counters, error) { return Counters{}, errors.New("no source") }
	var se bytes.Buffer
	out := filepath.Join(t.TempDir(), "s.ndjson")
	if code := run([]string{"-out", out}, &se, bad, nil); code != 1 || !strings.Contains(se.String(), "no source") {
		t.Fatalf("code=%d stderr=%q", code, se.String())
	}
}

func TestRun_TickReadFailureIsReportedAndSamplingContinues(t *testing.T) {
	var n atomic.Int64
	flaky := func() (Counters, error) {
		i := n.Add(1)
		if i == 2 {
			return Counters{}, errors.New("transient")
		}
		return Counters{At: time.Unix(i, 0), HasCPU: true, CPUBusy: uint64(i), CPUTotal: uint64(i) * 2}, nil
	}
	out := filepath.Join(t.TempDir(), "s.ndjson")
	var se bytes.Buffer
	if code := run([]string{"-out", out, "-interval", "10ms", "-duration", "80ms"}, &se, flaky, nil); code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(se.String(), "transient") || len(readSamples(t, out)) == 0 {
		t.Fatalf("stderr=%q", se.String())
	}
}

type brokenSink struct{ writeErr, closeErr error }

func (b brokenSink) Write(p []byte) (int, error) { return len(p), b.writeErr }
func (b brokenSink) Close() error                { return b.closeErr }

func TestRun_WriteAndCloseFailuresExit1(t *testing.T) {
	old := openOut
	t.Cleanup(func() { openOut = old })
	openOut = func(string) (io.WriteCloser, error) { return brokenSink{writeErr: errors.New("disk full")}, nil }
	var se bytes.Buffer
	if code := run([]string{"-out", "x", "-interval", "5ms", "-duration", "50ms"}, &se, fakeReader(), nil); code != 1 || !strings.Contains(se.String(), "disk full") {
		t.Fatalf("write: code=%d stderr=%q", code, se.String())
	}
	openOut = func(string) (io.WriteCloser, error) { return brokenSink{closeErr: errors.New("close failed")}, nil }
	se.Reset()
	if code := run([]string{"-out", "x", "-interval", "5ms", "-duration", "30ms"}, &se, fakeReader(), nil); code != 1 || !strings.Contains(se.String(), "close failed") {
		t.Fatalf("close: code=%d stderr=%q", code, se.String())
	}
}

func TestStart_UnsupportedPlatformLogsAndExitsZero(t *testing.T) {
	var se bytes.Buffer
	code := start([]string{"-out", filepath.Join(t.TempDir(), "s.ndjson")}, &se, nil, unsupportedSource)
	if code != 0 {
		t.Fatalf("code=%d; the sampler must never fail a job", code)
	}
	if !strings.Contains(se.String(), "unsupported platform") {
		t.Fatalf("stderr=%q", se.String())
	}
}

func TestStart_SupportedSourceRunsTheLoop(t *testing.T) {
	out := filepath.Join(t.TempDir(), "s.ndjson")
	var se bytes.Buffer
	code := start([]string{"-out", out, "-interval", "10ms", "-duration", "60ms"}, &se, nil, func() (reader, error) { return fakeReader(), nil })
	if code != 0 || len(readSamples(t, out)) == 0 {
		t.Fatalf("code=%d stderr=%q", code, se.String())
	}
}

func TestRun_ConcurrentReaderSeesOnlyWholeLines(t *testing.T) {
	out := filepath.Join(t.TempDir(), "s.ndjson")
	done := make(chan int, 1)
	go func() {
		done <- run([]string{"-out", out, "-interval", "1ms", "-duration", "300ms"}, io.Discard, fakeReader(), nil)
	}()
	checked := 0
	for finished := false; !finished; {
		select {
		case <-done:
			finished = true
		default:
		}
		raw, _ := os.ReadFile(out)
		if len(raw) == 0 {
			continue
		}
		if raw[len(raw)-1] != '\n' {
			t.Fatalf("partial last line: %q", raw[max(0, len(raw)-40):])
		}
		for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
			var s Sample
			if err := json.Unmarshal([]byte(line), &s); err != nil {
				t.Fatalf("incomplete JSON line %q: %v", line, err)
			}
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("never observed the file mid-write")
	}
}
