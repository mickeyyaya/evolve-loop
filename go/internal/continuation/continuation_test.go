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

// TestManifestName_IsTheFileTheManifestRoundTripsThrough — ADR-0103 unit 09
// review fold (F5): the manifest's file name is exported so its three outside
// consumers (the audit teardown hold, the citation self-cite denylist and the
// ledger's prompt-degrade path field) name the SAME file WriteManifest
// publishes and ReadManifest loads — a rename here fails to compile there.
func TestManifestName_IsTheFileTheManifestRoundTripsThrough(t *testing.T) {
	ws := t.TempDir()
	if err := WriteManifest(ws, Continuation{Cycle: 1285}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws, ManifestName)); err != nil {
		t.Fatalf("WriteManifest publishes <workspace>/%s: %v", ManifestName, err)
	}
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, ManifestName), []byte(`{"cycle":1290}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := ReadManifest(other); err != nil || !ok || got.Cycle != 1290 {
		t.Fatalf("ReadManifest loads the file at ManifestName: ok=%v err=%v got=%+v", ok, err, got)
	}
	if ManifestName != "continuation-manifest.json" {
		t.Fatalf("the on-disk name is the wire: %q", ManifestName)
	}
}
