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

func mustAppend(t *testing.T, ws, body string) {
	t.Helper()
	if err := Append(ws, "build", Envelope{Kind: KindCommand, Body: body, Source: "cli"}, fixedNow); err != nil {
		t.Fatalf("Append(%q): %v", body, err)
	}
}
