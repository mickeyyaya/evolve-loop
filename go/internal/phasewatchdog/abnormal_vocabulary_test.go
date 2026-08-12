package phasewatchdog

// abnormal_vocabulary_test.go — the watchdog and the dispatcher both append to
// abnormal-events.jsonl, and their records must speak the SAME vocabulary.
//
// They didn't. The keys matched, so the file looked consistent, but the VALUES
// were out of band: severity "HIGH" where the enum is WARN|ERROR, and an
// event_type absent from EventType. Records that parse but fall outside the
// vocabulary are the worst kind of telemetry defect — every jq filter and every
// typed reader silently skips them, so a stall that killed a phase is invisible
// to exactly the queries an operator runs when a phase went missing.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
)

func TestAppendAbnormalEvent_UsesTheSharedVocabulary(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	fixed := func() time.Time { return time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC) }
	appendAbnormalEvent(ws, "idle_s=600 threshold_s=600 cycle=1450", fixed)

	raw, err := os.ReadFile(filepath.Join(ws, "abnormal-events.jsonl"))
	if err != nil {
		t.Fatalf("watchdog wrote no abnormal event: %v", err)
	}
	// Decode through the DISPATCHER's own type — the other writer of this file.
	// If the watchdog's record cannot be read as one of these, the file has two
	// dialects and every consumer has to know which line came from whom.
	var got dispatchevents.Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &got); err != nil {
		t.Fatalf("the sibling writer's type cannot decode this record: %v\n%s", err, raw)
	}

	// Severity must be in the shared enum. "HIGH" parses into the field and is
	// silently dropped by any filter keyed on WARN|ERROR.
	if got.Severity != dispatchevents.SeverityError && got.Severity != dispatchevents.SeverityWarn {
		t.Errorf("severity = %q, which is outside the shared vocabulary — a stall-kill is invisible to every severity filter", got.Severity)
	}
	// EventType must be a declared one, so a typed reader can switch on it.
	if !dispatchevents.IsKnownEventType(got.EventType) {
		t.Errorf("event_type = %q is not declared in dispatchevents — a typed consumer cannot dispatch on it", got.EventType)
	}
	if got.SourcePhase == "" || got.Details == "" || got.Timestamp == "" {
		t.Errorf("record lost a required field: %+v", got)
	}
}
