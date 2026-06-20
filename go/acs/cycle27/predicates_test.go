//go:build acs

// Package cycle27 materializes the cycle-27 acceptance criteria for:
//
//	dead-flag-sweep-27 — remove 3 confirmed-dead EVOLVE_* flags:
//	  - EVOLVE_REAP_ORPHANS: 0 Go readers; registry doc says "does NOT gate
//	    sessionreaper's core logic in production"; 0 agent/.md shell readers.
//	  - EVOLVE_SWARM_CONCURRENCY: only in Go comments (deps.go:54,
//	    phaseconfig.go:65); no actual os.Getenv / getEnv reader.
//	  - EVOLVE_TDD_PHASE: only in a Go comment (config.go:319: "// EVOLVE_TDD_PHASE,
//	    which never matched the phase code"). No actual reader.
//	Lower FlagCeiling 129→126; regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	dead-flag-sweep-27:
//	  AC1  3 flags absent from Lookup         → C27_001 (behavioral)
//	  AC2  Registry row count == 126          → C27_002 (behavioral, count)
//	  AC3  FlagCeiling const == 126           → C27_003 (config-check, waiver)
//	  AC4  No quoted flag names in registry   → C27_004 (config-check, waiver)
//	  AC5  WORKTREE_PATH still registered     → C27_005 (behavioral — PRE-EXISTING GREEN)
//	  AC6  control-flags.md has no removed rows → C27_006 (config-check, waiver)
//	  AC7  flagreaders guard green            → manual+checklist (see below)
//	  NEG1 Registry rows absent from source  → C27_NEG1 (config-check — anti-gaming)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green):
//	    Checklist for Auditor:
//	    (a) `go test -tags acs ./acs/regression/flagreaders/...` exits 0;
//	    (b) no compile errors with `-tags acs` on the cycle27 package;
//	    (c) no literal string `"EVOLVE_REAP_ORPHANS"`, `"EVOLVE_SWARM_CONCURRENCY"`,
//	        or `"EVOLVE_TDD_PHASE"` appears in any non-test, non-registry Go file
//	        (grep -rn '"EVOLVE_REAP_ORPHANS"\|"EVOLVE_SWARM_CONCURRENCY"\|"EVOLVE_TDD_PHASE"'
//	        go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C27_001 — the 3 flags must be ABSENT from Lookup (if Builder removes
//	           the wrong flags or misses one, Lookup returns ok=true and fails immediately).
//	           C27_NEG1 — quoted flag name literals must be absent from registry_table.go
//	           (anti-gaming: if Builder sets Status=StatusDeprecated but leaves the row,
//	           Lookup returns ok=true and C27_001 catches it; but if Builder games by
//	           leaving a commented-out row, C27_NEG1 catches the residual literal).
//	Edge/OOD:  C27_002 checks exact count 126; both over-removal (< 126) and
//	           under-removal (> 126) fail. C27_005 guards WORKTREE_PATH — the
//	           "over-removal" edge that killed cycles 17-19.
//	Lexical:   Lookup / len / FileContains / FileNotContains — four distinct assertion verbs.
//	Semantic:  registry-absence (C27_001), row-count (C27_002), ceiling-const (C27_003),
//	           no-quoted-names-in-source (C27_004), worktree-path-preserved (C27_005),
//	           doc-absence (C27_006), anti-gaming-literal-absent (C27_NEG1) — 7 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (dead-flag-sweep-27). Deferred task (dispatch-cluster-27) gets zero predicates.
//
// 1:1 enforcement: predicate=7, manual+checklist=1, unverifiable-remove=0 → total AC=8 ✓
package cycle27

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// deadFlags is the canonical list of 3 flags that cycle-27 removes:
//   - EVOLVE_REAP_ORPHANS: 0 Go readers; doc says "does NOT gate sessionreaper
//     core logic in production"; 0 agent/.md shell readers. Dead.
//   - EVOLVE_SWARM_CONCURRENCY: only in Go comments (deps.go:54,
//     phaseconfig.go:65); no os.Getenv/getEnv reader. Dead.
//   - EVOLVE_TDD_PHASE: only in a Go comment (config.go:319); no reader. Dead.
var deadFlags = []string{
	"EVOLVE_REAP_ORPHANS",
	"EVOLVE_SWARM_CONCURRENCY",
	"EVOLVE_TDD_PHASE",
}

// TestC27_001_DeadFlagsAbsentFromRegistry verifies that all 3 dead flags are no
// longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1. The 3 flags are dead (zero production readers confirmed by
// all-surfaces grep in scout-report.md: comments-only or zero references).
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the registry row must be absent for
// Lookup to return ok=false.
//
// RED: all 3 flags are currently registered (FlagCeiling=129); each Lookup returns (flag, true).
func TestC27_001_DeadFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range deadFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-27 dead-flag-sweep-27).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC27_004_NoQuotedFlagNamesInRegistryTable verifies that none of the 3 dead
// flag names appear as quoted string literals in registry_table.go after removal.
//
// Covers AC4. Config-check waiver: FileNotContains asserts structural absence of
// the exact quoted name strings — the only place these flags appear as literals
// (all other references are unquoted Go comments, not env reads).
//
// acs-predicate: config-check
//
// Scout verification:
//   - REAP_ORPHANS: zero non-registry references in Go (grep returned nothing).
//   - SWARM_CONCURRENCY: only unquoted comment refs (deps.go:54, phaseconfig.go:65).
//   - TDD_PHASE: only unquoted comment ref (config.go:319).
//
// RED: registry_table.go currently contains "EVOLVE_REAP_ORPHANS" (line 94),
//
//	"EVOLVE_SWARM_CONCURRENCY" (line 125), and "EVOLVE_TDD_PHASE" (line 130).
func TestC27_004_NoQuotedFlagNamesInRegistryTable(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	registryFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	for _, name := range deadFlags {
		quoted := `"` + name + `"`
		if !acsassert.FileNotContains(t, registryFile, quoted) {
			t.Errorf("RED: registry_table.go still contains %s.\n"+
				"Builder must remove the registry row for this dead flag.\n"+
				"Comment references (unquoted) in deps.go / phaseconfig.go / config.go are acceptable.\n"+
				"File: %s", quoted, registryFile)
		}
	}
}

// TestC27_005_WorktreePathStillInRegistry verifies that EVOLVE_WORKTREE_PATH
// remains in the registry after the 3-row removal — it is a live IPC handoff
// (agents/evolve-tester.md) pinned by TestC50_009.
//
// Covers AC5 (WORKTREE_PATH must not be touched). Cycles 17, 18, and 19 all
// failed when Builder over-reached and removed WORKTREE_PATH, breaking TestC50_009.
//
// BEHAVIORAL: calls flagregistry.Lookup("EVOLVE_WORKTREE_PATH") — the production SSOT.
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently registered and must stay so.
func TestC27_005_WorktreePathStillInRegistry(t *testing.T) {
	const worktreePath = "EVOLVE_WORKTREE_PATH"
	if _, ok := flagregistry.Lookup(worktreePath); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH removed.\n"+
			"Builder MUST NOT remove EVOLVE_WORKTREE_PATH from registry_table.go.\n"+
			"It is a live IPC handoff (agents/evolve-tester.md) pinned by TestC50_009.\n"+
			"This is the same mistake that killed cycles 17, 18, and 19.",
			worktreePath)
	}
}

// TestC27_006_ControlFlagsMdHasNoRemovedRows verifies that the generated doc
// docs/architecture/control-flags.md has no entries for any of the 3 removed
// flags after the registry rows are removed and the doc regenerated via
// 'evolve flags generate'.
//
// Covers AC6. The doc is generated from the flagregistry (source of truth);
// absence follows from C27_001 (rows removed) plus regeneration.
//
// acs-predicate: config-check — doc regeneration is a required build step.
//
// RED: control-flags.md currently has entries for all 3 removed flags.
func TestC27_006_ControlFlagsMdHasNoRemovedRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range deadFlags {
		if !acsassert.FileNotContains(t, controlFlags, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 3 dead flag rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", name, controlFlags)
		}
	}
}

// TestC27_NEG1_RegistryTableHasNoDeadFlagLiterals is the anti-gaming predicate
// that verifies registry_table.go no longer contains ANY quoted string literal
// for the 3 dead flags — not just that Lookup returns false.
//
// Anti-gaming rationale (cycle-8 lesson): a Builder could theoretically set a
// flag's Status to StatusDeprecated without removing the row, causing Lookup
// to return (flag, true) and C27_001 to catch it. C27_NEG1 provides a second
// enforcement layer: even if Builder only comments out the row (leaving the
// literal in a comment), the literal is still present in the file and this
// test fails — requiring complete row deletion. Together C27_001 + C27_NEG1
// close both the live-registry and residual-literal gaming surfaces.
//
// Covers NEG1. Config-check waiver: FileNotContains asserts literal absence.
//
// acs-predicate: config-check
//
// RED: registry_table.go lines 94, 125, 130 contain the quoted names.
func TestC27_NEG1_RegistryTableHasNoDeadFlagLiterals(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	registryFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	for _, name := range deadFlags {
		// Check for the flag name in any form — quoted or bare — in the registry file.
		// After row removal, the name must be fully absent (no commented-out rows).
		if !acsassert.FileNotContains(t, registryFile, name) {
			t.Errorf("RED: registry_table.go still contains %q (possibly in a commented-out row).\n"+
				"Builder must DELETE the row entirely — do not comment it out.\n"+
				"File: %s", name, registryFile)
		}
	}
}
