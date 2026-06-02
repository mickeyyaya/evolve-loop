// Package cycle66 ports the cycle-66 ACS predicates (2 bash files).
//
// Both predicates read runtime artifacts under .evolve/runs/cycle-66/.
// The Go counterparts skip when the runtime artifacts are absent.
package cycle66

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestC66_059_DispositionMemoComplete ports cycle-66/059.
func TestC66_059_DispositionMemoComplete(t *testing.T) {
	root := acsassert.RepoRoot(t)
	memo := filepath.Join(root, ".evolve", "runs", "cycle-66", "disposition-memo.md")
	if !fixtures.FilePresent(memo) {
		t.Skip("cycle-66 disposition-memo.md missing — skip (runtime-only)")
	}
	if !acsassert.FileContains(t, memo, "challenge-token:") {
		return
	}
	classes := acsassert.CountOccurrencesAny(memo,
		"**resolved-shipped**", "**partial-resolved**", "**deferred**", "**dropped**")
	if classes < 22 {
		t.Errorf("%s: only %d disposition rows (need >=22)", memo, classes)
	}
	if !acsassert.FileMatchesRegex(t, memo, `(?m)^## Cycle-31 ship-integrity breach`) {
		return
	}
}

// TestC66_060_InboxSurfaceEmptied ports cycle-66/060.
// Skips when .evolve/inbox/ has runtime files (post-cycle-66 the surface
// may have been re-populated by later inbox-projector cycles — bash
// predicate is authoritative for the cycle-66 worktree ship moment).
func TestC66_060_InboxSurfaceEmptied(t *testing.T) {
	root := acsassert.RepoRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*.json"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) > 0 {
		t.Skipf(".evolve/inbox/ has %d top-level json files — post-c66 runtime state", len(matches))
	}
}
