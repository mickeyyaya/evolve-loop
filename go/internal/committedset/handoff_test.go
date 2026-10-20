package committedset

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHandoff_RecordsOnlyARealHandOffAndReadsItBack(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	if HandedOff(ws) {
		t.Fatal("a workspace with no record handed nothing off")
	}
	if err := WriteHandoff(ws, Handoff{Cycle: 7, Routed: []string{"item-a"}}); err != nil {
		t.Fatal(err)
	}
	if !HandedOff(ws) {
		t.Fatal("a recorded hand-off of item-a was not read back")
	}
	empty := t.TempDir()
	if err := os.WriteFile(filepath.Join(empty, HandoffFile), []byte(`{"cycle":7,"routed":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if HandedOff(empty) {
		t.Fatal("a record that routed no item is not a hand-off")
	}
}
