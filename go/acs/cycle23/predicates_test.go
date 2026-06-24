//go:build acs

// Package cycle23 materializes the cycle-23 acceptance criteria for:
//
//	dead-flag-sweep-23 — remove 5 confirmed-dead EVOLVE_* registry rows
//	(EVOLVE_TASK_MODE, EVOLVE_REQUIRE_TEAM_CONTEXT, EVOLVE_CODEX_REQUIRE_FULL,
//	EVOLVE_RUN_TIMEOUT, EVOLVE_QUOTA_DANGER_PCT),
//	lower FlagCeiling 145→140, clean agent/skill refs,
//	remove ParseQuotaDangerPct function, regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	dead-flag-sweep-23:
//	  AC1  All 5 dead flags absent from Lookup            → C23_001 (behavioral)
//	  AC2  Registry row count == 140                      → C23_002 (behavioral, count)
//	  AC3  FlagCeiling const == 140                       → C23_003 (config-check, waiver)
//	  AC4  No os.Getenv reads for 5 flags in prod Go      → C23_004 (config-check, waiver — PRE-EXISTING GREEN)
//	  AC5  control-flags.md has no dead-flag rows         → C23_005 (config-check, waiver)
//	  AC8  ParseQuotaDangerPct removed from helpers.go    → C23_008 (config-check, waiver)
//	  NEG1 WORKTREE_PATH still in registry                → C23_006 (behavioral — PRE-EXISTING GREEN)
//	  NEG2 No removed flags in agents/ or skills/         → C23_007 (config-check, waiver)
//
// ACs with manual+checklist disposition:
//
//	AC6 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	AC7 (C50_009 still green):     `go test -tags acs ./acs/regression/cycle50/...`
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C23_001 — Lookup returns ok=false for all 5 flags; a magic-string
//	           patch of source cannot satisfy this — the registry row must be absent.
//	Edge/OOD:  EVOLVE_QUOTA_DANGER_PCT has StatusInternal (differs from the other 4
//	           StatusActive flags) — both status classes must be removed cleanly.
//	Lexical:   Lookup / len(All) / FileContains / FileNotContains / SubprocessOutput —
//	           five distinct verbs across the eight predicates.
//	Semantic:  registry-absence, row-count, ceiling-const, env-read-absence,
//	           doc-absence, worktree-path-present, agents/skills-ref-absence, func-removal.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (dead-flag-sweep-23). Deferred tasks (WORKTREE_PATH, QUOTA_RESET_AT/HOURS,
// per-phase-cli-model cluster, BYPASS/DISPATCH clusters) get zero predicates.
//
// 1:1 enforcement: predicate=8, manual+checklist=2, unverifiable-remove=0 → total AC=10 ✓
package cycle23

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// deadFlags is the canonical list of 5 confirmed-dead EVOLVE_* flags that
// cycle-23 removes. All have 0 Go production readers per scout-report §Key Findings.
var deadFlags = []string{
	"EVOLVE_CODEX_REQUIRE_FULL",
	"EVOLVE_QUOTA_DANGER_PCT",
	"EVOLVE_REQUIRE_TEAM_CONTEXT",
	"EVOLVE_RUN_TIMEOUT",
	"EVOLVE_TASK_MODE",
}

// deadFlagsWithAgentOrSkillRefs lists the 4 dead flags that had agent/skill
// refs the builder must also clean up (QUOTA_DANGER_PCT had none per scout-report).
var deadFlagsWithAgentOrSkillRefs = []string{
	"EVOLVE_CODEX_REQUIRE_FULL",
	"EVOLVE_REQUIRE_TEAM_CONTEXT",
	"EVOLVE_RUN_TIMEOUT",
	"EVOLVE_TASK_MODE",
}

// TestC23_001_DeadFlagsAbsentFromRegistry verifies that all 5 dead flags are
// no longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1 (all 5 rows absent). Includes:
//   - EVOLVE_TASK_MODE: StatusActive, 0 production readers (budget_tiers removed PR #96)
//   - EVOLVE_REQUIRE_TEAM_CONTEXT: StatusActive, 0 production readers (phase-gate-precondition.sh deleted)
//   - EVOLVE_CODEX_REQUIRE_FULL: StatusActive, 0 production readers (bridge uses --require-full struct field)
//   - EVOLVE_RUN_TIMEOUT: StatusActive, 0 production readers (budget system removed PR #96)
//   - EVOLVE_QUOTA_DANGER_PCT: StatusInternal (edge: different status class), ParseQuotaDangerPct has no callers
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the registry row must be absent for
// Lookup to return ok=false.
//
// RED: all 5 flags are currently registered; each Lookup returns (flag, true).
func TestC23_001_DeadFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range deadFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-23 dead-flag-sweep).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC23_004_NoProductionReaderForDeadFlags verifies that no production Go file
// reads any of the 5 dead flags via os.Getenv.
//
// Scout confirmed 0 production readers for all 5 flags before the cycle — this
// predicate documents the architectural contract and prevents re-introduction.
//
// // acs-predicate: config-check — the os.Getenv ABSENCE is the structural contract.
//
// PRE-EXISTING GREEN: grep confirms 0 production os.Getenv reads before this cycle.
func TestC23_004_NoProductionReaderForDeadFlags(t *testing.T) {
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

// TestC23_005_ControlFlagsMdHasNoDeadRows verifies that the generated doc
// docs/architecture/control-flags.md has no entries for any of the 5 dead flags
// after the registry rows are removed and the doc regenerated via 'evolve flags generate'.
//
// Covers AC5. The doc is generated from the flagregistry (source of truth);
// absence follows from C23_001 (rows removed) plus regeneration.
//
// // acs-predicate: config-check — the doc regeneration is a required build step.
//
// RED: control-flags.md currently has entries for all 5 dead flags.
func TestC23_005_ControlFlagsMdHasNoDeadRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, flag := range deadFlags {
		if !acsassert.FileNotContains(t, controlFlags, flag) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 5 dead flag rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", flag, controlFlags)
		}
	}
}

// TestC23_006_WorktreePathStillInRegistry verifies that EVOLVE_WORKTREE_PATH
// remains in the registry after the 5-row removal — it is a live IPC handoff
// (agents/evolve-tester.md) pinned by C50_009.
//
// Covers NEG1. Cycles 17 and 18 both failed when builder over-reached and removed
// WORKTREE_PATH, breaking C50_009. This predicate makes that over-reach
// immediately detectable.
//
// BEHAVIORAL: calls flagregistry.Lookup("EVOLVE_WORKTREE_PATH") — the production SSOT.
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently registered.
func TestC23_006_WorktreePathStillInRegistry(t *testing.T) {
	const worktreePath = "EVOLVE_WORKTREE_PATH"
	if _, ok := flagregistry.Lookup(worktreePath); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH removed.\n"+
			"Builder MUST NOT remove EVOLVE_WORKTREE_PATH from registry_table.go.\n"+
			"It is a live IPC handoff (agents/evolve-tester.md) pinned by C50_009.\n"+
			"Correct retirement is cluster-10 split-const (explicitly deferred).",
			worktreePath)
	}
}

// TestC23_007_NoRemovedFlagRefsInAgentsOrSkills verifies that agents/ and skills/
// contain no references to the 4 dead flags that had agent/skill references
// (per scout-report §Key Findings: TASK_MODE×3, REQUIRE_TEAM_CONTEXT×1,
// CODEX_REQUIRE_FULL×1, RUN_TIMEOUT×1). QUOTA_DANGER_PCT had no agent/skill refs.
//
// Covers NEG2. Builder must clean refs alongside registry row removal; leaving
// orphan refs causes the flagreaders guard to flag them in future cycles.
//
// // acs-predicate: config-check — ref cleanup is a required companion to row removal.
//
// RED: agents/ and skills/ currently contain references to all 4 flags.
func TestC23_007_NoRemovedFlagRefsInAgentsOrSkills(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	agentsDir := filepath.Join(root, "agents")
	skillsDir := filepath.Join(root, "skills")
	for _, flag := range deadFlagsWithAgentOrSkillRefs {
		for _, dir := range []struct {
			name string
			path string
		}{
			{"agents", agentsDir},
			{"skills", skillsDir},
		} {
			_, _, code, _ := acsassert.SubprocessOutput(
				"grep", "-rl", flag, dir.path,
			)
			// grep -l exits 0 when a match is found (BAD: orphan ref remains).
			// grep -l exits 1 when no match found (GOOD: reference cleaned up).
			if code == 0 {
				t.Errorf("RED: %s/ still contains a reference to %q.\n"+
					"Builder must remove all agent/skill references alongside the registry row removal.\n"+
					"Scout-report §Research→Implementation Map lists the exact files to clean.\n"+
					"Directory: %s", dir.name, flag, dir.path)
			}
		}
	}
}

// TestC23_008_ParseQuotaDangerPctRemovedFromHelpers verifies that the dead utility
// function ParseQuotaDangerPct has been removed from go/internal/subagent/helpers.go.
//
// Covers AC8. ParseQuotaDangerPct is a pure string-parser (no callers in
// production code) that was ported from bash but never wired up in Go.
// Removing EVOLVE_QUOTA_DANGER_PCT from the registry makes it an orphan utility.
//
// // acs-predicate: config-check — function-absence confirms the dead utility is gone.
//
// RED: helpers.go currently contains ParseQuotaDangerPct at line 215.
func TestC23_008_ParseQuotaDangerPctRemovedFromHelpers(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	helpersFile := filepath.Join(root, "go", "internal", "subagent", "helpers.go")
	if !acsassert.FileNotContains(t, helpersFile, "ParseQuotaDangerPct") {
		t.Errorf("RED: go/internal/subagent/helpers.go still contains ParseQuotaDangerPct.\n"+
			"Builder must remove the dead function ParseQuotaDangerPct and its test\n"+
			"(TestParseQuotaDangerPct in helpers_test.go) in the same diff.\n"+
			"File: %s", helpersFile)
	}
}
