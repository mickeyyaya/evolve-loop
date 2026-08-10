package core

// disposition_seed_apicover_named_test.go — apicover named binding for the
// exported SeedDispositionSkeleton (issue #433 class: a new exported surface
// needs a NAMED covering test in its owning package; the phases/audit
// singlesource pin exercises it cross-package, which apicover does not
// count). Behavior is pinned by disposition_seed_test.go and the audit-side
// gate-semantics pin; this test binds the exported name.

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
