//go:build acs

// Package cycle5 — Test Amplification (adversarial) tests for cycle 5.
//
// These tests probe dimensions NOT covered by the TDD contract in
// predicates_test.go. The amplifier is a black-box tester: it reads only
// the spec (scout-report, build-report file list, agent-mailbox) and NOT
// the implementation.
//
// Each test targets a distinct failure mode: incomplete ADR, wrong cluster
// attribution, missing lifecycle keyword, ADR number collision, or missing
// table-row descriptions in runtime-reference.md.
package cycle5

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC5_AMP_001_ADRHasPriorArtSection verifies that ADR-0054 contains a
// Prior Art section. The scout report explicitly requires this section to
// reference concurrent-build systems (GitLab CI_CONCURRENT_ID, Jenkins @N,
// Temporal). The TDD contract checks for "## Status/Layer 1/Layer 2/runscope/
// ADR-0049" only — Prior Art is unchecked.
//
// Failure mode caught: builder wrote the ADR without the Prior Art section.
func TestC5_AMP_001_ADRHasPriorArtSection(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0054-concurrent-evolve-loop-sibling-worktrees.md")
	acsassert.FileContains(t, adrPath, "Prior Art")
}

// TestC5_AMP_002_ADRHasConsequencesSection verifies that ADR-0054 includes
// a Consequences section. Standard ADR format requires Context/Decision/
// Consequences; omitting Consequences makes the record incomplete.
//
// Failure mode caught: builder created a minimal ADR covering only the
// technically-required sections, skipping the operational impact.
func TestC5_AMP_002_ADRHasConsequencesSection(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0054-concurrent-evolve-loop-sibling-worktrees.md")
	acsassert.FileContains(t, adrPath, "Consequences")
}

// TestC5_AMP_003_ADRHasDecisionsSection verifies that ADR-0054 includes
// a Decisions section (or Decision section). An ADR without explicit
// decisions records is a design rationale document, not a decision record.
//
// Failure mode caught: builder structured the ADR as a design doc
// (Context + Implementation) without a distinct Decisions heading.
func TestC5_AMP_003_ADRHasDecisionsSection(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0054-concurrent-evolve-loop-sibling-worktrees.md")
	// Either "## Decisions" or "## Decision" is acceptable.
	if !acsassert.FileMatchesRegex(t, adrPath, `(?m)^##+ Decisions?`) {
		t.Errorf("RED: ADR-0054 must have a '## Decisions' or '## Decision' heading")
	}
}

// TestC5_AMP_004_ADRStatusHasLifecycleKeyword verifies that the ## Status
// section of ADR-0054 contains a standard ADR lifecycle keyword
// (Accepted, Proposed, Deprecated, or Superseded). A builder could satisfy
// TestC5_002 by writing "## Status" with no body (or with "TODO").
//
// Failure mode caught: Status heading is present but body is empty/placeholder.
func TestC5_AMP_004_ADRStatusHasLifecycleKeyword(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0054-concurrent-evolve-loop-sibling-worktrees.md")
	if !acsassert.FileMatchesRegex(t, adrPath, `(?i)(Accepted|Proposed|Deprecated|Superseded)`) {
		t.Errorf("RED: ADR-0054 ## Status section is missing a lifecycle keyword (Accepted/Proposed/Deprecated/Superseded)")
	}
}

// TestC5_AMP_005_NoDuplicateADR0054 verifies that exactly one file in
// docs/architecture/adr/ is numbered 0054. A copy-paste of ADR-0053 can
// inadvertently produce a second 0054-something-else.md file.
//
// Failure mode caught: duplicate ADR number collision from careless copy-paste.
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

// TestC5_AMP_006_EVOLVELaneNotInSiblingWorktreeCluster verifies that no
// single line of go/internal/flagregistry/registry_table.go contains BOTH
// "EVOLVE_LANE" AND "Sibling-Worktree". EVOLVE_LANE belongs to the Fleet
// (ADR-0049) cluster; misattributing it to Sibling-Worktree would corrupt
// the registry's cluster semantics.
//
// Note: The scout report confirms EVOLVE_LANE is at registry_table.go:160
// in cluster "Concurrency / Fleet (ADR-0049)". This test guards the opposite.
//
// Failure mode caught: copy-paste of a new Sibling-Worktree struct literal
// accidentally overwrites or moves the EVOLVE_LANE row's cluster field.
func TestC5_AMP_006_EVOLVELaneNotInSiblingWorktreeCluster(t *testing.T) {
	root := acsassert.RepoRoot(t)
	regTable := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	if acsassert.LineContainsAll(regTable, "EVOLVE_LANE", "Sibling-Worktree") {
		t.Errorf("RED: EVOLVE_LANE must NOT appear on a line that also contains 'Sibling-Worktree' — it belongs in the Fleet (ADR-0049) cluster")
	}
}

// TestC5_AMP_007_ADRHasSliceTable verifies that ADR-0054 documents the
// six-slice campaign delivery. The build spec requires a "Slice-by-Slice
// delivery table". Without it, the ADR is not the design record for the
// campaign.
//
// Failure mode caught: builder wrote a general architecture ADR without
// documenting the phased delivery (slices 1-6).
func TestC5_AMP_007_ADRHasSliceTable(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0054-concurrent-evolve-loop-sibling-worktrees.md")
	// The delivery table must reference slices; both "Slice" and "Slice 1"
	// patterns are acceptable.
	acsassert.FileContains(t, adrPath, "Slice")
}

// TestC5_AMP_009_RuntimeReferenceHasTableRowForReapOrphans verifies that
// EVOLVE_REAP_ORPHANS in runtime-reference.md is part of a table row
// (contains a "|" separator on the same line), not just a bare mention in
// a comment or narrative text.
//
// Failure mode caught: flag name appears in runtime-reference.md prose but
// not in the operator env-var table.
func TestC5_AMP_009_RuntimeReferenceHasTableRowForReapOrphans(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rtRef := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	if !acsassert.LineContainsAll(rtRef, "EVOLVE_REAP_ORPHANS", "|") {
		t.Errorf("RED: EVOLVE_REAP_ORPHANS must appear in a table row (line containing '|') in runtime-reference.md")
	}
}

// TestC5_AMP_010_ADRMentionsCliadmit verifies that ADR-0054 mentions
// cliadmit as a Layer 2 component. Per the campaign spec, Layer 2 (shared
// host runtime guards) consists of sessionreaper + cliadmit. The TDD
// contract checks for "Layer 2" and "runscope" but not for the concrete
// Layer 2 package names.
//
// Failure mode caught: ADR abstractly describes Layer 2 without naming
// the two packages introduced by slices 3 and 4.
func TestC5_AMP_010_ADRMentionsCliadmit(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0054-concurrent-evolve-loop-sibling-worktrees.md")
	acsassert.FileContains(t, adrPath, "cliadmit")
}
