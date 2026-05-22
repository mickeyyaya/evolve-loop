package log

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// EmitAbnormal appends a JSON line to abnormal-events.jsonl with the
// shape documented in CLAUDE.md (and observed in .evolve/abnormal-events.jsonl):
//
//	{event_type, timestamp, ...arbitrary fields}
//
// Two real-world shapes coexist in the corpus:
//   - subscription-auth-mode → {event_type, ts, cycle, mode, notes}
//   - stall-detected         → {event_type, timestamp, source_phase, severity, details, remediation_hint}
//
// The Go port standardises on `timestamp` (ISO8601 UTC, second precision)
// and writes arbitrary extra fields flat alongside event_type.
func TestEmitAbnormal_WritesJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "abnormal-events.jsonl")
	w := NewSidecarWriter(path)
	defer w.Close()

	if err := w.EmitAbnormal(Event{
		EventType: "stall-detected",
		Severity:  "HIGH",
		Fields: map[string]any{
			"source_phase":     "phase-watchdog",
			"details":          "idle_s=605 threshold_s=600",
			"remediation_hint": "reduce scope",
		},
	}); err != nil {
		t.Fatalf("EmitAbnormal: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	line := strings.TrimSpace(string(raw))
	if !strings.HasSuffix(line, "}") {
		t.Fatalf("not a single JSON line: %q", line)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("invalid JSON: %v: %q", err, line)
	}
	if got["event_type"] != "stall-detected" {
		t.Errorf("event_type=%v, want stall-detected", got["event_type"])
	}
	if got["severity"] != "HIGH" {
		t.Errorf("severity=%v, want HIGH", got["severity"])
	}
	if got["source_phase"] != "phase-watchdog" {
		t.Errorf("source_phase=%v, want phase-watchdog", got["source_phase"])
	}
	ts, ok := got["timestamp"].(string)
	if !ok || ts == "" {
		t.Fatalf("missing timestamp: %v", got["timestamp"])
	}
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Errorf("timestamp %q not RFC3339: %v", ts, err)
	}
}

func TestEmitAbnormal_AppendsMultipleLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "abnormal-events.jsonl")
	w := NewSidecarWriter(path)
	defer w.Close()

	for i := 0; i < 5; i++ {
		if err := w.EmitAbnormal(Event{EventType: "test", Fields: map[string]any{"i": i}}); err != nil {
			t.Fatalf("emit %d: %v", i, err)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5\n%s", len(lines), raw)
	}
	for i, line := range lines {
		var got map[string]any
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Errorf("line %d invalid JSON: %v: %q", i, err, line)
			continue
		}
		// JSON numbers decode as float64
		if int(got["i"].(float64)) != i {
			t.Errorf("line %d: i=%v, want %d", i, got["i"], i)
		}
	}
}

func TestEmitAbnormal_ConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "abnormal-events.jsonl")
	w := NewSidecarWriter(path)
	defer w.Close()

	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			_ = w.EmitAbnormal(Event{EventType: "race", Fields: map[string]any{"i": i}})
		}(i)
	}
	wg.Wait()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != N {
		t.Fatalf("got %d lines, want %d", len(lines), N)
	}
	for _, line := range lines {
		var got map[string]any
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Errorf("invalid JSON under race: %v: %q", err, line)
		}
	}
}

// EmitPhase records phase lifecycle into a structured slog channel. We
// don't pin the exact slog handler output (depends on stdlib version);
// we just verify it doesn't error and that any provided fields appear.
func TestEmitPhase_StructuredFields(t *testing.T) {
	var buf strings.Builder
	logger := NewJSONLogger(&buf, "info")
	EmitPhase(logger, "scout", "started", map[string]any{
		"cycle":    104,
		"agent":    "scout",
		"goal_sha": "abc123",
	})
	out := buf.String()
	if !strings.Contains(out, `"phase":"scout"`) {
		t.Errorf("missing phase=scout in: %s", out)
	}
	if !strings.Contains(out, `"event":"started"`) {
		t.Errorf("missing event=started in: %s", out)
	}
	if !strings.Contains(out, `"cycle":104`) {
		t.Errorf("missing cycle=104 in: %s", out)
	}
}

func TestEvent_OverrideTimestamp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "abnormal-events.jsonl")
	w := NewSidecarWriter(path)
	defer w.Close()

	fixedTS := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	if err := w.EmitAbnormal(Event{
		EventType: "pinned",
		Timestamp: fixedTS,
	}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), `"timestamp":"2026-05-22T12:00:00Z"`) {
		t.Errorf("override timestamp not honored:\n%s", raw)
	}
}

func TestNewSidecarWriter_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deeper", "abnormal-events.jsonl")
	w := NewSidecarWriter(path)
	defer w.Close()

	if err := w.EmitAbnormal(Event{EventType: "x"}); err != nil {
		t.Fatalf("emit into nested dir: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("sidecar file not created: %v", err)
	}
}

func TestEvent_EventTypeRequired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "abnormal-events.jsonl")
	w := NewSidecarWriter(path)
	defer w.Close()

	if err := w.EmitAbnormal(Event{}); err == nil {
		t.Fatal("EmitAbnormal with empty event_type must error")
	}
}

func TestParseLevel(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"debug", "DEBUG"},
		{"DEBUG", "DEBUG"},
		{"info", "INFO"},
		{"warn", "WARN"},
		{"warning", "WARN"},
		{"ERROR", "ERROR"},
		{"err", "ERROR"},
		{"", "INFO"},
		{"nonsense", "INFO"},
		{"  debug  ", "DEBUG"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := parseLevel(tc.in).String(); got != tc.want {
				t.Errorf("parseLevel(%q)=%s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestEmitPhase_NilLoggerIsNoOp(t *testing.T) {
	// Should not panic.
	EmitPhase(nil, "scout", "started", nil)
}

func TestEmitPhase_NoExtraFields(t *testing.T) {
	var buf strings.Builder
	logger := NewJSONLogger(&buf, "info")
	EmitPhase(logger, "audit", "completed", nil)
	out := buf.String()
	if !strings.Contains(out, `"phase":"audit"`) || !strings.Contains(out, `"event":"completed"`) {
		t.Errorf("missing phase/event in: %s", out)
	}
}

func TestEmitAbnormal_OpenFileError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission bits")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "abnormal-events.jsonl")
	// Pre-create as 0o000 so OpenFile O_APPEND can't open it.
	if err := os.WriteFile(path, []byte{}, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	w := NewSidecarWriter(path)
	defer w.Close()
	if err := w.EmitAbnormal(Event{EventType: "x"}); err == nil {
		t.Fatal("expected open error on 0o000 sidecar")
	}
}

func TestEmitAbnormal_MarshalError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "abnormal-events.jsonl")
	w := NewSidecarWriter(path)
	defer w.Close()
	// channels cannot marshal — drives marshalSorted's error branch.
	if err := w.EmitAbnormal(Event{
		EventType: "x",
		Fields:    map[string]any{"chan": make(chan int)},
	}); err == nil {
		t.Fatal("expected marshal error on un-marshalable field")
	}
}

func TestEmitAbnormal_MkdirParentError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Sidecar path under a regular file — mkdir parent fails.
	w := NewSidecarWriter(filepath.Join(blocker, "child", "abnormal.jsonl"))
	defer w.Close()
	if err := w.EmitAbnormal(Event{EventType: "x"}); err == nil {
		t.Fatal("expected mkdir error under a regular-file path")
	}
}

func TestEmitAbnormal_FieldsCannotOverrideCanonical(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "abnormal-events.jsonl")
	w := NewSidecarWriter(path)
	defer w.Close()

	if err := w.EmitAbnormal(Event{
		EventType: "real",
		Fields: map[string]any{
			"event_type": "FORGED",
			"timestamp":  "1970-01-01T00:00:00Z",
			"safe":       "kept",
		},
	}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	raw, _ := os.ReadFile(path)
	line := string(raw)
	if strings.Contains(line, `"event_type":"FORGED"`) {
		t.Errorf("Fields managed to override event_type: %s", line)
	}
	if strings.Contains(line, `1970-01-01`) {
		t.Errorf("Fields managed to override timestamp: %s", line)
	}
	if !strings.Contains(line, `"safe":"kept"`) {
		t.Errorf("non-canonical field lost: %s", line)
	}
}
