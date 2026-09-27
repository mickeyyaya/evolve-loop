package rawgitratchet

import (
	"os"
	"path/filepath"
	"testing"
)

// moduleRoot walks up from the working directory (the package directory under
// go test) to the directory holding go.mod, so a copy of the module scans itself.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the working directory")
		}
		dir = parent
	}
}

// TestRatchet_NoNewRawGitFixtures is the ratchet: every raw git init site in
// the module's bound test files must be listed in baseline.json, exactly.
func TestRatchet_NoNewRawGitFixtures(t *testing.T) {
	root := moduleRoot(t)
	files, note, err := BoundTestFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if note != "" {
		t.Log(note)
	}
	sites, err := Sites(root, files)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := LoadBaseline("baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(sites, baseline); err != nil {
		t.Error(err)
	}
}
