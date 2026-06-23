//go:build acs

// Package cycle354 — adversarial test amplification for cycle-354.
//
// This file extends C354_001–C354_007 (predicates_test.go) with edge-case
// and boundary tests derived from the specification only (anti-bias: no
// implementation code was read when designing these tests).
//
// Gaps addressed:
//
//	C354_001–003 covered CoreInfra, Platform/CLI Hybrid, Workflow Defaults
//	clusters but skipped the Worktree/Workspace cluster entirely.
//
//	C354_004 requires ≥5 of 10 flags to show DEAD — a partial fix that
//	updates exactly 5 flags would still pass. Tests here require all 10.
//
//	C354_001–002 check for absence of ACTIVE but not DEPRECATED for most
//	flags; a fix that set flags to DEPRECATED instead of DEAD would pass
//	those tests. Tests here check absence of DEPRECATED for all 10.
//
//	The deferred Task 2 (fix-dynamic-routing-registry-default) must not
//	have been accidentally implemented; its sentinel annotation must remain.
package cycle354

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// TestC354_Amp_002_WorktreeClusterFlagsNotActive is the mirror negative check:
// the two Worktree/Workspace cluster flags must not still appear as ACTIVE or
// DEPRECATED. Complementary to TestC354_Amp_001_WorktreeClusterFlagsAreDead.
func TestC354_Amp_002_WorktreeClusterFlagsNotActive(t *testing.T) {
	doc := controlFlagsPath(t)
	forbidden := []string{
		"`EVOLVE_DRY_RUN_PROVISION_WORKTREE` | ACTIVE",
		"`EVOLVE_PROFILE_WORKTREE_AWARE` | ACTIVE",
		"`EVOLVE_DRY_RUN_PROVISION_WORKTREE` | DEPRECATED",
		"`EVOLVE_PROFILE_WORKTREE_AWARE` | DEPRECATED",
	}
	for _, p := range forbidden {
		acsassert.FileNotContains(t, doc, p)
	}
}

// TestC354_Amp_004_NoDeprecatedForRemainingTargetFlags verifies that none of the 5
// remaining cycle-354 target flags carry a DEPRECATED status. Guards against a
// regression where a future edit sets a formerly-dead flag to DEPRECATED instead
// of leaving it absent (after cycle-359 removed the 5 Platform/CLI Hybrid flags).
func TestC354_Amp_004_NoDeprecatedForRemainingTargetFlags(t *testing.T) {
	doc := controlFlagsPath(t)
	flagsWithoutDep := []string{
		"`EVOLVE_RESOLVE_ROOTS_LOADED` | DEPRECATED",
		"`EVOLVE_FAILURE_CLASSIFICATIONS_LOADED` | DEPRECATED",
		"`EVOLVE_DRY_RUN_PROVISION_WORKTREE` | DEPRECATED",
		"`EVOLVE_PROFILE_WORKTREE_AWARE` | DEPRECATED",
		"`EVOLVE_STRICT_FAILURES` | DEPRECATED",
	}
	for _, p := range flagsWithoutDep {
		acsassert.FileNotContains(t, doc, p)
	}
}

// TestC354_Amp_005_DeferredTaskStillDeferred guards that the deferred
// fix-dynamic-routing-registry-default task was NOT accidentally implemented in
// this cycle. The sentinel is the "default-off" substring in the Cluster field of
// the EVOLVE_DYNAMIC_ROUTING registry entry (scout-report.md F2). If Task 2 were
// implemented, "default-off" would be replaced, breaking the generated index unless
// evolve flags generate was also re-run — which is out of scope for cycle-354.
func TestC354_Amp_005_DeferredTaskStillDeferred(t *testing.T) {
	registry := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "flagregistry", "registry_table.go")
	if !acsassert.FileContains(t, registry, "default-off") {
		t.Error("RED: EVOLVE_DYNAMIC_ROUTING registry Cluster field no longer contains 'default-off' — " +
			"Task 2 (fix-dynamic-routing-registry-default) may have been accidentally implemented in " +
			"this cycle; it is deferred to cycle-355.")
	}
}
