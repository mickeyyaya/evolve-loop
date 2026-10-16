package phasewatchdog

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
	var got dispatchevents.Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &got); err != nil {
		t.Fatalf("the sibling writer's type cannot decode this record: %v\n%s", err, raw)
	}

	if got.Severity != dispatchevents.SeverityError && got.Severity != dispatchevents.SeverityWarn {
		t.Errorf("severity = %q, which is outside the shared vocabulary — a stall-kill is invisible to every severity filter", got.Severity)
	}
	if !dispatchevents.IsKnownEventType(got.EventType) {
		t.Errorf("event_type = %q is not declared in dispatchevents — a typed consumer cannot dispatch on it", got.EventType)
	}
	if got.SourcePhase == "" || got.Details == "" || got.Timestamp == "" {
		t.Errorf("record lost a required field: %+v", got)
	}
}
