//go:build acs

// Package cycle24 materializes the cycle-24 acceptance criteria for:
//
//	per-phase-cli-model-profiles — remove 5 per-phase agent config flags
//	(EVOLVE_AUDITOR_CLI, EVOLVE_TDD_ENGINEER_CLI, EVOLVE_TDD_ENGINEER_MODEL,
//	EVOLVE_BUILD_PERMISSION_MODE, EVOLVE_TDD_ENGINEER_PERMISSION_MODE)
//	by deleting os.Getenv tier-2 reads and consolidating to Profile SSOT.
//	Lower FlagCeiling 140→135, regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	per-phase-cli-model-profiles:
//	  AC1  5 flags absent from Lookup             → C24_001 (behavioral)
//	  AC2  Registry row count == 135              → C24_002 (behavioral, count)
//	  AC3  FlagCeiling const == 135               → C24_003 (config-check, waiver)
//	  AC4  No prod readers for removed flags      → C24_004 (config-check, waiver)
//	  AC5  control-flags.md has no removed rows   → C24_005 (config-check, waiver)
//	  AC6  WORKTREE_PATH still in registry        → C24_006 (behavioral — PRE-EXISTING GREEN)
//	  AC8  llmroute skips os.Getenv for CLI+MODEL → C24_008 (behavioral)
//	  NEG1 profile.CLI honored (preserved)        → C24_NEG1 (behavioral — PRE-EXISTING GREEN)
//	  NEG2 runtime-reference.md no removed rows   → C24_NEG2 (config-check, waiver — PRE-EXISTING GREEN)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor: (a) no compile errors with -tags acs; (b) exit 0 from the
//	    flagreaders suite; (c) no stale EVOLVE_* references to the 5 removed flags in
//	    the flagreaders scan results.
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C24_008 — sets EVOLVE_AUDITOR_CLI and EVOLVE_TDD_ENGINEER_MODEL in the OS env
//	           (via t.Setenv), calls llmroute.Resolve with empty reqEnv, and asserts the
//	           sentinel OS values are NOT picked up. This is the strongest anti-no-op signal:
//	           the only way to satisfy it is to remove the os.Getenv tier-2 from resolvePrimary
//	           and resolveModel — a magic-string source edit cannot satisfy it.
//	Edge/OOD:  C24_001 tests ALL 5 flags (mix of StatusInternal status; all must be absent).
//	           C24_004 covers both runner.go (PERMISSION_MODE) and observer (phaseCLI) call sites.
//	Lexical:   Lookup / len / FileContains / FileNotContains / CountInGoFunc /
//	           t.Setenv / llmroute.Resolve — seven distinct verbs.
//	Semantic:  registry-absence, row-count, ceiling-const, structural-reader-absence,
//	           doc-absence, worktree-path-preserved, llmroute-env-bypass,
//	           profile-honored, runtime-reference-absence — 9 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (per-phase-cli-model-profiles). Deferred tasks (INTERACTIVE_POLICY flags,
// ROUTER_CLI/MODEL, Workflow Defaults cluster) get zero predicates.
//
// 1:1 enforcement: predicate=9, manual+checklist=1, unverifiable-remove=0 → total AC=10 ✓
package cycle24

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// removedFlags is the canonical list of 5 per-phase agent config flags that
// cycle-24 removes by migrating from os.Getenv tier-2 to Profile SSOT tier-3.
// Covers two reader surfaces: llmroute (CLI+MODEL) and runner.go (PERMISSION_MODE).
var removedFlags = []string{
	"EVOLVE_AUDITOR_CLI",
	"EVOLVE_BUILD_PERMISSION_MODE",
	"EVOLVE_TDD_ENGINEER_CLI",
	"EVOLVE_TDD_ENGINEER_MODEL",
	"EVOLVE_TDD_ENGINEER_PERMISSION_MODE",
}

// TestC24_001_DeadFlagsAbsentFromRegistry verifies that all 5 per-phase agent
// config flags are no longer registered after Builder removes their rows from
// registry_table.go.
//
// Covers AC1. Flags span two reader surfaces:
//   - EVOLVE_AUDITOR_CLI, EVOLVE_TDD_ENGINEER_CLI: llmroute.resolvePrimary
//   - EVOLVE_TDD_ENGINEER_MODEL: llmroute.resolveModel
//   - EVOLVE_BUILD_PERMISSION_MODE, EVOLVE_TDD_ENGINEER_PERMISSION_MODE: runner.go:~445
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the registry row must be absent for
// Lookup to return ok=false.
//
// RED: all 5 flags are currently registered; each Lookup returns (flag, true).
func TestC24_001_DeadFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range removedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-24 per-phase-cli-model-profiles).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC24_002_RegistryRowCountIs135 verifies that after removing all 5 rows the
// total registry count is exactly 135.
//
// Covers AC2. Both over-removal (< 135) and under-removal (> 135) fail.
//
// BEHAVIORAL: calls len(flagregistry.All) directly (the production SSOT slice).
// No source-file grepping; a magic-string patch cannot satisfy this.
//
// RED: len(flagregistry.All) is currently 140, which is 5 rows above 135.
func TestC24_002_RegistryRowCountIs135(t *testing.T) {
	const want = 135
	if got := len(flagregistry.All); got != want {
		t.Errorf("RED: len(flagregistry.All) = %d, want %d.\n"+
			"Builder must remove all 5 per-phase config flag rows from registry_table.go.\n"+
			"Both over-removal (< 135) and under-removal (> 135) fail.\n"+
			"Expected: 140 − 5 = 135.",
			got, want)
	}
}

// TestC24_003_FlagCeilingConstIs135 verifies that the FlagCeiling ratchet
// constant in registry_ceiling_test.go has been lowered from 140 to 135
// in the same diff as the 5-row removal.
//
// // acs-predicate: config-check — the constant value is the canonical ratchet;
// keeping 140 after the 5-row removal breaks the ratchet guarantee.
//
// RED: registry_ceiling_test.go currently has FlagCeiling = 140.
func TestC24_003_FlagCeilingConstIs135(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	ceilingFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_ceiling_test.go")
	if !acsassert.FileContains(t, ceilingFile, "FlagCeiling = 135") {
		t.Errorf("RED: registry_ceiling_test.go does not contain 'FlagCeiling = 135'.\n"+
			"Builder must lower the FlagCeiling constant from 140 to 135 in the same diff\n"+
			"as removing the 5 per-phase config flag rows (140 − 5 = 135).\n"+
			"File: %s", ceilingFile)
	}
}

// TestC24_004_NoProductionReaderForRemovedFlags verifies that the os.Getenv
// tier-2 read sites for the 5 removed flags have been deleted from production code:
//   - runner.go: no longer calls envchain.Resolve(envchain.PhaseEnvKey(...)) for PERMISSION_MODE
//   - observer/core_adapter.go: phaseCLI function no longer calls a.envGet (os.Getenv wrapper)
//
// Covers AC4. The behavioral proof for llmroute CLI+MODEL is in C24_008.
//
// // acs-predicate: config-check — the structural removal of envchain.Resolve call sites
// that wired PERMISSION_MODE and observer phaseCLI to os.Getenv is the required fix.
//
// RED:
//   - runner.go:~445 has envchain.Resolve(envchain.PhaseEnvKey(profileName, "PERMISSION_MODE"), ...)
//   - phaseCLI in core_adapter.go calls a.envGet(k) inside its look closure
func TestC24_004_NoProductionReaderForRemovedFlags(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)

	// Check 1: runner.go no longer uses envchain.Resolve(envchain.PhaseEnvKey for per-phase config.
	// The only envchain.Resolve(envchain.PhaseEnvKey site in runner.go is the PERMISSION_MODE reader.
	// After fix: replaced with reqEnv-first then profile.PermissionMode lookup.
	runnerFile := filepath.Join(root, "go", "internal", "phases", "runner", "runner.go")
	if !acsassert.FileNotContains(t, runnerFile, "envchain.Resolve(envchain.PhaseEnvKey") {
		t.Errorf("RED: runner.go still calls envchain.Resolve(envchain.PhaseEnvKey(...)).\n"+
			"Builder must replace the PERMISSION_MODE envchain.Resolve at line ~445\n"+
			"with reqEnv-first then profile.PermissionMode lookup (os.Getenv tier removed).\n"+
			"File: %s", runnerFile)
	}

	// Check 2: observer/core_adapter.go phaseCLI no longer calls a.envGet (os.Getenv wrapper).
	// After fix: the look closure in phaseCLI uses only req.Env map lookup (reqEnv tier-1 only).
	coreAdapterFile := filepath.Join(root, "go", "internal", "adapters", "observer", "core_adapter.go")
	count, err := acsassert.CountInGoFunc(coreAdapterFile, "phaseCLI", "a.envGet")
	if err != nil {
		t.Fatalf("CountInGoFunc(phaseCLI, a.envGet): %v", err)
	}
	if count > 0 {
		t.Errorf("RED: phaseCLI in core_adapter.go calls a.envGet %d time(s).\n"+
			"Builder must remove the a.envGet(k) call from the look closure inside phaseCLI\n"+
			"so only req.Env[k] (reqEnv tier-1) is consulted for per-agent CLI.\n"+
			"File: %s", count, coreAdapterFile)
	}
}

// TestC24_005_ControlFlagsMdHasNoRemovedRows verifies that the generated doc
// docs/architecture/control-flags.md has no entries for any of the 5 removed flags
// after the registry rows are removed and the doc regenerated via 'evolve flags generate'.
//
// Covers AC5. The doc is generated from the flagregistry (source of truth);
// absence follows from C24_001 (rows removed) plus regeneration.
//
// // acs-predicate: config-check — the doc regeneration is a required build step.
//
// RED: control-flags.md currently has entries for all 5 removed flags.
func TestC24_005_ControlFlagsMdHasNoRemovedRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, flag := range removedFlags {
		if !acsassert.FileNotContains(t, controlFlags, flag) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 5 per-phase config flag rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", flag, controlFlags)
		}
	}
}

// TestC24_006_WorktreePathStillInRegistry verifies that EVOLVE_WORKTREE_PATH
// remains in the registry after the 5-row removal — it is a live IPC handoff
// (agents/evolve-tester.md) pinned by C50_009.
//
// Covers AC6 (WORKTREE_PATH must not be touched). Cycles 17 and 18 both failed
// when builder over-reached and removed WORKTREE_PATH, breaking C50_009.
//
// BEHAVIORAL: calls flagregistry.Lookup("EVOLVE_WORKTREE_PATH") — the production SSOT.
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently registered.
func TestC24_006_WorktreePathStillInRegistry(t *testing.T) {
	const worktreePath = "EVOLVE_WORKTREE_PATH"
	if _, ok := flagregistry.Lookup(worktreePath); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH removed.\n"+
			"Builder MUST NOT remove EVOLVE_WORKTREE_PATH from registry_table.go.\n"+
			"It is a live IPC handoff (agents/evolve-tester.md) pinned by C50_009.\n"+
			"This is the same mistake that killed cycles 17 and 18.",
			worktreePath)
	}
}

// TestC24_008_LlmrouteSkipsEnvForPerAgentCLI verifies that llmroute.Resolve
// no longer reads os.Getenv for the per-agent CLI or MODEL env vars when the
// value is absent from reqEnv.
//
// Covers AC8. After the fix: resolvePrimary uses env[perAgentKey] (reqEnv map
// lookup only), not envchain.Resolve(perAgentKey, env, ...) which goes through
// os.Getenv. Same change applied to resolveModel for EVOLVE_<AGENT>_MODEL.
//
// BEHAVIORAL: sets per-agent flags in the OS environment via t.Setenv, then calls
// llmroute.Resolve with an empty reqEnv map and nil profile — the only source that
// could supply values is os.Getenv. Asserts the OS values are NOT honored.
//
// RED: envchain.Resolve(perAgentKey, env, ...) reads os.Getenv → picks up sentinel
// values → Candidates[0] == sentinelCLI and Model == sentinelModel → test fails.
//
// GREEN: env[perAgentKey] lookup → not in map → falls through to profile (nil)
// → default ("claude-tmux" / "balanced") → sentinel values absent → test passes.
func TestC24_008_LlmrouteSkipsEnvForPerAgentCLI(t *testing.T) {
	const (
		sentinelCLI   = "sentinel-cli-c24-008-should-not-be-used"
		sentinelModel = "sentinel-model-c24-008-should-not-be-used"
	)

	// Install sentinel values in the OS environment.
	// envchain.Resolve (current code) reads os.Getenv as tier-2 and will pick
	// these up. The fixed code uses only env[key] (reqEnv map) and will not.
	t.Setenv("EVOLVE_AUDITOR_CLI", sentinelCLI)
	t.Setenv("EVOLVE_TDD_ENGINEER_MODEL", sentinelModel)

	// auditor CLI: call with empty reqEnv and nil profile.
	auditorPlan := llmroute.Resolve(
		"auditor", "audit", "balanced",
		map[string]string{}, // empty reqEnv — only os.Getenv could supply EVOLVE_AUDITOR_CLI
		nil,                 // nil profile
		nil,                 // nil AutoModel
		nil,                 // nil policy.Pin
	)
	if len(auditorPlan.Candidates) > 0 && auditorPlan.Candidates[0] == sentinelCLI {
		t.Errorf("RED: llmroute.Resolve picked up EVOLVE_AUDITOR_CLI=%q from os.Getenv.\n"+
			"envchain.Resolve(perAgentKey, env, ...) tier-2 is still active in resolvePrimary.\n"+
			"Builder must replace envchain.Resolve(perAgentKey, env, ...) with env[perAgentKey]\n"+
			"(reqEnv-only map lookup) — os.Getenv tier removed for per-agent CLI.\n"+
			"Got Candidates: %v", sentinelCLI, auditorPlan.Candidates)
	}

	// tdd-engineer MODEL: call with empty reqEnv and nil profile.
	tddPlan := llmroute.Resolve(
		"tdd-engineer", "tdd", "balanced",
		map[string]string{}, // empty reqEnv — only os.Getenv could supply EVOLVE_TDD_ENGINEER_MODEL
		nil,                 // nil profile
		nil,                 // nil AutoModel
		nil,                 // nil policy.Pin
	)
	if tddPlan.Model == sentinelModel {
		t.Errorf("RED: llmroute.Resolve picked up EVOLVE_TDD_ENGINEER_MODEL=%q from os.Getenv.\n"+
			"envchain.Resolve(PhaseEnvKey(agent, \"MODEL\"), env, ...) tier-2 still active in resolveModel.\n"+
			"Builder must replace envchain.Resolve(PhaseEnvKey(agent, \"MODEL\"), env, ...) with\n"+
			"reqEnv-only lookup in resolveModel — os.Getenv tier removed for per-agent MODEL.\n"+
			"Got Model: %q", sentinelModel, tddPlan.Model)
	}
}

// TestC24_NEG1_ProfileCLIIsHonored verifies that profile.CLI is still honored
// as the per-agent CLI source after the os.Getenv tier-2 is removed.
//
// Covers NEG1 (capability preservation). The profile tier-3 must remain operative:
// when reqEnv is empty and OS env has no override, profile.CLI is the resolved primary.
//
// BEHAVIORAL: calls llmroute.Resolve with a profile.CLI="codex-tmux" and empty reqEnv,
// asserting the profile is the primary source.
//
// PRE-EXISTING GREEN: profile.CLI is already tier-3 and works in the current code.
func TestC24_NEG1_ProfileCLIIsHonored(t *testing.T) {
	prof := &profiles.Profile{CLI: "codex-tmux"}
	plan := llmroute.Resolve(
		"auditor", "audit", "balanced",
		map[string]string{}, // empty reqEnv
		prof,
		nil, // nil AutoModel
		nil, // nil policy.Pin
	)
	if len(plan.Candidates) == 0 || plan.Candidates[0] != "codex-tmux" {
		t.Errorf("RED: llmroute.Resolve did not honor profile.CLI=%q — got Candidates=%v.\n"+
			"Profile tier-3 must remain operative after the os.Getenv tier-2 removal.\n"+
			"Removing os.Getenv must NOT degrade the profile → CLI routing path.",
			"codex-tmux", plan.Candidates)
	}
	if plan.PrimarySource != "profile.auditor.cli" {
		t.Errorf("RED: expected PrimarySource=%q, got %q.\n"+
			"The source label must reflect profile provenance after the tier-2 removal.",
			"profile.auditor.cli", plan.PrimarySource)
	}
}

// TestC24_NEG2_RuntimeReferenceHasNoRemovedFlagsDoc verifies that
// docs/operations/runtime-reference.md has no operator-facing documentation
// for the 5 removed flags.
//
// Covers NEG2 (doc hygiene). These flags were StatusInternal — never operator
// dials — so they should not appear in runtime-reference.md.
//
// // acs-predicate: config-check — reference doc must not document removed flags.
//
// PRE-EXISTING GREEN: runtime-reference.md currently has no entries for these flags.
func TestC24_NEG2_RuntimeReferenceHasNoRemovedFlagsDoc(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	runtimeRef := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	for _, flag := range removedFlags {
		if !acsassert.FileNotContains(t, runtimeRef, flag) {
			t.Errorf("RED: runtime-reference.md contains %q — should have no operator-facing\n"+
				"documentation for this removed flag (it was StatusInternal, never an operator dial).\n"+
				"File: %s", flag, runtimeRef)
		}
	}
}
