package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApicoverNamed_SeedDispositionSkeleton(t *testing.T) {
	t.Parallel()
	// Empty inputs are a documented silent no-op.
	SeedDispositionSkeleton("", "", 0)
	ws := t.TempDir()
	SeedDispositionSkeleton(ws, t.TempDir(), 999)
	if _, err := os.Stat(filepath.Join(ws, "defect-dispositions.json")); !os.IsNotExist(err) {
		t.Error("seed minted a skeleton with no ancestor ledger present")
	}
}
