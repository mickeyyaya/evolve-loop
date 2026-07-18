package dispatchevents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// EmitGoalStallEscalated must append a WARN goal-stall-escalated event naming the
// stalled goal to abnormal-events.jsonl, so the observer can react.
func TestEmitGoalStallEscalated(t *testing.T) {
	dir := t.TempDir()
	w := NewWriter(dir)
	if err := w.EmitGoalStallEscalated(644, 3, 3, "805f6cedabc"); err != nil {
		t.Fatalf("EmitGoalStallEscalated: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "abnormal-events.jsonl"))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	s := string(raw)
	for _, want := range []string{string(EventGoalStallEscalated), string(SeverityWarn), "805f6cedabc"} {
		if !strings.Contains(s, want) {
			t.Errorf("event line missing %q:\n%s", want, s)
		}
	}
}
