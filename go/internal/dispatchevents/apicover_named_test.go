package dispatchevents

import (
	"testing"
	"time"
)

// TestWriter_NamedConstructAndEmit names the Writer type explicitly and pins
// its core contract: NewWriter returns a *Writer targeting the workspace's
// abnormal-events.jsonl, and Emit appends one decodable JSONL line carrying
// the event verbatim (with SourcePhase defaulted to "dispatch").
func TestWriter_NamedConstructAndEmit(t *testing.T) {
	ws := t.TempDir()
	var w *Writer = NewWriter(ws)
	w.now = func() time.Time { return time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC) }

	if err := w.Emit(Event{EventType: EventVerifyFailed, Severity: SeverityError, Cycle: 9, Details: "named-writer"}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	events := readEvents(t, ws)
	if len(events) != 1 {
		t.Fatalf("len=%d want 1", len(events))
	}
	e := events[0]
	if e.EventType != EventVerifyFailed || e.Cycle != 9 || e.Severity != SeverityError {
		t.Fatalf("event mismatch: %+v", e)
	}
	if e.SourcePhase != "dispatch" {
		t.Fatalf("SourcePhase=%q want dispatch (default)", e.SourcePhase)
	}
	if e.Timestamp != "2026-06-16T00:00:00Z" {
		t.Fatalf("Timestamp=%q want injected clock value", e.Timestamp)
	}
}
