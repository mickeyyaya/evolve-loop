//go:build acs

// Package cycle32 materializes the cycle-32 acceptance criteria for:
//
//	workflow-internal-cluster-32 — migrate 4 live EVOLVE_* flags from os.Getenv/
//	envchain/envEnabled reads to policy.WorkflowConfig (Configuration Object + DI);
//	dead-sweep EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S (zero Go reader). 5 flags:
//	  - EVOLVE_BACKFILL_ENABLED        → WorkflowConfig.BackfillEnabled *bool (cyclerun_dispatch)
//	  - EVOLVE_CYCLE_BUDGET            → WorkflowConfig.CycleBudget string (cmd_loop)
//	  - EVOLVE_ALLOW_DEEP_RESEARCH     → QuotaConfig.AllowDeepResearch bool (DI in guards/quota)
//	  - EVOLVE_ALLOW_DOC_DELETE        → DocDelete.allow bool (DI in guards/docdelete)
//	  - EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S → dead sweep (docs-only; no Go reader)
//	Lower FlagCeiling 102→97; regenerate docs/architecture/control-flags.md.
//	Delete envEnabled() helper from guards/helpers.go (zero callers after migration).
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	workflow-internal-cluster-32:
//	  AC1   5 flags absent from Lookup                → C32_001 (behavioral)
//	  AC2   Registry row count == 97                  → C32_002 (behavioral, count)
//	  AC3   FlagCeiling const == 97                   → C32_003 (config-check, waiver)
//	  AC4   No env reads for 4 migrated flags in prod → C32_004 (config-check, waiver)
//	  AC5   WorkflowConfig().BackfillEnabled==true,   → C32_005 (behavioral, direct Go call)
//	         AllowDeepResearch==false (defaults)
//	  EDGE1 nil *bool BackfillEnabled → true default  → (covered by C32_005)
//	  AC6   EVOLVE_WORKTREE_PATH still registered     → C32_006 (behavioral, PRE-EXISTING GREEN)
//	  AC7   flagreaders regression guard green         → manual+checklist (see below)
//	  AC8   control-flags.md has no removed flag rows → C32_008 (config-check, waiver)
//	  NEG1  envEnabled helper deleted from helpers.go → C32_NEG1 (config-check, waiver)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle32 package;
//	    (b) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) no literal string "EVOLVE_BACKFILL_ENABLED", "EVOLVE_CYCLE_BUDGET",
//	        "EVOLVE_ALLOW_DEEP_RESEARCH", or "EVOLVE_ALLOW_DOC_DELETE"
//	        in any non-test, non-registry Go file via os.Getenv, envchain, or envEnabled
//	        (grep -rn 'os\.Getenv.*EVOLVE_BACKFILL_ENABLED\|EVOLVE_CYCLE_BUDGET\|
//	         EVOLVE_ALLOW_DEEP_RESEARCH\|EVOLVE_ALLOW_DOC_DELETE'
//	         go/ --include='*.go'
//	        | grep -v '_test.go' | grep -v 'registry_table.go'
//	        | grep -v 'acs/cycle32' → 0 matches);
//	    (d) EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S had zero Go readers before dead
//	        sweep; verify no new reader was added (grep returns 0 non-test Go matches).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C32_001 — 5 flags must be ABSENT from Lookup (if Builder misses any
//	           one, Lookup returns ok=true and the test fails immediately).
//	           C32_004 — env-read literals must be ABSENT from specific production
//	           files (if Builder removes registry rows without deleting call sites,
//	           the literal strings remain and this fails).
//	           C32_NEG1 — envEnabled() function itself must be ABSENT (if Builder
//	           migrates callers but leaves the dead helper, this fails — the cycle-85
//	           grep-only gaming surface closed at the function level).
//	Edge/OOD:  C32_002 checks exact count 97; both over-removal (< 97) and
//	           under-removal (> 97) fail.
//	Lexical:   Lookup / len / FileContains / FileNotContains / direct struct field
//	           access — distinct assertion verbs across the suite.
//	Semantic:  registry-absence, row-count, ceiling-const, no-env-reads,
//	           config-defaults, worktree-path-present, doc-absence,
//	           helper-fn-deleted — 8 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (workflow-internal-cluster-32). Deferred tasks (PSMAS_SKIP, STRICT_AUDIT,
// STRATEGY, CONSENSUS_AUDIT) get zero predicates.
//
// 1:1 enforcement:
//
//	predicate=8, manual+checklist=1, unverifiable-remove=0 → total AC=9 ✓
//	(EDGE1 merged into C32_005; both default behaviors covered by one predicate)
package cycle32

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// removedFlags is the canonical list of 5 flags that cycle-32 removes:
//   - EVOLVE_BACKFILL_ENABLED:               migrated to WorkflowConfig.BackfillEnabled
//   - EVOLVE_CYCLE_BUDGET:                   migrated to WorkflowConfig.CycleBudget
//   - EVOLVE_ALLOW_DEEP_RESEARCH:            migrated to QuotaConfig.AllowDeepResearch (DI)
//   - EVOLVE_ALLOW_DOC_DELETE:               migrated to DocDelete.allow (DI)
//   - EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S: dead sweep (docs-only; no Go reader)
var removedFlags = []string{
	"EVOLVE_ALLOW_DEEP_RESEARCH",
	"EVOLVE_ALLOW_DOC_DELETE",
	"EVOLVE_BACKFILL_ENABLED",
	"EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S",
	"EVOLVE_CYCLE_BUDGET",
}

// TestC32_001_RemovedFlagsAbsentFromRegistry verifies that all 5 removed flags
// are no longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1. The 5 flags span two removal patterns:
//   - 4 live flags: WorkflowConfig / DI migration (campaign buckets 1 + 5)
//   - EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S: dead sweep (zero Go reader)
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the registry row must be absent for
// Lookup to return ok=false.
//
// RED: all 5 flags are currently registered (FlagCeiling=102); each Lookup
// returns (flag, true).
func TestC32_001_RemovedFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range removedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-32 workflow-internal-cluster-32).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC32_002_RegistryRowCountIs97 verifies that after removing all 5 rows the
// total registry count is exactly 97.
//
// Covers AC2. Both over-removal (< 97) and under-removal (> 97) fail.
//
// BEHAVIORAL: calls len(flagregistry.All) directly — the production SSOT slice.
// No source-file grepping; adding a magic string to source cannot satisfy this.
//
// RED: len(flagregistry.All) is currently 102, which is 5 rows above 97.
func TestC32_002_RegistryRowCountIs97(t *testing.T) {
	const want = 97
	if got := len(flagregistry.All); got != want {
		t.Errorf("RED: len(flagregistry.All) = %d, want %d.\n"+
			"Builder must remove all 5 rows from registry_table.go.\n"+
			"Both over-removal (< 97) and under-removal (> 97) fail.\n"+
			"Expected: 102 − 5 = 97.",
			got, want)
	}
}

// TestC32_003_FlagCeilingConstIs97 verifies that the FlagCeiling ratchet
// constant in registry_ceiling_test.go has been lowered from 102 to 97
// in the same diff as the 5-row removal.
//
// acs-predicate: config-check — the constant value is the canonical ratchet;
// keeping 102 after the 5-row removal breaks the ratchet guarantee.
//
// RED: registry_ceiling_test.go currently has FlagCeiling = 102.
func TestC32_003_FlagCeilingConstIs97(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	ceilingFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_ceiling_test.go")
	if !acsassert.FileContains(t, ceilingFile, "FlagCeiling = 97") {
		t.Errorf("RED: registry_ceiling_test.go does not contain 'FlagCeiling = 97'.\n"+
			"Builder must lower the FlagCeiling constant from 102 to 97 in the same diff\n"+
			"as removing the 5 removed rows (102 − 5 = 97).\n"+
			"File: %s", ceilingFile)
	}
}

// TestC32_004_NoEnvReadsInProductionGo verifies that the env-read mechanisms
// for all 4 live migrated flags have been deleted from their specific production
// Go files.
//
// Covers AC4. Config-check waiver: FileNotContains asserts structural absence
// of the env-read patterns per file:
//   - cyclerun_dispatch.go:  "EVOLVE_BACKFILL_ENABLED" (read via envchain.BoolValue)
//   - cmd_loop.go:           "EVOLVE_CYCLE_BUDGET" (read via os.Getenv)
//   - guards/quota.go:       "EVOLVE_ALLOW_DEEP_RESEARCH" (read via envEnabled)
//   - guards/docdelete.go:   "EVOLVE_ALLOW_DOC_DELETE" (read via envEnabled)
//
// acs-predicate: config-check
//
// RED:
//
//	cyclerun_dispatch.go:179  envchain.BoolValue(cr.envSnap["EVOLVE_BACKFILL_ENABLED"], true)
//	cmd_loop.go:269           cyclebudget.ParseStage(os.Getenv("EVOLVE_CYCLE_BUDGET"))
//	guards/quota.go:45        envEnabled("EVOLVE_ALLOW_DEEP_RESEARCH")
//	guards/docdelete.go:26    envEnabled("EVOLVE_ALLOW_DOC_DELETE")
func TestC32_004_NoEnvReadsInProductionGo(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	checks := []struct {
		file    string
		absents []string
	}{
		{
			filepath.Join(root, "go", "internal", "core", "cyclerun_dispatch.go"),
			[]string{"EVOLVE_BACKFILL_ENABLED"},
		},
		{
			filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go"),
			[]string{"EVOLVE_CYCLE_BUDGET"},
		},
		{
			filepath.Join(root, "go", "internal", "guards", "quota.go"),
			[]string{"EVOLVE_ALLOW_DEEP_RESEARCH"},
		},
		{
			filepath.Join(root, "go", "internal", "guards", "docdelete.go"),
			[]string{"EVOLVE_ALLOW_DOC_DELETE"},
		},
	}
	for _, c := range checks {
		for _, pattern := range c.absents {
			if !acsassert.FileNotContains(t, c.file, pattern) {
				t.Errorf("RED: %s still contains env read for %q.\n"+
					"Builder must remove the os.Getenv / envchain / envEnabled() call for this flag\n"+
					"and replace it with the WorkflowConfig field or DI bool parameter.\n"+
					"File: %s",
					filepath.Base(c.file), pattern, c.file)
			}
		}
	}
}

// TestC32_005_WorkflowConfigDefaults verifies that policy.Policy{}.WorkflowConfig()
// returns the correct zero-value defaults for the two new fields:
//   - BackfillEnabled == true  (nil *bool → default-on, matching envchain default)
//   - AllowDeepResearch == false (zero value; opt-in override, not default-on)
//
// Covers AC5 + EDGE1 (merged). BEHAVIORAL: directly calls the production
// WorkflowConfig() resolver on an empty Policy — the same code path the
// orchestrator uses at composition time. A magic-string source edit cannot
// satisfy this; the actual resolver logic must wire the defaults correctly.
//
// RED: WorkflowConfig does not yet have BackfillEnabled or AllowDeepResearch
// fields — this test fails to compile until Builder adds them (compile failure
// IS the RED state for struct-extension ACs).
func TestC32_005_WorkflowConfigDefaults(t *testing.T) {
	cfg := policy.Policy{}.WorkflowConfig()

	// BackfillEnabled must default to true (nil *bool in WorkflowPolicy → true).
	// EDGE1: an absent/nil *bool must resolve to true, not false — matching the
	// existing envchain.BoolValue(..., true) default that this field replaces.
	if !cfg.BackfillEnabled {
		t.Errorf("RED: WorkflowConfig().BackfillEnabled = false, want true.\n" +
			"WorkflowPolicy.BackfillEnabled is *bool; nil must resolve to the default-on\n" +
			"value (true), matching envchain.BoolValue(cr.envSnap[\"EVOLVE_BACKFILL_ENABLED\"], true).\n" +
			"Builder must initialize the default: c.BackfillEnabled = true in WorkflowConfig().")
	}

	// AllowDeepResearch must default to false (zero value; opt-in, not default-on).
	if cfg.AllowDeepResearch {
		t.Errorf("RED: WorkflowConfig().AllowDeepResearch = true, want false.\n" +
			"AllowDeepResearch is an opt-in override (was gated by envEnabled == os.Getenv == '1').\n" +
			"The default must be false; the operator sets it to true in policy.json when needed.")
	}
}

// TestC32_006_WorktreePathStillRegistered is the non-repeat guard: verifies that
// EVOLVE_WORKTREE_PATH was NOT accidentally removed as part of the cluster sweep.
// Cycles 17, 18, and 19 all failed when a Builder removed WORKTREE_PATH —
// this predicate closes that regression surface.
//
// Covers AC6 (FORBIDDEN-REPEAT guard). BEHAVIORAL: calls flagregistry.Lookup —
// the test fails if Builder removes the row (Lookup returns ok=false).
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently in the registry (StatusInternal);
// this test is GREEN before Builder makes any changes. It stays GREEN only if
// Builder does NOT touch the WORKTREE_PATH row.
func TestC32_006_WorktreePathStillRegistered(t *testing.T) {
	if _, ok := flagregistry.Lookup("EVOLVE_WORKTREE_PATH"); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH was removed.\n"+
			"This flag is on the FORBIDDEN-REPEAT list (cycles 17/18/19 fail history).\n"+
			"Builder must NOT touch EVOLVE_WORKTREE_PATH in registry_table.go.",
			"EVOLVE_WORKTREE_PATH")
	}
}

// TestC32_008_ControlFlagsMdHasNoRemovedRows verifies that the generated doc
// docs/architecture/control-flags.md has no entries for any of the 5 removed
// flags after the registry rows are removed and the doc regenerated via
// 'evolve flags generate'.
//
// Covers AC8. The doc is generated from the flagregistry (source of truth);
// absence follows from C32_001 (rows removed) plus regeneration.
//
// acs-predicate: config-check — doc regeneration is a required build step.
//
// RED: control-flags.md currently has entries for all 5 removed flags.
func TestC32_008_ControlFlagsMdHasNoRemovedRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range removedFlags {
		if !acsassert.FileNotContains(t, controlFlags, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 5 rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", name, controlFlags)
		}
	}
}

// TestC32_NEG1_EnvEnabledHelperDeleted is the anti-gaming predicate that verifies
// the envEnabled() helper function has been completely deleted from guards/helpers.go.
//
// Anti-gaming rationale (cycle-8/cycle-85 lesson): a Builder could migrate both
// quota.go and docdelete.go callers away from envEnabled() while leaving the dead
// helper function in place. C32_004 confirms the ALLOW_DEEP_RESEARCH and
// ALLOW_DOC_DELETE env-var strings are gone from the individual guard files;
// NEG1 adds a second layer by asserting the shared envEnabled() helper itself is
// gone — closing the gaming surface where the function stays as dead code.
//
// acs-predicate: config-check
//
// RED: guards/helpers.go:29 defines "func envEnabled(name string) bool" that
// reads os.Getenv(name) == "1". After migration, this function must not exist.
func TestC32_NEG1_EnvEnabledHelperDeleted(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	helpersFile := filepath.Join(root, "go", "internal", "guards", "helpers.go")
	if !acsassert.FileNotContains(t, helpersFile, "envEnabled") {
		t.Errorf("RED: guards/helpers.go still contains the 'envEnabled' function.\n"+
			"Builder must DELETE the envEnabled() helper (not just remove callers)\n"+
			"after migrating both quota.go and docdelete.go callers to the DI bool params.\n"+
			"The helper is the sole env-enable mechanism for guards; its presence means\n"+
			"the migration is incomplete even if individual callers are updated.\n"+
			"File: %s", helpersFile)
	}
}
