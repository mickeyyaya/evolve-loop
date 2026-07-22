package continuation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadManifest_RoundTrip(t *testing.T) {
	ws := t.TempDir()
	in := Continuation{
		Worktree:     "/repo/.evolve/worktrees/cycle-a-1071",
		Branch:       "evolve/cycle-a-1071",
		SnapshotSHA:  "abc123",
		BaseSHA:      "def456",
		FindingsPath: ".evolve/runs/cycle-1071/failure-digest.json",
		Cycle:        1071,
	}
	if err := WriteManifest(ws, in); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	got, ok, err := ReadManifest(ws)
	if err != nil || !ok {
		t.Fatalf("ReadManifest: ok=%v err=%v", ok, err)
	}
	if got != in {
		t.Errorf("round-trip mismatch: got %+v want %+v", got, in)
	}
}

func TestReadManifest_MissingIsCleanAbsence(t *testing.T) {
	_, ok, err := ReadManifest(t.TempDir())
	if ok || err != nil {
		t.Errorf("missing manifest must be (false, nil), got ok=%v err=%v", ok, err)
	}
}

func TestReadManifest_CorruptIsLoud(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "continuation-manifest.json"), []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadManifest(ws); err == nil {
		t.Error("corrupt manifest must error loudly, never a silent fresh start")
	}
}
