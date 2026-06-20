//go:build acs

// Package cycle5 materializes the cycle-5 acceptance criteria for:
//
//   - concurrent-loop-adr-docs: Slice 6 of the concurrency-arch-slices campaign.
//     Deliverables: ADR-0054 (sibling-worktree architecture doc), runtime-reference
//     flag entries for EVOLVE_LANE/EVOLVE_REAP_ORPHANS/EVOLVE_CLI_MAX_CONCURRENT_<CLI>,
//     and flag registry rows for EVOLVE_REAP_ORPHANS + EVOLVE_CLI_MAX_CONCURRENT_<CLI>.
package cycle5

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// --- Task: concurrent-loop-adr-docs ---

// TestC5_001_ADRFileExistsAndTracked asserts that
// docs/architecture/adr/0054-concurrent-evolve-loop-sibling-worktrees.md
// was created in the worktree and is git-tracked. A gitignored file is
// silently dropped at ship (cycle-93 lesson). Also covers AC7 — the ADR
// number must be exactly 0054 (not a renumbered copy of 0053 or 0055).
func TestC5_001_ADRFileExistsAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("docs", "architecture", "adr", "0054-concurrent-evolve-loop-sibling-worktrees.md")
	path := filepath.Join(root, rel)
	if !acsassert.FileExists(t, path) {
		t.Fatalf("RED: %s missing on disk — Builder must create ADR-0054", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s not git-tracked — may be gitignored and dropped at ship", rel)
	}
}

// TestC5_002_ADRFileHasRequiredSections verifies that the ADR file contains
// all five required structural elements: a Status section, Layer 1, Layer 2,
// runscope, and a reference to ADR-0049. Encodes AC1's content requirements.
//
// acs-predicate: config-check — ADR structural assertions are inherently
// doc-section-presence checks; the behavioral anchor is TestC5_001 (git-tracked)
// and TestC5_005 (go build passes with the doc committed).
func TestC5_002_ADRFileHasRequiredSections(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0054-concurrent-evolve-loop-sibling-worktrees.md")
	// acs-predicate: config-check
	acsassert.FileContains(t, adrPath, "## Status")
	acsassert.FileContains(t, adrPath, "Layer 1")
	acsassert.FileContains(t, adrPath, "Layer 2")
	acsassert.FileContains(t, adrPath, "runscope")
	acsassert.FileContains(t, adrPath, "ADR-0049")
}

// TestC5_003_RuntimeReferenceHasAllConcurrencyFlags verifies that
// docs/operations/runtime-reference.md documents all three concurrency flags
// from the sibling-worktree architecture. AC2.
//
// acs-predicate: config-check — runtime-reference.md is an ops documentation
// table; presence of the flag names is the acceptance criterion itself.
func TestC5_003_RuntimeReferenceHasAllConcurrencyFlags(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rtRef := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	// acs-predicate: config-check
	acsassert.FileContains(t, rtRef, "EVOLVE_LANE")
	acsassert.FileContains(t, rtRef, "EVOLVE_REAP_ORPHANS")
	acsassert.FileContains(t, rtRef, "EVOLVE_CLI_MAX_CONCURRENT")
}

// TestC5_005_GoBuildPassesAfterFlagRows verifies that adding the two flag
// registry rows introduces no compilation regression. AC4.
//
// Pre-existing GREEN expected: the branch is documentation-only (Slice 6).
// HEAD aaf12fc5 passes go build; new flag rows are purely data (no new Go code).
func TestC5_005_GoBuildPassesAfterFlagRows(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "build",
		"-C", goDir,
		"./...",
	)
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("RED: go build ./... failed (exit %d):\n%s", code, combined)
	}
}

// TestC5_006_FlagRegistryTestsPassAfterNewRows verifies that the flagregistry
// unit tests still pass after the two new rows are added. AC5. The flag
// registry package has tests that enforce table invariants; new rows must
// satisfy them.
//
// Pre-existing GREEN expected: flagregistry tests pass on HEAD aaf12fc5.
func TestC5_006_FlagRegistryTestsPassAfterNewRows(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-count=1",
		"./internal/flagregistry/...",
	)
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("RED: go test ./internal/flagregistry/ failed (exit %d):\n%s", code, combined)
	}
}

// TestC5_007_SessionreaperDoesNotGateOnEnvVar verifies that sessionreaper.go
// does NOT read EVOLVE_REAP_ORPHANS to gate its core reap logic. AC6 (negative).
//
// The sessionreaper is unconditionally wired in looppreflight (Slice 3); the
// flag exists solely as operator documentation (an opt-out surface for the
// ops table), not as a runtime code gate. Introducing a conditional os.Getenv
// guard in sessionreaper.go would violate the Slice-3 architecture decision.
//
// Pre-existing GREEN expected: sessionreaper.go currently has no such gate.
func TestC5_007_SessionreaperDoesNotGateOnEnvVar(t *testing.T) {
	root := acsassert.RepoRoot(t)
	reaperPath := filepath.Join(root, "go", "internal", "sessionreaper", "sessionreaper.go")
	acsassert.FileNotContains(t, reaperPath, "EVOLVE_REAP_ORPHANS")
}
