package dossier

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWrite verifies Write creates cycle-N.json and cycle-N.md in the
// target directory (no git commit path). RED: Write doesn't exist yet.
func TestWrite(t *testing.T) {
	d := &Dossier{
		Cycle:        42,
		Goal:         "write test",
		FinalVerdict: VerdictPass,
		Phases:       []PhaseRecord{{Name: "build", Verdict: VerdictPass}},
	}
	dir := t.TempDir()
	if err := Write(d, dir, false); err != nil {
		t.Fatalf("Write: %v", err)
	}
	for _, name := range []string{"cycle-42.json", "cycle-42.md"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("Write: expected %s to exist: %v", name, err)
		}
	}
}
