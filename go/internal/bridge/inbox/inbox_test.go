package inbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fixedNow returns a deterministic clock for TS minting assertions.
func fixedNow() time.Time {
	return time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
}

func TestPath(t *testing.T) {
	tests := []struct {
		name      string
		workspace string
		agent     string
		want      string
	}{
		{"named agent", "/ws", "build", "/ws/.bridge-inbox/build.ndjson"},
		{"empty agent defaults", "/ws", "", "/ws/.bridge-inbox/agent.ndjson"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Path(tt.workspace, tt.agent); got != tt.want {
				t.Errorf("Path(%q,%q) = %q, want %q", tt.workspace, tt.agent, got, tt.want)
			}
		})
	}
}

func TestAppendThenReadBack(t *testing.T) {
	ws := t.TempDir()
	if err := Append(ws, "build", Envelope{Kind: KindCommand, Body: "hello", Source: "cli"}, fixedNow); err != nil {
		t.Fatalf("Append: %v", err)
	}
	data, err := os.ReadFile(Path(ws, "build"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d: %q", len(lines), string(data))
	}
	var env Envelope
	if err := json.Unmarshal([]byte(lines[0]), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Kind != KindCommand || env.Body != "hello" || env.Source != "cli" {
		t.Errorf("round-trip mismatch: %+v", env)
	}
	if env.TS != "2026-05-26T12:00:00Z" {
		t.Errorf("TS not minted from now(): %q", env.TS)
	}
}

func TestAppendConcurrentAtomicity(t *testing.T) {
	ws := t.TempDir()
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_ = Append(ws, "build", Envelope{Kind: KindCommand, Body: strings.Repeat("x", 100), Source: "cli"}, fixedNow)
		}(i)
	}
	wg.Wait()
	data, err := os.ReadFile(Path(ws, "build"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != n {
		t.Fatalf("want %d lines, got %d (interleaving?)", n, len(lines))
	}
	for i, ln := range lines {
		var env Envelope
		if err := json.Unmarshal([]byte(ln), &env); err != nil {
			t.Fatalf("line %d not valid JSON (interleaved): %q", i, ln)
		}
	}
}

func TestAppendOversizedRejected(t *testing.T) {
	ws := t.TempDir()
	err := Append(ws, "build", Envelope{Kind: KindCommand, Body: strings.Repeat("y", 5000), Source: "cli"}, fixedNow)
	if err == nil {
		t.Fatal("want error for oversized envelope, got nil")
	}
}

func TestCursorDrainOnlyNew(t *testing.T) {
	ws := t.TempDir()
	c := NewCursor(ws, "build")

	// Missing file → no envelopes, no error.
	envs, err := c.Drain()
	if err != nil || len(envs) != 0 {
		t.Fatalf("drain on missing file: envs=%d err=%v", len(envs), err)
	}

	mustAppend(t, ws, "first")
	envs, err = c.Drain()
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if len(envs) != 1 || envs[0].Body != "first" {
		t.Fatalf("want [first], got %+v", envs)
	}

	// Second drain with no new writes → nothing.
	envs, _ = c.Drain()
	if len(envs) != 0 {
		t.Fatalf("want no re-delivery, got %+v", envs)
	}

	mustAppend(t, ws, "second")
	envs, _ = c.Drain()
	if len(envs) != 1 || envs[0].Body != "second" {
		t.Fatalf("want [second], got %+v", envs)
	}
}

func TestCursorPartialTrailingLineNotParsed(t *testing.T) {
	ws := t.TempDir()
	p := Path(ws, "build")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	// One complete line + one partial (no trailing newline yet).
	complete, _ := json.Marshal(Envelope{Kind: KindCommand, Body: "done", Source: "cli"})
	partial, _ := json.Marshal(Envelope{Kind: KindCommand, Body: "half", Source: "cli"})
	if err := os.WriteFile(p, append(append(complete, '\n'), partial...), 0o644); err != nil {
		t.Fatal(err)
	}
	c := NewCursor(ws, "build")
	envs, _ := c.Drain()
	if len(envs) != 1 || envs[0].Body != "done" {
		t.Fatalf("partial line should not parse: got %+v", envs)
	}
	// Now complete the partial line.
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.Write([]byte("\n"))
	_ = f.Close()
	envs, _ = c.Drain()
	if len(envs) != 1 || envs[0].Body != "half" {
		t.Fatalf("completed line should parse on next drain: got %+v", envs)
	}
}

func TestCursorSetOffsetSkipsBacklog(t *testing.T) {
	ws := t.TempDir()
	mustAppend(t, ws, "old1")
	mustAppend(t, ws, "old2")
	fi, err := os.Stat(Path(ws, "build"))
	if err != nil {
		t.Fatal(err)
	}
	c := NewCursor(ws, "build")
	c.SetOffset(fi.Size()) // skip pre-existing backlog (T3 resume fix)
	if envs, _ := c.Drain(); len(envs) != 0 {
		t.Fatalf("backlog should be skipped, got %+v", envs)
	}
	mustAppend(t, ws, "new")
	envs, _ := c.Drain()
	if len(envs) != 1 || envs[0].Body != "new" {
		t.Fatalf("want [new] after offset skip, got %+v", envs)
	}
}

func TestCursorTruncationResets(t *testing.T) {
	ws := t.TempDir()
	// Two lines so the post-rotation single line is strictly shorter than
	// the cursor offset (the realistic rotation case: file shrinks below
	// where we last read). Same-size in-place rewrites within one tick are
	// intentionally not detected — see reader.go.
	mustAppend(t, ws, "first")
	mustAppend(t, ws, "second")
	c := NewCursor(ws, "build")
	if envs, _ := c.Drain(); len(envs) != 2 {
		t.Fatalf("setup drain: want 2, got %d", len(envs))
	}
	// Rotate: clear and write a single shorter line (size < offset).
	if err := os.WriteFile(Path(ws, "build"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	mustAppend(t, ws, "b")
	envs, _ := c.Drain()
	if len(envs) != 1 || envs[0].Body != "b" {
		t.Fatalf("want [b] after truncation reset, got %+v", envs)
	}
}

func TestDrainSkipsMalformedLine(t *testing.T) {
	ws := t.TempDir()
	p := Path(ws, "build")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	good, _ := json.Marshal(Envelope{Kind: KindCommand, Body: "ok", Source: "cli"})
	content := append([]byte("not json\n"), good...)
	content = append(content, '\n')
	_ = os.WriteFile(p, content, 0o644)
	c := NewCursor(ws, "build")
	envs, _ := c.Drain()
	if len(envs) != 1 || envs[0].Body != "ok" {
		t.Fatalf("malformed line should be skipped, got %+v", envs)
	}
}

// TestDrainSeekError covers reader.go:68 — the f.Seek error branch.
// A negative SetOffset forces Seek(negative, SeekStart) → "invalid argument".
func TestDrainSeekError(t *testing.T) {
	ws := t.TempDir()
	mustAppend(t, ws, "existing")
	c := NewCursor(ws, "build")
	c.SetOffset(-1) // negative offset → Seek from start returns EINVAL
	_, err := c.Drain()
	if err == nil {
		t.Fatal("want Seek error for negative offset, got nil")
	}
}

func TestEnvelope_CorrID_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	now := func() time.Time { return time.Unix(0, 0).UTC() }
	want := Envelope{Kind: KindCommand, Body: "summarize", CorrID: "c1", Source: "supervisor"}
	if err := Append(dir, "build", want, now); err != nil {
		t.Fatalf("append: %v", err)
	}
	got, err := NewCursor(dir, "build").Drain()
	if err != nil || len(got) != 1 {
		t.Fatalf("drain: %v len=%d", err, len(got))
	}
	if got[0].CorrID != "c1" {
		t.Fatalf("CorrID = %q, want c1", got[0].CorrID)
	}
}

func mustAppend(t *testing.T, ws, body string) {
	t.Helper()
	if err := Append(ws, "build", Envelope{Kind: KindCommand, Body: body, Source: "cli"}, fixedNow); err != nil {
		t.Fatalf("Append(%q): %v", body, err)
	}
}

// TestKind_Valid_AllRecognizedKinds pins the exhaustive set of envelope kinds
// the bridge accepts. Adding a Kind constant requires extending Valid() in
// the same change; this test fails if either drifts. Cycle-124 F4 added
// KindKeystroke to the set (per ADR-0023 addendum) so the operator can send
// raw tmux key tokens through `evolve bridge send --kind=keystroke`.
func TestKind_Valid_AllRecognizedKinds(t *testing.T) {
	for _, k := range []Kind{KindCommand, KindInterrupt, KindNudge, KindSystemRule, KindKeystroke} {
		if !k.Valid() {
			t.Errorf("Kind(%q).Valid() = false; every defined Kind must satisfy Valid()", k)
		}
	}
}

// TestKind_Valid_RejectsUnknown is the negative pin: an unknown string value
// MUST be rejected. This is the gate cmd_bridge.go relies on to keep
// operator typos from reaching the dispatch switch in injectEnvelope.
func TestKind_Valid_RejectsUnknown(t *testing.T) {
	for _, bad := range []Kind{"", "unknown", "Command", "KEYSTROKE", "key-stroke"} {
		if bad.Valid() {
			t.Errorf("Kind(%q).Valid() = true; want false (case-sensitive exact match only)", bad)
		}
	}
}

// TestKindKeystroke_JSONRoundTrip pins that a keystroke envelope survives the
// full Append→ReadFile→Unmarshal cycle byte-for-byte. Operators script
// `evolve bridge send --kind=keystroke --body=Enter` which writes one NDJSON
// line; the driver's drain loop reads it back and dispatches. A serialization
// drift here would silently corrupt the operator's "full tmux control" hatch.
func TestKindKeystroke_JSONRoundTrip(t *testing.T) {
	ws := t.TempDir()
	cases := []struct {
		name, body, source string
	}{
		{"named-key", "Enter", "cli"},
		{"control-char", "C-c", "cli"},
		{"multi-token", "y Enter", "observer"},
		{"unicode", "はい", "cli"},
		{"empty", "", "cli"},
		{"long", strings.Repeat("x", 1024), "cli"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Each subtest gets its own agent to avoid Append's append-to-same
			// file behavior bleeding across cases.
			agent := "agent-" + tc.name
			env := Envelope{Kind: KindKeystroke, Body: tc.body, Source: tc.source}
			if err := Append(ws, agent, env, fixedNow); err != nil {
				t.Fatalf("Append: %v", err)
			}
			envs, err := NewCursor(ws, agent).Drain()
			if err != nil {
				t.Fatalf("Drain: %v", err)
			}
			if len(envs) != 1 {
				t.Fatalf("expected 1 envelope; got %d", len(envs))
			}
			got := envs[0]
			if got.Kind != KindKeystroke {
				t.Errorf("Kind=%q want %q", got.Kind, KindKeystroke)
			}
			if got.Body != tc.body {
				t.Errorf("Body=%q want %q", got.Body, tc.body)
			}
			if got.Source != tc.source {
				t.Errorf("Source=%q want %q", got.Source, tc.source)
			}
			if got.TS == "" {
				t.Error("TS empty — Append must mint a timestamp")
			}
		})
	}
}

// TestKindKeystroke_AppendOrderPreserved pins that multiple keystroke
// envelopes for the same agent drain in the order they were appended.
// This matters when the operator scripts a multi-step recovery
// ("Escape" then "y Enter") — the bridge must inject them in that order.
func TestKindKeystroke_AppendOrderPreserved(t *testing.T) {
	ws := t.TempDir()
	bodies := []string{"Escape", "y Enter", "Up", "Down", "C-c"}
	for _, b := range bodies {
		if err := Append(ws, "ord", Envelope{Kind: KindKeystroke, Body: b, Source: "cli"}, fixedNow); err != nil {
			t.Fatalf("Append(%q): %v", b, err)
		}
	}
	envs, err := NewCursor(ws, "ord").Drain()
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if len(envs) != len(bodies) {
		t.Fatalf("want %d envelopes; got %d", len(bodies), len(envs))
	}
	for i, want := range bodies {
		if envs[i].Body != want {
			t.Errorf("envs[%d].Body=%q want %q (order broken)", i, envs[i].Body, want)
		}
		if envs[i].Kind != KindKeystroke {
			t.Errorf("envs[%d].Kind=%q want %q", i, envs[i].Kind, KindKeystroke)
		}
	}
}

// TestKindKeystroke_MixedWithOtherKinds pins that an inbox with mixed
// envelope kinds — keystroke + command + interrupt + nudge — drains all
// of them with each Kind preserved. The cursor doesn't filter; the
// driver's dispatch switch reads Kind and routes per-envelope.
func TestKindKeystroke_MixedWithOtherKinds(t *testing.T) {
	ws := t.TempDir()
	sends := []Envelope{
		{Kind: KindCommand, Body: "do x", Source: "cli"},
		{Kind: KindKeystroke, Body: "Enter", Source: "cli"},
		{Kind: KindInterrupt, Body: "STOP", Source: "cli"},
		{Kind: KindKeystroke, Body: "Escape", Source: "observer"},
		{Kind: KindNudge, Body: "still there?", Source: "observer"},
		{Kind: KindSystemRule, Body: "rule", Source: "cli"},
	}
	for _, e := range sends {
		if err := Append(ws, "mix", e, fixedNow); err != nil {
			t.Fatalf("Append %+v: %v", e, err)
		}
	}
	envs, err := NewCursor(ws, "mix").Drain()
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if len(envs) != len(sends) {
		t.Fatalf("want %d envelopes; got %d", len(sends), len(envs))
	}
	for i, want := range sends {
		if envs[i].Kind != want.Kind {
			t.Errorf("envs[%d].Kind=%q want %q", i, envs[i].Kind, want.Kind)
		}
		if envs[i].Body != want.Body {
			t.Errorf("envs[%d].Body=%q want %q", i, envs[i].Body, want.Body)
		}
		if envs[i].Source != want.Source {
			t.Errorf("envs[%d].Source=%q want %q", i, envs[i].Source, want.Source)
		}
	}
}
