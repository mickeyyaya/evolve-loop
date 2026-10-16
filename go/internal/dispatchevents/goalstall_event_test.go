package dispatchevents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmitGoalStallEscalated(t *testing.T) {
	dir := t.TempDir()
	w := NewWriter(dir)
	if err := w.EmitGoalStallEscalated(644, 3, 3, "805f6cedabc", "empty/blocked"); err != nil {
		t.Fatalf("EmitGoalStallEscalated: %v", err)
	}
	if err := w.EmitGoalStallEscalated(645, 5, 5, "805f6cedabc", "non-shipping (fail/empty/blocked)"); err != nil {
		t.Fatalf("EmitGoalStallEscalated(union): %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "abnormal-events.jsonl"))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	s := string(raw)
	for _, want := range []string{
		string(EventGoalStallEscalated), string(SeverityWarn), "805f6cedabc",
		"3 consecutive empty/blocked cycles", "5 consecutive non-shipping (fail/empty/blocked) cycles",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("event line missing %q:\n%s", want, s)
		}
	}
}
