package treewalk

import (
	"path/filepath"
	"testing"
)

func TestNamedExports(t *testing.T) {
	root := t.TempDir()
	var snap Snapshot
	snap, err := Take(root)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "s.json")
	if err := snap.Write(file); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := Filter(Diff(snap, loaded), []string{"ignored"}); len(got) != 0 {
		t.Errorf("Filter(Diff) = %v, want none", got)
	}
}
