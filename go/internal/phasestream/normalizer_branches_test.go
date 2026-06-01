package phasestream

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestNormalizer_NewNormalizer_DefaultsKillPgrp pins the NewNormalizer
// nil-KillPgrp fallback: a normalizer built without an injected kill seam
// still polls cleanly (the default kill fn is wired but not invoked when
// enforce/pgid are unset).
func TestNormalizer_NewNormalizer_DefaultsKillPgrp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	clk := &testClock{t: baseClock()}
	var sink bytes.Buffer
	n := NewNormalizer(NormalizerConfig{
		Source:     Source{Producer: "normalizer", Cycle: 1, Phase: "tdd", Agent: "tdd"},
		TraceID:    "t",
		StdoutPath: filepath.Join(dir, "tdd-stdout.log"),
		Sink:       &sink,
		StallS:     0, // no stall rule → default KillPgrp never invoked, only constructed
		Now:        clk.now,
		// KillPgrp intentionally omitted to exercise the nil-default branch.
	})
	stdout := filepath.Join(dir, "tdd-stdout.log")
	appendLines(t, stdout, `{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`)
	got, err := n.Poll()
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(got) != 1 || got[0].Data["text"] != "hi" {
		t.Fatalf("want 1 assistant_text 'hi', got %#v", got)
	}
}

// TestNormalizer_DefaultKillPgrp_RunsOnStallEnforce pins the body of the
// default KillPgrp closure: with no injected kill seam but stall+enforce
// armed, the real syscall.Kill runs against a non-existent process group,
// which returns ESRCH — surfaced on the stall envelope's kill_result rather
// than swallowed. No real process is signaled (the group does not exist).
func TestNormalizer_DefaultKillPgrp_RunsOnStallEnforce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	clk := &testClock{t: baseClock()}
	var sink bytes.Buffer
	n := NewNormalizer(NormalizerConfig{
		Source:     Source{Producer: "normalizer", Cycle: 1, Phase: "tdd", Agent: "tdd"},
		TraceID:    "t",
		StdoutPath: filepath.Join(dir, "tdd-stdout.log"),
		Sink:       &sink,
		StallS:     600,
		Enforce:    true,
		// A process group that does not exist → syscall.Kill returns ESRCH.
		PGID: 0x3FFFFFFF,
		Now:  clk.now,
		// KillPgrp omitted → exercises the real default closure body.
	})
	clk.t = clk.t.Add(601 * time.Second)
	got, err := n.Poll()
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(got) != 1 || got[0].Kind != KindStall {
		t.Fatalf("want 1 stall envelope, got %#v", got)
	}
	kr, ok := got[0].Data["kill_result"].(string)
	if !ok || kr == "" {
		t.Fatalf("stall envelope must carry a kill_result string, got %#v", got[0].Data["kill_result"])
	}
	// The default kill ran and returned an error (no such process group) —
	// it must NOT report "ok" for a non-existent group.
	if kr == "ok" {
		t.Errorf("killing a non-existent process group should surface an error, got kill_result=ok")
	}
}

// TestTailFile_StatOKButUnreadable pins the tailFile os.Open-error leg: a file
// that stats fine but cannot be opened (mode 000) is a no-op that leaves the
// offset unchanged. Skips as root, where 000 doesn't block reads.
func TestTailFile_StatOKButUnreadable(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("running as root — chmod 000 doesn't block reads")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "blocked.log")
	if err := os.WriteFile(path, []byte("line one\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(path, 0o644)

	lines, off := tailFile(path, 0)
	if lines != nil {
		t.Errorf("unreadable file should yield no lines, got %#v", lines)
	}
	if off != 0 {
		t.Errorf("offset must stay at 0 when the file can't be opened, got %d", off)
	}
}

// TestNormalizer_Poll_NilSinkSkipsWrite pins the Sink==nil branch of Poll:
// with no sink the envelopes are still classified and returned, and no write
// is attempted (no panic, no error).
func TestNormalizer_Poll_NilSinkSkipsWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	clk := &testClock{t: baseClock()}
	n := NewNormalizer(NormalizerConfig{
		Source:     Source{Producer: "normalizer", Cycle: 1, Phase: "tdd", Agent: "tdd"},
		TraceID:    "t",
		StdoutPath: filepath.Join(dir, "tdd-stdout.log"),
		Sink:       nil, // no sink
		Now:        clk.now,
	})
	stdout := filepath.Join(dir, "tdd-stdout.log")
	appendLines(t, stdout, `{"type":"assistant","message":{"content":[{"type":"text","text":"x"}]}}`)
	got, err := n.Poll()
	if err != nil {
		t.Fatalf("poll with nil sink should not error, got %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("envelopes should still be returned with a nil sink, got %d", len(got))
	}
}

// TestNormalizer_Poll_MissingStdoutNoOp pins the tailFile stat-error branch:
// a stdout path that does not exist yields no envelopes and no error (the
// offset stays put for a later poll once the file appears).
func TestNormalizer_Poll_MissingStdoutNoOp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	clk := &testClock{t: baseClock()}
	var sink bytes.Buffer
	n := NewNormalizer(NormalizerConfig{
		Source:     Source{Producer: "normalizer", Cycle: 1, Phase: "tdd", Agent: "tdd"},
		TraceID:    "t",
		StdoutPath: filepath.Join(dir, "does-not-exist.log"),
		Sink:       &sink,
		Now:        clk.now,
	})
	got, err := n.Poll()
	if err != nil {
		t.Fatalf("poll on a missing stdout file must be a no-op, got err %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("missing file should yield 0 envelopes, got %d", len(got))
	}
}

// TestWriteEnvelope_MarshalErrorPropagates pins the writeEnvelope
// marshal-error branch: an envelope whose Data holds an unmarshalable value
// (a channel) surfaces a marshal error rather than writing garbage.
func TestWriteEnvelope_MarshalErrorPropagates(t *testing.T) {
	t.Parallel()
	var sink bytes.Buffer
	bad := Envelope{
		SchemaVersion: SchemaVersion,
		Seq:           7,
		Data:          map[string]any{"ch": make(chan int)}, // channels can't be JSON-marshaled
	}
	err := writeEnvelope(&sink, bad)
	if err == nil {
		t.Fatalf("expected a marshal error for an unmarshalable envelope")
	}
	if !strings.Contains(err.Error(), "marshal envelope seq 7") {
		t.Errorf("marshal error should name the seq, got %q", err.Error())
	}
	if sink.Len() != 0 {
		t.Errorf("nothing should be written when marshaling fails, got %q", sink.String())
	}
}
