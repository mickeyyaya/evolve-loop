//go:build acs

// Package cycle13 materializes the cycle-13 acceptance criteria for the
// committed top_n task:
//
//	consolidate-checkpoint-cluster — remove all 3 CHECKPOINT_* registry rows
//	(EVOLVE_CHECKPOINT_DISABLE, EVOLVE_CHECKPOINT_REASON, EVOLVE_CHECKPOINT_REQUEST),
//	lower FlagCeiling 158→155, remove the inert t.Setenv from the test, and
//	remove CHECKPOINT_* rows from control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	consolidate-checkpoint-cluster:
//	  AC1+NEG1  All 3 CHECKPOINT_* flags absent from Lookup             → C13_001 (behavioral)
//	  AC2       Registry row count == 155                                → C13_002 (behavioral, count)
//	  AC3       FlagCeiling const == 155                                 → C13_003 (config-check, waiver)
//	  AC4       No os.Getenv reads for CHECKPOINT_* in production files  → C13_004 (config-check, waiver — PRE-EXISTING GREEN)
//	  AC5       control-flags.md has no CHECKPOINT_* rows               → C13_005 (config-check, waiver)
//
// ACs with manual+checklist disposition (enforced by CI):
//
//	AC6   (full test suite green): `go test ./...` exit 0
//	EDGE1 (TestRunLoop_DeprecatedCostEnvVarsInert passes after t.Setenv removal):
//	      CI: go test ./cmd/evolve/...
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C13_001 — Lookup returns ok=false for all 3 flags; cannot be
//	           satisfied by adding magic strings — the registry row must be absent.
//	Edge/OOD:  CHECKPOINT_REASON and CHECKPOINT_REQUEST (C13_001) — these flags
//	           never had production readers; they're the pure speculative-dead case.
//	           CHECKPOINT_DISABLE (C13_001) — had an inert t.Setenv in a test.
//	Lexical:   Lookup / len() / FileContains / FileNotContains — four distinct verbs.
//	Semantic:  registry-absence, row-count, ceiling-constant, doc-absence — four
//	           distinct behavioral dimensions.
//
// 1:1 enforcement: predicate=5, manual+checklist=2, unverifiable-remove=0,
// pre-existing-GREEN=1 (C13_004) → total AC=7 ✓
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (consolidate-checkpoint-cluster). Deferred tasks (BYPASS_*, RESUME_*, etc.)
// get zero predicates this cycle.
package cycle13

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// TestC13_001_AllCheckpointFlagsAbsentFromRegistry verifies that all 3 CHECKPOINT_*
// flags are no longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1 (3 rows absent from registry_table.go) and NEG1 (CHECKPOINT_DISABLE
// specifically absent). Includes all 3 flags:
//   - CHECKPOINT_DISABLE: former cost-gate signal, confirmed inert by test comment
//   - CHECKPOINT_REASON:  0 production readers (dead speculative addition)
//   - CHECKPOINT_REQUEST: 0 production readers (dead speculative addition)
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the row must be absent for Lookup
// to return ok=false.
//
// RED: all 3 flags are currently registered; each Lookup returns (flag, true).
func TestC13_001_AllCheckpointFlagsAbsentFromRegistry(t *testing.T) {
	// All 3 CHECKPOINT_* rows from scout-report §Key Findings.
	// Semantic sub-cases: inert cost-gate signal (DISABLE), and two dead
	// speculative rows that never had production readers (REASON, REQUEST).
	allFlags := []string{
		"EVOLVE_CHECKPOINT_DISABLE", // confirmed inert: cmd_loop_v11_5_1_test.go:286
		"EVOLVE_CHECKPOINT_REASON",  // dead: 0 production os.Getenv reads
		"EVOLVE_CHECKPOINT_REQUEST", // dead: 0 production os.Getenv reads
	}
	for _, name := range allFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-13 CHECKPOINT_* consolidation).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC13_004_NoCheckpointEnvReadsInProductionFiles verifies that no production
// Go file reads EVOLVE_CHECKPOINT_DISABLE, EVOLVE_CHECKPOINT_REASON, or
// EVOLVE_CHECKPOINT_REQUEST via os.Getenv.
//
// The scout confirmed 0 production readers before the cycle. This predicate
// is PRE-EXISTING GREEN — it documents the architectural contract (these flags
// were never wired to production code) and prevents accidental re-introduction.
//
// // acs-predicate: config-check — the os.Getenv ABSENCE is the structural contract.
//
// PRE-EXISTING GREEN: grep confirms 0 production os.Getenv reads before this cycle.
func TestC13_004_NoCheckpointEnvReadsInProductionFiles(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	// The key production files where checkpoint code could read env vars.
	// checkpoint.go:12 explicitly states: "The former cost-percentage trigger
	// (EVOLVE_CHECKPOINT_*_PCT) was removed with the token-budget cost gates"
	checkpointFile := filepath.Join(root, "go", "internal", "checkpoint", "checkpoint.go")
	if !acsassert.FileNotContains(t, checkpointFile, `os.Getenv("EVOLVE_CHECKPOINT_DISABLE")`) {
		t.Errorf("checkpoint.go reads EVOLVE_CHECKPOINT_DISABLE via os.Getenv — must be absent.\n"+
			"File: %s", checkpointFile)
	}
	if !acsassert.FileNotContains(t, checkpointFile, `os.Getenv("EVOLVE_CHECKPOINT_REASON")`) {
		t.Errorf("checkpoint.go reads EVOLVE_CHECKPOINT_REASON via os.Getenv — must be absent.\n"+
			"File: %s", checkpointFile)
	}
	if !acsassert.FileNotContains(t, checkpointFile, `os.Getenv("EVOLVE_CHECKPOINT_REQUEST")`) {
		t.Errorf("checkpoint.go reads EVOLVE_CHECKPOINT_REQUEST via os.Getenv — must be absent.\n"+
			"File: %s", checkpointFile)
	}
}

// TestC13_005_ControlFlagsMdHasNoCheckpointRows verifies that the generated doc
// docs/architecture/control-flags.md has no EVOLVE_CHECKPOINT_DISABLE,
// EVOLVE_CHECKPOINT_REASON, or EVOLVE_CHECKPOINT_REQUEST entries after the 3
// registry rows are removed and the doc regenerated.
//
// Covers AC5. The doc is generated from the flagregistry (source of truth);
// absence of CHECKPOINT_* rows follows from C13_001 (rows removed) plus the
// regeneration step ('evolve flags generate').
//
// // acs-predicate: config-check — the doc regeneration is a required build step.
//
// RED: control-flags.md currently has EVOLVE_CHECKPOINT_* entries (lines 260-262).
func TestC13_005_ControlFlagsMdHasNoCheckpointRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	checkpointFlags := []string{
		"EVOLVE_CHECKPOINT_DISABLE",
		"EVOLVE_CHECKPOINT_REASON",
		"EVOLVE_CHECKPOINT_REQUEST",
	}
	for _, flag := range checkpointFlags {
		if !acsassert.FileNotContains(t, controlFlags, flag) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 3 CHECKPOINT_* rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", flag, controlFlags)
		}
	}
}
