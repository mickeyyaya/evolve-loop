package contextfillcorrelate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRejectsAMalformedGlobRoot(t *testing.T) {
	t.Parallel()
	if _, err := Load("["); err == nil {
		t.Fatal("Load over a root that breaks the glob pattern returned nil error")
	}
}

func TestLoadFallsBackToBodyIDOnlyForAnUnclaimedID(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "knowledge-base", "cycles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{"cycle-a.json", "cycle-b.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`{"cycle": 7, "final_verdict": "PASS"}`), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	rows, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Cycle != 7 {
		t.Errorf("first unparseable name took Cycle %d, want the body id 7", rows[0].Cycle)
	}
	if rows[1].Cycle != 0 {
		t.Errorf("second unparseable name took Cycle %d, want 0: the body id 7 is already claimed", rows[1].Cycle)
	}
}
