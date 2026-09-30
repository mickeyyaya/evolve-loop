//go:build acs

package cycle354

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

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

func TestC354_Amp_005_DeferredTaskStillDeferred(t *testing.T) {
	registry := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "flagregistry", "registry_table.go")
	if !acsassert.FileContains(t, registry, "default-off") {
		t.Error("RED: EVOLVE_DYNAMIC_ROUTING registry Cluster field no longer contains 'default-off' — " +
			"Task 2 (fix-dynamic-routing-registry-default) may have been accidentally implemented in " +
			"this cycle; it is deferred to cycle-355.")
	}
}
