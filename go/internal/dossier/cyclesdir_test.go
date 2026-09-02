package dossier

import (
	"path/filepath"
	"testing"
)

// TestCyclesDir pins the dossier corpus location: the producer commits here,
// the chronicle and the dashboard read here. Renaming it is a protocol change
// (ADR-0094), not a refactor.
func TestCyclesDir(t *testing.T) {
	t.Parallel()
	if got, want := CyclesDir("/root"), filepath.Join("/root", "knowledge-base", "cycles"); got != want {
		t.Fatalf("CyclesDir = %q, want %q", got, want)
	}
}
