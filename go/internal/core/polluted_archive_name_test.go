package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func TestArchivePollutedWorkspace_MintsANameThatGCDiscoveryAccepts(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "cycle-1604")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "gc-shadow-manifest.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 9, 1, 21, 14, 50, 11361000, time.UTC) }

	if err := archivePollutedWorkspace(ws, now); err != nil {
		t.Fatalf("archive: %v", err)
	}

	entries, err := os.ReadDir(filepath.Dir(ws))
	if err != nil || len(entries) != 1 {
		t.Fatalf("want one archive beside the workspace, got %v (err %v)", entries, err)
	}
	if name := entries[0].Name(); !gcpolicy.IsPollutedArchive(name) {
		t.Errorf("the guard minted %q, which gc discovery does not take as a dead run", name)
	}
}
