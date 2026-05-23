package dispatchevents

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// readEvents parses every NDJSON line in <workspace>/abnormal-events.jsonl
// for round-trip assertions. Returns nil on missing file.
func readEvents(t *testing.T, workspace string) []Event {
	t.Helper()
	f, err := os.Open(filepath.Join(workspace, "abnormal-events.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	var out []Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("unmarshal %q: %v", sc.Text(), err)
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return out
}

func TestEmit_BasicShape(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	w := NewWriter(ws)
	if err := w.Emit(Event{EventType: EventCounterNonAdvance, Severity: SeverityWarn, Details: "hi"}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	events := readEvents(t, ws)
	if len(events) != 1 {
		t.Fatalf("len=%d want 1", len(events))
	}
	e := events[0]
	if e.EventType != EventCounterNonAdvance || e.Severity != SeverityWarn || e.Details != "hi" {
		t.Fatalf("event mismatch: %+v", e)
	}
	if e.SourcePhase != "dispatch" {
		t.Fatalf("source_phase=%q want dispatch (default)", e.SourcePhase)
	}
	if e.Timestamp == "" {
		t.Fatalf("timestamp must be auto-filled")
	}
}

func TestEmit_TimestampPreserved(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	w := NewWriter(ws)
	want := "2026-01-15T00:00:00Z"
	if err := w.Emit(Event{EventType: EventClassification, Timestamp: want, Details: "x"}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	got := readEvents(t, ws)[0].Timestamp
	if got != want {
		t.Fatalf("timestamp=%q want %q", got, want)
	}
}

func TestEmit_Append(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	w := NewWriter(ws)
	for _, et := range []EventType{EventCounterNonAdvance, EventVerifyFailed, EventClassification} {
		if err := w.Emit(Event{EventType: et, Severity: SeverityInfo, Details: "x"}); err != nil {
			t.Fatalf("emit: %v", err)
		}
	}
	events := readEvents(t, ws)
	if len(events) != 3 {
		t.Fatalf("len=%d want 3", len(events))
	}
}

func TestEmit_ConcurrentSafe(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	w := NewWriter(ws)
	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			_ = w.Emit(Event{EventType: EventCounterNonAdvance, Cycle: i, Details: "concurrent"})
		}()
	}
	wg.Wait()
	events := readEvents(t, ws)
	if len(events) != N {
		t.Fatalf("len=%d want %d (atomic append failed)", len(events), N)
	}
	// Each line must be valid JSON. readEvents would have failed
	// already if any line was corrupt, but be explicit.
	for i, e := range events {
		if e.EventType != EventCounterNonAdvance {
			t.Fatalf("event %d type=%q want counter-non-advance", i, e.EventType)
		}
	}
}

func TestEmit_CustomNowFunc(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	w := NewWriter(ws)
	w.now = func() time.Time { return time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC) }
	if err := w.Emit(Event{EventType: EventVerifyFailed}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	got := readEvents(t, ws)[0].Timestamp
	if got != "2026-05-23T00:00:00Z" {
		t.Fatalf("ts=%q want 2026-05-23T00:00:00Z", got)
	}
}

func TestEmit_OpenError(t *testing.T) {
	t.Parallel()
	// Workspace dir doesn't exist → OpenFile fails.
	w := NewWriter(filepath.Join(t.TempDir(), "does-not-exist", "sub"))
	if err := w.Emit(Event{EventType: EventCounterNonAdvance}); err == nil {
		t.Fatalf("expected error when workspace missing")
	}
}

func TestEmitCounterNonAdvance_Helper(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	w := NewWriter(ws)
	if err := w.EmitCounterNonAdvance(7); err != nil {
		t.Fatalf("emit: %v", err)
	}
	e := readEvents(t, ws)[0]
	if e.EventType != EventCounterNonAdvance || e.Cycle != 7 || e.Severity != SeverityWarn {
		t.Fatalf("event mismatch: %+v", e)
	}
	if !strings.Contains(e.Details, "cycle 7") {
		t.Fatalf("details should mention cycle 7: %q", e.Details)
	}
	if e.RemediationHint == "" {
		t.Fatalf("counter-non-advance must include remediation hint")
	}
}

func TestEmitVerifyFailed_Helper(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	w := NewWriter(ws)
	if err := w.EmitVerifyFailed(3, []string{"scout", "builder"}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	e := readEvents(t, ws)[0]
	if e.EventType != EventVerifyFailed || e.Severity != SeverityError || e.Cycle != 3 {
		t.Fatalf("event mismatch: %+v", e)
	}
	if !strings.Contains(e.Details, "scout") || !strings.Contains(e.Details, "builder") {
		t.Fatalf("details should list missing roles: %q", e.Details)
	}
}

func TestEmitClassification_SeverityByClass(t *testing.T) {
	t.Parallel()
	tests := []struct {
		class    string
		wantSev  Severity
	}{
		{"infrastructure", SeverityInfo},
		{"audit-fail", SeverityInfo},
		{"build-fail", SeverityInfo},
		{"ship-gate-config", SeverityInfo},
		{"integrity-breach", SeverityError},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.class, func(t *testing.T) {
			t.Parallel()
			ws := t.TempDir()
			w := NewWriter(ws)
			if err := w.EmitClassification(1, tc.class); err != nil {
				t.Fatalf("emit: %v", err)
			}
			e := readEvents(t, ws)[0]
			if e.Severity != tc.wantSev {
				t.Fatalf("class=%s sev=%s want %s", tc.class, e.Severity, tc.wantSev)
			}
			if e.Classification != tc.class {
				t.Fatalf("classification=%s want %s", e.Classification, tc.class)
			}
		})
	}
}

func TestEmitCircuitBreakerTripped_Helper(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	w := NewWriter(ws)
	if err := w.EmitCircuitBreakerTripped(5, 5, 5); err != nil {
		t.Fatalf("emit: %v", err)
	}
	e := readEvents(t, ws)[0]
	if e.EventType != EventCircuitBreakerTripped || e.Severity != SeverityError || e.Cycle != 5 {
		t.Fatalf("event mismatch: %+v", e)
	}
	if !strings.Contains(e.Details, "threshold=5") {
		t.Fatalf("details should mention threshold: %q", e.Details)
	}
}
