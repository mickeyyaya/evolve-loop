package dossier

import (
	"path/filepath"
	"testing"
)

func TestCyclesDir(t *testing.T) {
	t.Parallel()
	if got, want := CyclesDir("/root"), filepath.Join("/root", "knowledge-base", "cycles"); got != want {
		t.Fatalf("CyclesDir = %q, want %q", got, want)
	}
}
