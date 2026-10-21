package dispatchevents

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmitVerifySingleFlightUnavailable_NamesTheLockPathAndTheError(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()

	err := NewWriter(ws).EmitVerifySingleFlightUnavailable(7, "build-floor", "/hub/.git/evolve-verify.lock", errors.New("flock: operation not permitted"))

	raw, rerr := os.ReadFile(filepath.Join(ws, "abnormal-events.jsonl"))
	var e Event
	if err != nil || rerr != nil || json.Unmarshal(raw, &e) != nil {
		t.Fatalf("emit err = %v, read err = %v, line = %q", err, rerr, raw)
	}
	if e.EventType != EventVerifySingleFlightUnavailable || e.Severity != SeverityWarn || e.Cycle != 7 || e.SourcePhase != "build-floor" ||
		!strings.Contains(e.Details, "/hub/.git/evolve-verify.lock") || !strings.Contains(e.Details, "operation not permitted") || !IsKnownEventType(e.EventType) {
		t.Errorf("event = %+v; want a WARN of the known type that names the lock path and the error", e)
	}
}
