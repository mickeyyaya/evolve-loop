package phasestream

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func baseClock() time.Time { return time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC) }

type testClock struct{ t time.Time }

func (c *testClock) now() time.Time { return c.t }

func appendLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	for _, ln := range lines {
		if _, err := f.WriteString(ln + "\n"); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

// appendRaw appends exactly s (no trailing newline added) — used to model a
// streaming writer that has flushed only a partial line so far.
func appendRaw(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// failWriter always fails — used to verify sink-write errors propagate.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("sink full") }

func newTestNormalizer(dir string, sink *bytes.Buffer, clk *testClock) *Normalizer {
	return NewNormalizer(NormalizerConfig{
		Source:     Source{Producer: "normalizer", CLI: "claude-p", Cycle: 1, Phase: "tdd", Agent: "tdd"},
		TraceID:    "trace-1",
		StdoutPath: filepath.Join(dir, "tdd-stdout.log"),
		StderrPath: filepath.Join(dir, "tdd-stderr.log"),
		Sink:       sink,
		StallS:     600,
		Now:        clk.now,
	})
}

// 1. Offset tracking: a second Poll sees only bytes appended since the first.
func TestNormalizer_PollTailsOnlyNewBytes(t *testing.T) {
	dir := t.TempDir()
	clk := &testClock{t: baseClock()}
	var sink bytes.Buffer
	n := newTestNormalizer(dir, &sink, clk)
	stdout := filepath.Join(dir, "tdd-stdout.log")

	appendLines(t, stdout, `{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]}}`)
	got1, err := n.Poll()
	if err != nil {
		t.Fatalf("poll1: %v", err)
	}
	if len(got1) != 1 {
		t.Fatalf("poll1 want 1 envelope, got %d", len(got1))
	}
	if got1[0].Data["text"] != "hello" {
		t.Fatalf("poll1 text = %v want hello", got1[0].Data["text"])
	}

	appendLines(t, stdout, `{"type":"assistant","message":{"content":[{"type":"text","text":"world"}]}}`)
	got2, err := n.Poll()
	if err != nil {
		t.Fatalf("poll2: %v", err)
	}
	if len(got2) != 1 {
		t.Fatalf("poll2 should see only the newly-appended line, got %d", len(got2))
	}
	if got2[0].Data["text"] != "world" {
		t.Fatalf("poll2 text = %v want world", got2[0].Data["text"])
	}
}

// 2. A burst of stream_event deltas in one poll coalesces to a single progress tick.
func TestNormalizer_PollCoalescesStreamEvents(t *testing.T) {
	dir := t.TempDir()
	clk := &testClock{t: baseClock()}
	var sink bytes.Buffer
	n := newTestNormalizer(dir, &sink, clk)
	stdout := filepath.Join(dir, "tdd-stdout.log")

	appendLines(t, stdout,
		`{"type":"stream_event","event":{"type":"content_block_delta"}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta"}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta"}}`,
	)
	got, err := n.Poll()
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 coalesced progress envelope, got %d", len(got))
	}
	if got[0].Kind != KindProgress {
		t.Fatalf("kind = %s want progress", got[0].Kind)
	}
	if dc := got[0].Data["delta_count"]; dc != int64(3) {
		t.Fatalf("delta_count = %v want 3", dc)
	}
}

// 3. A stderr line carrying an infra marker becomes one infra_failure incident.
func TestNormalizer_PollEmitsInfraFailureFromStderr(t *testing.T) {
	dir := t.TempDir()
	clk := &testClock{t: baseClock()}
	var sink bytes.Buffer
	n := newTestNormalizer(dir, &sink, clk)
	stderr := filepath.Join(dir, "tdd-stderr.log")

	appendLines(t, stderr, `dial tcp 1.2.3.4:443: connection refused`)
	got, err := n.Poll()
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 infra_failure envelope, got %d", len(got))
	}
	if got[0].Kind != KindInfraFailure {
		t.Fatalf("kind = %s want infra_failure", got[0].Kind)
	}
	if got[0].Severity != SeverityIncident {
		t.Fatalf("severity = %s want INCIDENT", got[0].Severity)
	}
	if got[0].Data["marker"] != "conn_refused" {
		t.Fatalf("marker = %v want conn_refused", got[0].Data["marker"])
	}
}

// 4. With no output, once the clock crosses StallS a single stall incident fires; it does not re-fire.
func TestNormalizer_PollEmitsStallOnceAfterThreshold(t *testing.T) {
	dir := t.TempDir()
	clk := &testClock{t: baseClock()}
	var sink bytes.Buffer
	n := newTestNormalizer(dir, &sink, clk)

	clk.t = clk.t.Add(601 * time.Second)
	got, err := n.Poll()
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 stall envelope, got %d", len(got))
	}
	if got[0].Kind != KindStall {
		t.Fatalf("kind = %s want stall", got[0].Kind)
	}
	if got[0].Severity != SeverityIncident {
		t.Fatalf("severity = %s want INCIDENT", got[0].Severity)
	}

	clk.t = clk.t.Add(601 * time.Second)
	got2, err := n.Poll()
	if err != nil {
		t.Fatalf("poll2: %v", err)
	}
	if len(got2) != 0 {
		t.Fatalf("stall must fire once; poll2 emitted %d", len(got2))
	}
}

// 5. Every envelope written to the sink carries a gap-free monotonic seq across kinds.
func TestNormalizer_SinkSeqMonotonicGapFree(t *testing.T) {
	dir := t.TempDir()
	clk := &testClock{t: baseClock()}
	var sink bytes.Buffer
	n := newTestNormalizer(dir, &sink, clk)
	stdout := filepath.Join(dir, "tdd-stdout.log")
	stderr := filepath.Join(dir, "tdd-stderr.log")

	appendLines(t, stdout,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"a"}]}}`,
		`{"type":"stream_event","event":{}}`,
	)
	appendLines(t, stderr, `connection refused`)
	if _, err := n.Poll(); err != nil {
		t.Fatalf("poll: %v", err)
	}

	dec := json.NewDecoder(bytes.NewReader(sink.Bytes()))
	var prev int64
	count := 0
	for dec.More() {
		var e Envelope
		if err := dec.Decode(&e); err != nil {
			t.Fatalf("decode sink envelope: %v", err)
		}
		count++
		if e.Seq != prev+1 {
			t.Fatalf("seq gap: got %d after %d", e.Seq, prev)
		}
		prev = e.Seq
	}
	if count != 3 {
		t.Fatalf("want exactly 3 envelopes in sink (assistant_text, infra_failure, progress), got %d", count)
	}
}

// 6. Stall + Enforce + a pgid kills the process group via the injected seam.
func TestNormalizer_StallEnforceKillsPgrp(t *testing.T) {
	dir := t.TempDir()
	clk := &testClock{t: baseClock()}
	var sink bytes.Buffer
	var gotPGID int
	var gotSig syscall.Signal
	called := false
	n := NewNormalizer(NormalizerConfig{
		Source:     Source{Producer: "normalizer", Cycle: 1, Phase: "tdd", Agent: "tdd"},
		TraceID:    "t",
		StdoutPath: filepath.Join(dir, "tdd-stdout.log"),
		Sink:       &sink,
		StallS:     600,
		Enforce:    true,
		PGID:       4242,
		Now:        clk.now,
		KillPgrp: func(pgid int, sig syscall.Signal) error {
			gotPGID, gotSig, called = pgid, sig, true
			return nil
		},
	})

	clk.t = clk.t.Add(601 * time.Second)
	got, err := n.Poll()
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if !called {
		t.Fatalf("expected KillPgrp called on stall+enforce")
	}
	if gotPGID != 4242 || gotSig != syscall.SIGTERM {
		t.Fatalf("kill args: pgid=%d sig=%v want 4242/SIGTERM", gotPGID, gotSig)
	}
	// H2: the kill outcome must be surfaced on the stall envelope, not swallowed.
	if len(got) != 1 || got[0].Data["kill_result"] != "ok" {
		t.Fatalf("stall envelope should carry kill_result=ok, got %+v", got)
	}
}

// H1: a partial (no-newline) line must not be consumed until it is completed;
// the offset must not advance past it (else the line is split/corrupted).
func TestNormalizer_PollHoldsPartialLineUntilComplete(t *testing.T) {
	dir := t.TempDir()
	clk := &testClock{t: baseClock()}
	var sink bytes.Buffer
	n := newTestNormalizer(dir, &sink, clk)
	stdout := filepath.Join(dir, "tdd-stdout.log")

	// A genuinely partial JSON line (streaming writer mid-flush, no newline yet).
	appendRaw(t, stdout, `{"type":"result","total_cost`)
	got1, err := n.Poll()
	if err != nil {
		t.Fatalf("poll1: %v", err)
	}
	if len(got1) != 0 {
		t.Fatalf("partial line must not be consumed yet, got %d envelopes", len(got1))
	}

	// Complete the line.
	appendRaw(t, stdout, `_usd":0.5,"usage":{}}`+"\n")
	got2, err := n.Poll()
	if err != nil {
		t.Fatalf("poll2: %v", err)
	}
	if len(got2) != 1 {
		t.Fatalf("completed line should yield exactly 1 envelope, got %d", len(got2))
	}
	if got2[0].Kind != KindResult {
		t.Fatalf("kind = %s want result (line must not have been split)", got2[0].Kind)
	}
}

// M4: a shrunk file (truncation/rotation) resets the offset and re-reads.
func TestNormalizer_PollHandlesRotation(t *testing.T) {
	dir := t.TempDir()
	clk := &testClock{t: baseClock()}
	var sink bytes.Buffer
	n := newTestNormalizer(dir, &sink, clk)
	stdout := filepath.Join(dir, "tdd-stdout.log")

	appendLines(t, stdout, `{"type":"assistant","message":{"content":[{"type":"text","text":"a fairly long first line of output"}]}}`)
	if got1, err := n.Poll(); err != nil || len(got1) != 1 {
		t.Fatalf("poll1: err=%v len=%d", err, len(got1))
	}

	// Rotation: replace with a much shorter file (size drops below the offset).
	if err := os.WriteFile(stdout, []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"x"}]}}`+"\n"), 0o644); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	got2, err := n.Poll()
	if err != nil {
		t.Fatalf("poll2: %v", err)
	}
	if len(got2) != 1 {
		t.Fatalf("after rotation want 1 envelope, got %d", len(got2))
	}
	if got2[0].Data["text"] != "x" {
		t.Fatalf("after rotation text = %v want x", got2[0].Data["text"])
	}
}

// M5: a sink write failure propagates, and envelopes built before the failure
// are still returned for the caller to inspect.
func TestNormalizer_PollPropagatesSinkWriteError(t *testing.T) {
	dir := t.TempDir()
	clk := &testClock{t: baseClock()}
	n := NewNormalizer(NormalizerConfig{
		Source:     Source{Producer: "normalizer", Cycle: 1, Phase: "tdd", Agent: "tdd"},
		TraceID:    "t",
		StdoutPath: filepath.Join(dir, "tdd-stdout.log"),
		Sink:       failWriter{},
		StallS:     600,
		Now:        clk.now,
	})
	stdout := filepath.Join(dir, "tdd-stdout.log")
	appendLines(t, stdout, `{"type":"assistant","message":{"content":[{"type":"text","text":"x"}]}}`)

	got, err := n.Poll()
	if err == nil {
		t.Fatalf("expected sink write error to propagate")
	}
	if len(got) == 0 {
		t.Fatalf("envelopes produced before the sink failure should still be returned")
	}
}
