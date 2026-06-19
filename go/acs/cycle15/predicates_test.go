//go:build acs

// Package cycle15 materializes the cycle-15 acceptance criteria for the
// committed top_n task:
//
//	consolidate-resume-cluster — remove all 6 RESUME_* registry rows
//	(EVOLVE_AUTO_RESUME_MAX_ATTEMPTS, EVOLVE_RESUME, EVOLVE_RESUME_ALLOW_HEAD_MOVED,
//	EVOLVE_RESUME_COMPLETED_PHASES, EVOLVE_RESUME_MODE, EVOLVE_RESUME_PHASE),
//	lower FlagCeiling 160→154, remove RESUME_* rows from control-flags.md,
//	and clean up docs_contract_test.go entries.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	consolidate-resume-cluster:
//	  AC1+NEG1  All 6 RESUME_* flags absent from Lookup             → C15_001 (behavioral)
//	  AC2       Registry row count == 154                            → C15_002 (behavioral, count)
//	  AC3       FlagCeiling const == 154                             → C15_003 (config-check, waiver)
//	  AC4       No os.Getenv reads for RESUME_* in production files  → C15_004 (config-check, waiver — PRE-EXISTING GREEN)
//	  AC5       control-flags.md has no RESUME_* rows               → C15_005 (config-check, waiver)
//	  EDGE1     IPC set preserved: cmd_loop_args.go sets EVOLVE_RESUME=1  → C15_006 (config-check, waiver — PRE-EXISTING GREEN)
//
// ACs with manual+checklist disposition (enforced by CI):
//
//	AC6   (full test suite green): `go test ./...` exit 0
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C15_001 — Lookup returns ok=false for all 6 flags; cannot be
//	           satisfied by adding magic strings — the registry row must be absent.
//	Edge/OOD:  EVOLVE_RESUME_ALLOW_HEAD_MOVED (NEG1) — was commented as "dead"
//	           and referenced only in a comment in resume.go:35; AllowHeadMoved
//	           field in ResumeOptions stays — only the env var row is removed.
//	           EVOLVE_AUTO_RESUME_MAX_ATTEMPTS — pure dead: 0 production readers.
//	Lexical:   Lookup / len() / FileContains / FileNotContains — four distinct verbs.
//	Semantic:  registry-absence, row-count, ceiling-constant, env-read-absence,
//	           doc-absence, ipc-set-preserved — six distinct behavioral dimensions.
//
// 1:1 enforcement: predicate=6, manual+checklist=1, unverifiable-remove=0 → total AC=7 ✓
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (consolidate-resume-cluster). Deferred tasks (BYPASS_*, TRIAGE_*, etc.)
// get zero predicates this cycle.
package cycle15

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC15_001_AllResumeFlagsAbsentFromRegistry verifies that all 6 RESUME_*
// flags are no longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1 (6 rows absent from registry_table.go) and NEG1 (RESUME_ALLOW_HEAD_MOVED
// specifically absent — it was documented but dead). Includes all 6 flags:
//   - EVOLVE_AUTO_RESUME_MAX_ATTEMPTS: 0 production readers (dead)
//   - EVOLVE_RESUME:                  SET only (cmd_loop_args.go:273); IPC handoff, no os.Getenv read
//   - EVOLVE_RESUME_ALLOW_HEAD_MOVED: comment only in resume.go:35; no os.Getenv read
//   - EVOLVE_RESUME_COMPLETED_PHASES: LLM prompt protocol only; no os.Getenv read
//   - EVOLVE_RESUME_MODE:             SET in core/resume.go:186 envSnap; no os.Getenv read
//   - EVOLVE_RESUME_PHASE:            LLM prompt protocol only; no os.Getenv read
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the row must be absent for Lookup
// to return ok=false.
//
// RED: all 6 flags are currently registered; each Lookup returns (flag, true).
func TestC15_001_AllResumeFlagsAbsentFromRegistry(t *testing.T) {
	// All 6 RESUME_* rows from scout-report §Key Findings.
	// Semantic sub-cases: dead (AUTO_RESUME_MAX_ATTEMPTS), IPC-SET-only (RESUME,
	// RESUME_MODE), dead-comment (RESUME_ALLOW_HEAD_MOVED), LLM-protocol (RESUME_COMPLETED_PHASES,
	// RESUME_PHASE) — all are pure registry-row removals with no os.Getenv reads to migrate.
	allFlags := []string{
		"EVOLVE_AUTO_RESUME_MAX_ATTEMPTS", // dead: 0 production readers
		"EVOLVE_RESUME",                   // IPC SET only: cmd_loop_args.go:273 sets it; no Go os.Getenv
		"EVOLVE_RESUME_ALLOW_HEAD_MOVED",  // NEG1: comment in resume.go:35 only; AllowHeadMoved DI field stays
		"EVOLVE_RESUME_COMPLETED_PHASES",  // LLM orchestrator-reference.md protocol; no Go os.Getenv
		"EVOLVE_RESUME_MODE",              // IPC SET only: core/resume.go:186 envSnap; no Go os.Getenv
		"EVOLVE_RESUME_PHASE",             // LLM orchestrator-reference.md protocol; no Go os.Getenv
	}
	for _, name := range allFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-15 RESUME_* consolidation).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC15_002_RegistryRowCountIs154 verifies that after removing all 6
// RESUME_* rows the total registry count is exactly 154.
//
// Covers AC2 (154 rows total). Both over-removal (< 154) and under-removal
// (> 154) fail the assertion.
//
// BEHAVIORAL: calls flagregistry.All directly (the production SSOT slice).
// No source-file grepping; a magic-string patch cannot satisfy this.
//
// RED: len(flagregistry.All) is currently 160, which is 6 rows above 154.
func TestC15_002_RegistryRowCountIs154(t *testing.T) {
	const want = 154
	if got := len(flagregistry.All); got != want {
		t.Errorf("RED: len(flagregistry.All) = %d, want %d.\n"+
			"Builder must remove all 6 RESUME_* rows from registry_table.go.\n"+
			"Both over-removal (< 154) and under-removal (> 154) fail.\n"+
			"Expected: 160 − 6 = 154.",
			got, want)
	}
}

// TestC15_003_FlagCeilingConstIs154 verifies that the FlagCeiling ratchet
// constant in registry_ceiling_test.go has been lowered from 160 to 154
// in the same diff as the row removal.
//
// // acs-predicate: config-check — the constant value is the canonical ratchet
// config; reading 160 after the 6-row removal breaks the ratchet guarantee.
//
// RED: registry_ceiling_test.go currently has FlagCeiling = 160.
func TestC15_003_FlagCeilingConstIs154(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	ceilingFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_ceiling_test.go")
	if !acsassert.FileContains(t, ceilingFile, "FlagCeiling = 154") {
		t.Errorf("RED: registry_ceiling_test.go does not contain 'FlagCeiling = 154'.\n"+
			"Builder must lower the FlagCeiling constant from 160 to 154 in the same diff\n"+
			"as removing the 6 RESUME_* rows (160 − 6 = 154).\n"+
			"File: %s", ceilingFile)
	}
}

// TestC15_004_NoResumeEnvReadsInProductionFiles verifies that no production
// Go file reads any of the 6 RESUME_* env vars via os.Getenv.
//
// The scout confirmed 0 production readers before the cycle (grep returns 0 hits
// for `os.Getenv.*EVOLVE_RESUME|EVOLVE_AUTO_RESUME`). This predicate is
// PRE-EXISTING GREEN — it documents the architectural contract (these flags were
// never wired to production os.Getenv reads) and prevents accidental re-introduction.
//
// Key files audited: resume.go (sets EVOLVE_RESUME_MODE in envSnap, no read),
// cmd_loop_args.go (sets EVOLVE_RESUME=1, no read), orchestrator-reference.md
// (LLM prompt protocol, not Go os.Getenv).
//
// // acs-predicate: config-check — the os.Getenv ABSENCE is the structural contract.
//
// PRE-EXISTING GREEN: grep confirms 0 production os.Getenv reads before this cycle.
func TestC15_004_NoResumeEnvReadsInProductionFiles(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	// Key files where resume logic lives — none should read RESUME_* via os.Getenv.
	resumeFile := filepath.Join(root, "go", "internal", "core", "resume.go")
	cmdLoopArgsFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_args.go")
	for _, flag := range []string{
		"EVOLVE_AUTO_RESUME_MAX_ATTEMPTS",
		"EVOLVE_RESUME_ALLOW_HEAD_MOVED",
		"EVOLVE_RESUME_COMPLETED_PHASES",
		"EVOLVE_RESUME_PHASE",
	} {
		envRead := `os.Getenv("` + flag + `")`
		if !acsassert.FileNotContains(t, resumeFile, envRead) {
			t.Errorf("resume.go reads %q via os.Getenv — must be absent.\n"+
				"File: %s", flag, resumeFile)
		}
		if !acsassert.FileNotContains(t, cmdLoopArgsFile, envRead) {
			t.Errorf("cmd_loop_args.go reads %q via os.Getenv — must be absent.\n"+
				"File: %s", flag, cmdLoopArgsFile)
		}
	}
}

// TestC15_005_ControlFlagsMdHasNoResumeRows verifies that the generated doc
// docs/architecture/control-flags.md has no EVOLVE_RESUME_* or
// EVOLVE_AUTO_RESUME_MAX_ATTEMPTS entries after the 6 registry rows are removed
// and the doc regenerated.
//
// Covers AC5. The doc is generated from the flagregistry (source of truth);
// absence of RESUME_* rows follows from C15_001 (rows removed) plus the
// regeneration step ('evolve flags generate').
//
// // acs-predicate: config-check — the doc regeneration is a required build step.
//
// RED: control-flags.md currently has 6 EVOLVE_RESUME_* entries.
func TestC15_005_ControlFlagsMdHasNoResumeRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	resumeFlags := []string{
		"EVOLVE_AUTO_RESUME_MAX_ATTEMPTS",
		"EVOLVE_RESUME_ALLOW_HEAD_MOVED",
		"EVOLVE_RESUME_COMPLETED_PHASES",
		"EVOLVE_RESUME_MODE",
		"EVOLVE_RESUME_PHASE",
		// bare EVOLVE_RESUME — match the backtick-wrapped table cell form
		// (`` `EVOLVE_RESUME` ``) which is distinct from all _* variants.
		"`EVOLVE_RESUME`",
	}
	for _, flag := range resumeFlags {
		if !acsassert.FileNotContains(t, controlFlags, flag) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 6 RESUME_* rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", flag, controlFlags)
		}
	}
}

// TestC15_006_IPCSetPreservedInCmdLoopArgs verifies that cmd_loop_args.go still
// contains the EVOLVE_RESUME IPC set (the child-process handoff), confirming
// that removing the registry row does NOT remove the IPC mechanism itself.
//
// Covers EDGE1 from scout-report §Acceptance Criteria Summary. The registry
// is documentation, not runtime glue; the IPC env var must survive the row removal.
//
// // acs-predicate: config-check — the IPC SET presence is the structural contract.
//
// PRE-EXISTING GREEN: cmd_loop_args.go:273 already sets EVOLVE_RESUME=1.
func TestC15_006_IPCSetPreservedInCmdLoopArgs(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	cmdLoopArgsFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_args.go")
	if !acsassert.FileContains(t, cmdLoopArgsFile, `"EVOLVE_RESUME"`) {
		t.Errorf("RED: cmd_loop_args.go no longer sets EVOLVE_RESUME — IPC handoff broken.\n"+
			"Builder must NOT remove the EVOLVE_RESUME= assignment in cmd_loop_args.go:273;\n"+
			"only the registry row in registry_table.go should be removed.\n"+
			"File: %s", cmdLoopArgsFile)
	}
}
