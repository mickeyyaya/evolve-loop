//go:build acs

package cycle5

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC5_AMP_001_ADRHasPriorArtSection(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0054-concurrent-evolve-loop-sibling-worktrees.md")
	acsassert.FileContains(t, adrPath, "Prior Art")
}

func TestC5_AMP_002_ADRHasConsequencesSection(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0054-concurrent-evolve-loop-sibling-worktrees.md")
	acsassert.FileContains(t, adrPath, "Consequences")
}

func TestC5_AMP_003_ADRHasDecisionsSection(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0054-concurrent-evolve-loop-sibling-worktrees.md")
	if !acsassert.FileMatchesRegex(t, adrPath, `(?m)^##+ Decisions?`) {
		t.Errorf("RED: ADR-0054 must have a '## Decisions' or '## Decision' heading")
	}
}

func TestC5_AMP_004_ADRStatusHasLifecycleKeyword(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0054-concurrent-evolve-loop-sibling-worktrees.md")
	if !acsassert.FileMatchesRegex(t, adrPath, `(?i)(Accepted|Proposed|Deprecated|Superseded)`) {
		t.Errorf("RED: ADR-0054 ## Status section is missing a lifecycle keyword (Accepted/Proposed/Deprecated/Superseded)")
	}
}

func TestC5_AMP_005_NoDuplicateADR0054(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrDir := filepath.Join(root, "docs", "architecture", "adr")
	matches, err := filepath.Glob(filepath.Join(adrDir, "0054-*.md"))
	if err != nil {
		t.Fatalf("glob error: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("RED: expected exactly 1 file matching 0054-*.md in adr/, got %d: %v", len(matches), matches)
	}
}

func TestC5_AMP_006_EVOLVELaneNotInSiblingWorktreeCluster(t *testing.T) {
	root := acsassert.RepoRoot(t)
	regTable := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	if acsassert.LineContainsAll(regTable, "EVOLVE_LANE", "Sibling-Worktree") {
		t.Errorf("RED: EVOLVE_LANE must NOT appear on a line that also contains 'Sibling-Worktree' — it belongs in the Fleet (ADR-0049) cluster")
	}
}

func TestC5_AMP_007_ADRHasSliceTable(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0054-concurrent-evolve-loop-sibling-worktrees.md")
	acsassert.FileContains(t, adrPath, "Slice")
}

func TestC5_AMP_009_RuntimeReferenceHasTableRowForReapOrphans(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rtRef := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	if !acsassert.LineContainsAll(rtRef, "EVOLVE_REAP_ORPHANS", "|") {
		t.Errorf("RED: EVOLVE_REAP_ORPHANS must appear in a table row (line containing '|') in runtime-reference.md")
	}
}

func TestC5_AMP_010_ADRMentionsCliadmit(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0054-concurrent-evolve-loop-sibling-worktrees.md")
	acsassert.FileContains(t, adrPath, "cliadmit")
}
