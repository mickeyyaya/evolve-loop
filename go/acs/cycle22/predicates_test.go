//go:build acs

package cycle22

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var deadFlags = []string{
	"EVOLVE_BUILDER_REVIEW_SKILLS",
	"EVOLVE_BUILDER_REVIEW_THRESHOLD",
	"EVOLVE_BUILDER_SELF_REVIEW",
	"EVOLVE_BUILDER_WORKTREE",
	"EVOLVE_PASS_CONFIDENCE_THRESHOLD",
	"EVOLVE_RESEARCH_CACHE_ENABLED",
	"EVOLVE_TRIAGE_AUTO_SKIP_TRIVIAL",
	"EVOLVE_TRIAGE_TOP_N",
	"EVOLVE_USE_LEGACY_BASH",
}

func TestC22_001_DeadFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range deadFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-22 dead-flag-sweep).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

func TestC22_004_NoProductionReaderForDeadFlags(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	for _, flag := range deadFlags {
		envRead := `os.Getenv("` + flag + `")`
		_, _, code, _ := acsassert.SubprocessOutput(
			"grep", "-rn", envRead, goDir,
			"--include=*.go",
			"--exclude=*_test.go",
			"--exclude=registry_table.go",
		)
		if code == 0 {
			t.Errorf("production Go code reads %q via os.Getenv — must be absent.\n"+
				"These flags are dead (0 production readers per scout-report §Key Findings).\n"+
				"Do not add os.Getenv calls for removed flags.",
				flag)
		}
	}
}

func TestC22_005_ControlFlagsMdHasNoDeadRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, flag := range deadFlags {
		if !acsassert.FileNotContains(t, controlFlags, flag) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 9 dead flag rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", flag, controlFlags)
		}
	}
}

func TestC22_006_WorktreePathStillInRegistry(t *testing.T) {
	const worktreePath = "EVOLVE_WORKTREE_PATH"
	if _, ok := flagregistry.Lookup(worktreePath); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH removed.\n"+
			"Builder MUST NOT remove EVOLVE_WORKTREE_PATH from registry_table.go.\n"+
			"It is a live IPC handoff (agents/evolve-tester.md:96,113) pinned by C50_009.\n"+
			"Correct retirement is cluster-10 split-const (deferred to cycle 23).",
			worktreePath)
	}
}

func TestC22_007_RuntimeReferenceMdPreservesResearchCacheEnabled(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	runtimeRef := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	if !acsassert.FileContains(t, runtimeRef, "RESEARCH_CACHE_ENABLED") {
		t.Errorf("RED: docs/operations/runtime-reference.md no longer contains RESEARCH_CACHE_ENABLED.\n"+
			"Builder MUST NOT modify this file.\n"+
			"C89_003 asserts RESEARCH_CACHE_ENABLED presence in this doc; removing it breaks C89_003.\n"+
			"File: %s", runtimeRef)
	}
}
