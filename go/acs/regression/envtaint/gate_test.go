//go:build acs

// Package envtaint's gate: every fold-aware Go reader of an EVOLVE_ operator dial
// must have a flagregistry row (R_go ⊆ registry). This is the honest-metric half
// of ADR-0064 Pillar 2 — the deterministic kill for the cycle-20 metric dodge.
//
// It is the go/types-folded counterpart to the flagreaders guard. flagreaders
// scans go/ast string LITERALS, so a reader written as a split-const
// (os.Getenv("EVOLVE_"+"WORKTREE_BASE")) is invisible to it and its registry row
// could be deleted with no guard objecting — exactly cycle-20. This gate folds
// the concatenation, so the reader is seen and a deleted row becomes an orphan.
//
// The gate is ONE-directional (R_go ⊆ registry, not equality): the registry
// legitimately holds flags whose only reader is non-Go (agent/skill/CI/shell
// surfaces, e.g. EVOLVE_REFLECTION_JOURNAL), which the existing flagreaders text
// scan covers. IPC-protocol keys (writer-injected) are excluded via the
// `// SSOT IPC-protocol-allowed` marker, a Pillar-1 protected surface.
package envtaint

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// TestEveryGoReaderHasRegistryRow fails if any fold-aware EVOLVE_ operator-dial
// reader in production Go lacks a flagregistry row.
func TestEveryGoReaderHasRegistryRow(t *testing.T) {
	repo := acsassert.RepoRoot(t)
	r, skipped, err := ReadSet(filepath.Join(repo, "go"))
	if err != nil {
		t.Fatalf("ReadSet: %v", err)
	}
	if len(skipped) > 0 {
		t.Logf("envtaint: %d unparseable file(s) skipped: %v", len(skipped), skipped)
	}
	// Non-vacuity: the scan must actually find the live dials; an empty or tiny
	// read-set means the walk broke and the gate would pass trivially.
	if len(r) < 10 {
		t.Fatalf("read-set implausibly small (%d keys) — the production scan is likely broken", len(r))
	}
	for _, key := range r {
		if _, ok := flagregistry.Lookup(key); !ok {
			t.Errorf("fold-aware Go reader %q has NO flagregistry row.\n"+
				"  Either add it to go/internal/flagregistry/registry_table.go (sorted), or — if it is a\n"+
				"  writer-injected IPC key, not an operator dial — annotate the declaration\n"+
				"  `// SSOT IPC-protocol-allowed`. Unlike the go/ast flagreaders scan, a split-const\n"+
				"  reader (\"EVOLVE_\"+\"X\") does not hide from this gate.", key)
		}
	}
}

// TestCycle20Dodge_OrphanedWhenRowDeleted replays the exact cycle-20 metric dodge
// and proves the gate catches it: a split-const reader of an operator dial keeps
// the dial working byte-identically while vanishing from the go/ast literal scan,
// so the registry row could be deleted unnoticed. The fold-aware read-set still
// contains the key, so deleting the row leaves a detectable orphan.
func TestCycle20Dodge_OrphanedWhenRowDeleted(t *testing.T) {
	const dodge = `package p

import "os"

// the cycle-20 dodge: byte-identical behavior, invisible to a literal scan.
var _ = os.Getenv("EVOLVE_" + "WORKTREE_BASE")
`
	r, err := EvolveConstKeys(dodge)
	if err != nil {
		t.Fatalf("EvolveConstKeys: %v", err)
	}
	// Simulate the registry AFTER the dodge deleted the WORKTREE_BASE row.
	rowExists := func(string) bool { return false }
	var orphans []string
	for _, k := range r {
		if !rowExists(k) {
			orphans = append(orphans, k)
		}
	}
	found := false
	for _, o := range orphans {
		if o == "EVOLVE_WORKTREE_BASE" {
			found = true
		}
	}
	if !found {
		t.Fatalf("cycle-20 dodge NOT caught: deleting the row left no orphan (read-set=%v)", r)
	}
}
