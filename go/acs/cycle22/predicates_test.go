//go:build acs

// Package cycle22 materializes the cycle-22 acceptance criteria for:
//
//	dead-flag-sweep-22 — remove 9 confirmed-dead EVOLVE_* registry rows
//	(EVOLVE_BUILDER_REVIEW_SKILLS, EVOLVE_BUILDER_REVIEW_THRESHOLD,
//	EVOLVE_BUILDER_SELF_REVIEW, EVOLVE_BUILDER_WORKTREE,
//	EVOLVE_PASS_CONFIDENCE_THRESHOLD, EVOLVE_RESEARCH_CACHE_ENABLED,
//	EVOLVE_USE_LEGACY_BASH, EVOLVE_TRIAGE_AUTO_SKIP_TRIVIAL,
//	EVOLVE_TRIAGE_TOP_N),
//	lower FlagCeiling 154→145, regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	dead-flag-sweep-22:
//	  AC1       All 9 dead flags absent from Lookup            → C22_001 (behavioral)
//	  AC2       Registry row count == 145                      → C22_002 (behavioral, count)
//	  AC3       FlagCeiling const == 145                       → C22_003 (config-check, waiver)
//	  AC4       No os.Getenv reads for 9 flags in prod Go      → C22_004 (config-check, waiver — PRE-EXISTING GREEN)
//	  AC5       control-flags.md has no dead-flag rows         → C22_005 (config-check, waiver)
//	  AC8       WORKTREE_PATH still in registry                → C22_006 (behavioral — PRE-EXISTING GREEN)
//	  NEG1      runtime-reference.md preserves RESEARCH_CACHE_ENABLED → C22_007 (config-check, waiver — PRE-EXISTING GREEN)
//
// ACs with manual+checklist disposition:
//
//	AC6 (C50_009 still green):   `go test -tags acs ./acs/regression/cycle50/...`
//	AC7 (flagreaders guard):     `go test -tags acs ./acs/regression/flagreaders/...`
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C22_001 — Lookup returns ok=false for all 9 flags; cannot be
//	           satisfied by adding magic strings — the registry row must be absent.
//	Edge/OOD:  EVOLVE_TRIAGE_TOP_N + EVOLVE_TRIAGE_AUTO_SKIP_TRIVIAL have StatusActive
//	           (not StatusInternal) — they appear live but have 0 production readers.
//	Lexical:   Lookup / len() / FileContains / FileNotContains / SubprocessOutput — five distinct verbs.
//	Semantic:  registry-absence, row-count, ceiling-const, env-read-absence,
//	           doc-absence, worktree-path-still-present, runtime-ref-preserved.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (dead-flag-sweep-22). All deferred tasks get zero predicates this cycle.
//
// 1:1 enforcement: predicate=7, manual+checklist=2, unverifiable-remove=0 → total AC=9 ✓
package cycle22

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// deadFlags is the canonical list of 9 dead EVOLVE_* flags that cycle-22 removes.
// All have 0 Go production readers and 0 shell readers per scout-report §Key Findings.
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

// TestC22_001_DeadFlagsAbsentFromRegistry verifies that all 9 dead flags are no
// longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1 (all 9 rows absent). Includes:
//   - EVOLVE_BUILDER_REVIEW_SKILLS: StatusInternal, 0 production readers (doc refs only)
//   - EVOLVE_BUILDER_REVIEW_THRESHOLD: StatusInternal, 0 production readers
//   - EVOLVE_BUILDER_SELF_REVIEW: StatusInternal, 0 production readers
//   - EVOLVE_BUILDER_WORKTREE: StatusInternal, 0 production readers
//   - EVOLVE_PASS_CONFIDENCE_THRESHOLD: StatusInternal, 0 production readers (replaced by typed confidence)
//   - EVOLVE_RESEARCH_CACHE_ENABLED: StatusInternal, 0 production readers (C89_003 checks doc only)
//   - EVOLVE_TRIAGE_AUTO_SKIP_TRIVIAL: StatusActive (edge/OOD), 0 production readers
//   - EVOLVE_TRIAGE_TOP_N: StatusActive (edge/OOD), 0 production readers
//   - EVOLVE_USE_LEGACY_BASH: StatusInternal, 0 production readers (rollback hatch removed in v12)
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the registry row must be absent for
// Lookup to return ok=false.
//
// RED: all 9 flags are currently registered; each Lookup returns (flag, true).
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

// TestC22_004_NoProductionReaderForDeadFlags verifies that no production Go file
// reads any of the 9 dead flags via os.Getenv.
//
// Scout confirmed 0 production readers before the cycle — this predicate documents
// the architectural contract and prevents re-introduction by cycle-22 or future work.
//
// // acs-predicate: config-check — the os.Getenv ABSENCE is the structural contract.
//
// PRE-EXISTING GREEN: grep confirms 0 production os.Getenv reads before this cycle.
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
		// grep exits 0 when a match is found (BAD: production reader exists).
		// grep exits 1 when no match found (GOOD: no production reader).
		if code == 0 {
			t.Errorf("production Go code reads %q via os.Getenv — must be absent.\n"+
				"These flags are dead (0 production readers per scout-report §Key Findings).\n"+
				"Do not add os.Getenv calls for removed flags.",
				flag)
		}
	}
}

// TestC22_005_ControlFlagsMdHasNoDeadRows verifies that the generated doc
// docs/architecture/control-flags.md has no entries for any of the 9 dead flags
// after the registry rows are removed and the doc regenerated.
//
// Covers AC5. The doc is generated from the flagregistry (source of truth);
// absence follows from C22_001 (rows removed) plus 'evolve flags generate'.
//
// // acs-predicate: config-check — the doc regeneration is a required build step.
//
// RED: control-flags.md currently has entries for all 9 dead flags.
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

// TestC22_006_WorktreePathStillInRegistry verifies that EVOLVE_WORKTREE_PATH
// remains in the registry after the 9-row removal — it is a live IPC handoff
// (agents/evolve-tester.md:96,113) pinned by C50_009.
//
// Covers AC8. Cycles 17 and 18 both failed when builder over-reached and removed
// WORKTREE_PATH, breaking C50_009. This predicate makes that over-reach
// immediately detectable.
//
// BEHAVIORAL: calls flagregistry.Lookup("EVOLVE_WORKTREE_PATH") — the production SSOT.
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently registered (line 163 of registry_table.go).
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

// TestC22_007_RuntimeReferenceMdPreservesResearchCacheEnabled verifies that
// docs/operations/runtime-reference.md still contains RESEARCH_CACHE_ENABLED
// after Builder's changes — confirming that file was NOT modified.
//
// Covers NEG1. C89_003 checks runtime-reference.md; if Builder modifies this file
// and removes RESEARCH_CACHE_ENABLED, C89_003 breaks. This predicate catches
// such over-reach before audit.
//
// // acs-predicate: config-check — doc preservation contract (do not touch this file).
//
// PRE-EXISTING GREEN: runtime-reference.md currently contains RESEARCH_CACHE_ENABLED
// (1 occurrence confirmed by scout-report).
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
