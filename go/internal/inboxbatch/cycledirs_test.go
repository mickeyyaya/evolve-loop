package inboxbatch

import (
	"os"
	"path/filepath"
	"testing"
)

// CycleDirs is the ONE scan for every lifecycle directory the promoter nests
// by cycle (processing/, processed/, rejected/): ascending cycle order, files
// and non-cycle names ignored, a missing parent an empty list — the layout
// the dispatch-state resolver was blind to when cycle 1682 re-pinned a lane
// to an item cycle 1679 had already shipped.
func TestCycleDirs_ListsCycleSubdirsAscendingAndTolerantOfAbsence(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "processed")
	for _, name := range []string{"cycle-10", "cycle-2", "cycle-x", "notes"} {
		if err := os.MkdirAll(filepath.Join(parent, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(parent, "cycle-7"), []byte("a file, not a cycle dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := CycleDirs(parent)
	want := []string{filepath.Join(parent, "cycle-2"), filepath.Join(parent, "cycle-10")}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("CycleDirs = %v, want %v (numeric order, dirs only)", got, want)
	}
	if got := CycleDirs(filepath.Join(t.TempDir(), "never-created")); got != nil {
		t.Fatalf("a missing parent is an empty list, got %v", got)
	}
	if got := ProcessingCycleDirs(filepath.Dir(parent)); got != nil {
		t.Fatalf("ProcessingCycleDirs shares the scan (no processing/ here → nil), got %v", got)
	}
	// The writer's spelling round-trips through the reader's parser.
	if n, ok := ParseProcessingCycle(filepath.Base(CycleDir(parent, "1679"))); !ok || n != 1679 {
		t.Fatalf("CycleDir(parent, \"1679\") must parse back as cycle 1679, got %d %v", n, ok)
	}
}
