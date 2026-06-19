//go:build acs

// Package cycle37 materializes the cycle-37 acceptance criteria for TWO tasks
// (both in triage ## top_n):
//
//  1. router-config-cluster-37 — migrate 6 EVOLVE_ROUTER_* / EVOLVE_ROUTING_*
//     flags from os.Getenv/applyEnv to policy.RouterPolicy (Configuration Object);
//     delete 3 stale env-path test files; update cmd_router_dispatch_test.go;
//     lower FlagCeiling 89→83; regenerate docs/architecture/control-flags.md.
//     Flags targeted:
//     - EVOLVE_ROUTER_REPLAN      → policy.RouterPolicy.RouterReplan  (default "shadow")
//     - EVOLVE_ROUTING_JUDGE      → policy.RouterPolicy.RoutingJudge  (default false)
//     - EVOLVE_ROUTER_RECON_DIGEST → policy.RouterPolicy.ReconDigest (default false)
//     - EVOLVE_ROUTER_REPLAN_DEPTH → policy.RouterPolicy.ReplanDepth (default 1)
//     - EVOLVE_ROUTER_PLAN_MODEL  → policy.RouterPolicy.PlanModel    (default "")
//     - EVOLVE_ROUTER_PROPOSE_MODEL → policy.RouterPolicy.ProposeModel (default "")
//
//  2. unexplained-outcome-cycle-0 — Route the escaping terminal path (when
//     newCycleRun fails before any phase runs) through outcome recording so
//     cyclehealth.ClassifyOutcome returns FAILED_EXPLAINED instead of
//     FAILED_UNEXPLAINED. Closes the ADR-0044 C1 chokepoint for the cycle-0
//     init-failure path.
//
// AC map (1:1 with triage top_n tasks):
//
//	router-config-cluster-37:
//	  AC1  6 flags absent from Lookup                    → C37_001 (behavioral)
//	  AC2  Registry row count == 83                      → C37_002 (behavioral, count)
//	  AC3  FlagCeiling const == 83                       → C37_003 (config-check, waiver)
//	  AC4  No router env reads in config.go or cmd_cycle.go → C37_004 (config-check, waiver)
//	  AC5  policy.Policy{}.RouterConfig() returns correct defaults → C37_005 (behavioral)
//	  AC6  EVOLVE_WORKTREE_PATH still registered         → C37_006 (behavioral, PRE-EXISTING GREEN)
//	  AC7  flagreaders regression guard green             → manual+checklist (see below)
//	  AC8  control-flags.md has no entries for 6 removed flags → C37_008 (config-check, waiver)
//	  NEG1 3 stale env-path test files absent from disk  → C37_NEG1 (behavioral)
//	  FULL go test ./... exits clean (cycle-35 regression gate) → manual+checklist
//
//	unexplained-outcome-cycle-0:
//	  AC9  ClassifyOutcome on empty workspace returns FAILED_EXPLAINED, not FAILED_UNEXPLAINED → C37_009 (behavioral)
//	  NEG2 ClassifyOutcome on workspace with ship PASS still returns SHIPPED → C37_NEG2 (behavioral, regression)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle37 package;
//	    (b) exit 0 from `cd go && go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) none of the 6 flag name strings appear in any non-test, non-registry Go file:
//	        grep -rn '"EVOLVE_ROUTER_REPLAN"\|"EVOLVE_ROUTING_JUDGE"\|"EVOLVE_ROUTER_RECON_DIGEST"
//	         \|"EVOLVE_ROUTER_REPLAN_DEPTH"\|"EVOLVE_ROUTER_PLAN_MODEL"\|"EVOLVE_ROUTER_PROPOSE_MODEL"'
//	         go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go'
//	         | grep -v 'acs/cycle37' → 0 matches;
//	    (d) all 6 flags had active env reads in config.go:applyEnv or cmd_cycle.go before the sweep;
//	        verify that their env reads (and only their env reads) are removed.
//
//	FULL (go test ./... clean): `cd go && go test ./... -count=1` (not just -tags acs).
//	    Checklist for Auditor:
//	    (a) exit 0 from `cd go && go test ./... -count=1` — the cycle-35 regression gate;
//	    (b) in particular confirm the 3 deleted test files no longer cause compile errors;
//	    (c) cmd_router_dispatch_test.go compiles with updated function signatures.
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C37_001 — 6 flags must be ABSENT from Lookup (if Builder misses
//	            any one, Lookup returns ok=true and the test fails immediately).
//	            C37_004 — env-read literals must be ABSENT from their source files
//	            (split-const anti-gaming: removing the registry row without deleting
//	            the os.Getenv / env[...] call is the cycle-8 failure mode).
//	            C37_NEG1 — 3 stale test files must NOT exist on disk (the cycle-35
//	            root cause: these files tested the now-deleted env paths and caused
//	            full-suite red after the migration).
//	Edge/OOD:   C37_002 checks exact count 83; both over-removal (<83) and
//	            under-removal (>83) fail.
//	Lexical:    Lookup / len / FileContains / FileNotContains / RouterConfig() /
//	            os.Stat / ClassifyOutcome — distinct assertion verbs across the suite.
//	Semantic:   registry-absence, row-count, ceiling-const, no-env-reads,
//	            router-config-defaults, worktree-path-present, no-doc-entries,
//	            stale-file-deletion, outcome-classification, outcome-regression —
//	            10 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n tasks
// (router-config-cluster-37 and unexplained-outcome-cycle-0). Deferred tasks
// (per-phase agent config cluster, cycle-audit-cycle-scoped-ci-gap, etc.) get
// zero predicates this cycle.
//
// 1:1 enforcement:
//
//	predicate=9, manual+checklist=2, unverifiable-remove=0 → total AC=11 ✓
//	(AC7=manual+checklist, FULL=manual+checklist; remaining 9 are predicate)
package cycle37

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// removedFlags is the canonical list of 6 flags that cycle-37 removes:
//   - EVOLVE_ROUTER_REPLAN:       migrated to policy.RouterPolicy.RouterReplan
//   - EVOLVE_ROUTING_JUDGE:       migrated to policy.RouterPolicy.RoutingJudge
//   - EVOLVE_ROUTER_RECON_DIGEST: migrated to policy.RouterPolicy.ReconDigest
//   - EVOLVE_ROUTER_REPLAN_DEPTH: migrated to policy.RouterPolicy.ReplanDepth
//   - EVOLVE_ROUTER_PLAN_MODEL:   migrated to policy.RouterPolicy.PlanModel
//   - EVOLVE_ROUTER_PROPOSE_MODEL: migrated to policy.RouterPolicy.ProposeModel
var removedFlags = []string{
	"EVOLVE_ROUTER_REPLAN",
	"EVOLVE_ROUTING_JUDGE",
	"EVOLVE_ROUTER_RECON_DIGEST",
	"EVOLVE_ROUTER_REPLAN_DEPTH",
	"EVOLVE_ROUTER_PLAN_MODEL",
	"EVOLVE_ROUTER_PROPOSE_MODEL",
}

// TestC37_001_RemovedFlagsAbsentFromRegistry verifies that all 6 removed flags
// are no longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1. BEHAVIORAL: calls flagregistry.Lookup() for each flag — the
// production SSOT. A source edit alone cannot satisfy this; the registry row
// must be absent for Lookup to return ok=false.
//
// RED: all 6 flags are currently registered (FlagCeiling=89); each Lookup
// returns (flag, true).
func TestC37_001_RemovedFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range removedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-37 router-config-cluster-37).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC37_002_RegistryRowCountIs83 verifies that after removing all 6 rows the
// total registry count is exactly 83.
//
// Covers AC2. BEHAVIORAL: calls len(flagregistry.All) — the production count.
// Over-removal (<83) and under-removal (>83) both fail.
//
// RED: registry currently has 89 rows (FlagCeiling=89); count is 89.
func TestC37_002_RegistryRowCountIs83(t *testing.T) {
	got := len(flagregistry.All)
	if got != 83 {
		t.Errorf("RED: len(flagregistry.All) = %d, want 83 (89 − 6 removed router flags).\n"+
			"Builder must remove exactly 6 rows from registry_table.go:\n"+
			"  EVOLVE_ROUTER_REPLAN, EVOLVE_ROUTING_JUDGE, EVOLVE_ROUTER_RECON_DIGEST,\n"+
			"  EVOLVE_ROUTER_REPLAN_DEPTH, EVOLVE_ROUTER_PLAN_MODEL, EVOLVE_ROUTER_PROPOSE_MODEL",
			got)
	}
}

// TestC37_003_FlagCeilingConstIs83 verifies that the FlagCeiling ratchet constant
// has been updated from 89 to 83 in registry_ceiling_test.go.
//
// Covers AC3. The ratchet prevents accidental registry growth; lowering it by 6
// (89−6=83) is mandatory alongside the row removal.
//
// acs-predicate: config-check
//
// RED: registry_ceiling_test.go currently has FlagCeiling = 89.
func TestC37_003_FlagCeilingConstIs83(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	ceilingFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_ceiling_test.go")
	if !acsassert.FileContains(t, ceilingFile, "FlagCeiling = 83") {
		t.Errorf("RED: registry_ceiling_test.go does not contain 'FlagCeiling = 83'.\n"+
			"Builder must lower the FlagCeiling constant from 89 to 83 in the same diff\n"+
			"as removing the 6 router rows (89 − 6 = 83).\n"+
			"File: %s", ceilingFile)
	}
}

// TestC37_004_NoRouterEnvReadsInSourceFiles verifies that all 6 router-flag
// env-read string literals have been deleted from their source files:
//   - config.go:applyEnv  — reads env["EVOLVE_ROUTER_REPLAN"], env["EVOLVE_ROUTING_JUDGE"],
//     env["EVOLVE_ROUTER_RECON_DIGEST"], env["EVOLVE_ROUTER_REPLAN_DEPTH"]
//   - cmd_cycle.go        — reads os.Getenv("EVOLVE_ROUTER_PLAN_MODEL"),
//     os.Getenv("EVOLVE_ROUTER_PROPOSE_MODEL")
//
// Covers AC4. Anti-gaming (cycle-8 split-const lesson): Builder cannot remove
// the registry rows while leaving the env["..."] or os.Getenv("...") call sites.
// This predicate catches that gap for all 6 flags across both source files.
//
// acs-predicate: config-check
//
// RED: config.go currently reads 4 flags at lines ~530–580; cmd_cycle.go reads
// 2 flags at lines ~574–578. All 6 flag name strings must be absent after migration.
func TestC37_004_NoRouterEnvReadsInSourceFiles(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	configFile := filepath.Join(root, "go", "internal", "config", "config.go")
	cmdFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle.go")

	// 4 flags read in config.go:applyEnv
	configFlags := []string{
		"EVOLVE_ROUTER_REPLAN",
		"EVOLVE_ROUTING_JUDGE",
		"EVOLVE_ROUTER_RECON_DIGEST",
		"EVOLVE_ROUTER_REPLAN_DEPTH",
	}
	for _, name := range configFlags {
		if !acsassert.FileNotContains(t, configFile, name) {
			t.Errorf("RED: config.go still contains the string %q.\n"+
				"Builder must delete the entire if-block for env[%q] in applyEnv\n"+
				"(cycle-8 anti-gaming: removing the registry row without deleting the env read\n"+
				"is the split-const hiding pattern).\n"+
				"File: %s", name, name, configFile)
		}
	}

	// 2 flags read in cmd_cycle.go via os.Getenv
	cmdFlags := []string{
		"EVOLVE_ROUTER_PLAN_MODEL",
		"EVOLVE_ROUTER_PROPOSE_MODEL",
	}
	for _, name := range cmdFlags {
		if !acsassert.FileNotContains(t, cmdFile, name) {
			t.Errorf("RED: cmd_cycle.go still contains the string %q.\n"+
				"Builder must replace os.Getenv(%q) with rc.PlanModel/rc.ProposeModel\n"+
				"from the policy.RouterConfig parameter (cycle-37 fix for cmd_cycle.go).\n"+
				"File: %s", name, name, cmdFile)
		}
	}
}

// TestC37_005_RouterConfigDefaults verifies that policy.Policy{}.RouterConfig()
// returns the correct zero-value defaults for the six new fields:
//   - RouterReplan  == "shadow" (replicates config defaults() StageShadow)
//   - RoutingJudge  == false   (default off)
//   - ReconDigest   == false   (default off)
//   - ReplanDepth   == 1       (default 1 — cap, not disable)
//   - PlanModel     == ""      (default: no override)
//   - ProposeModel  == ""      (default: no override)
//
// Covers AC5. BEHAVIORAL: directly calls the production RouterConfig() resolver
// on an empty Policy — the same code path the orchestrator uses at composition
// time (cmd_cycle.go, after pol := policy.Load(...)).
//
// RED: policy.Policy does not yet have RouterConfig() (RouterPolicy struct
// does not exist) — this test fails to COMPILE until Builder adds it.
// A compile failure IS the RED state for new-method ACs.
func TestC37_005_RouterConfigDefaults(t *testing.T) {
	cfg := policy.Policy{}.RouterConfig()

	if cfg.RouterReplan != "shadow" {
		t.Errorf("RED: RouterConfig().RouterReplan = %q, want \"shadow\".\n"+
			"RouterPolicy.RouterReplan default must be \"shadow\", matching\n"+
			"config.defaults() RouterReplan: StageShadow (ADR-0052 WS0-S1).",
			cfg.RouterReplan)
	}

	if cfg.RoutingJudge != false {
		t.Errorf("RED: RouterConfig().RoutingJudge = %v, want false.\n"+
			"RouterPolicy.RoutingJudge default must be false (off by default).",
			cfg.RoutingJudge)
	}

	if cfg.ReconDigest != false {
		t.Errorf("RED: RouterConfig().ReconDigest = %v, want false.\n"+
			"RouterPolicy.ReconDigest default must be false (off by default).",
			cfg.ReconDigest)
	}

	if cfg.ReplanDepth != 1 {
		t.Errorf("RED: RouterConfig().ReplanDepth = %d, want 1.\n"+
			"RouterPolicy.ReplanDepth default must be 1, matching\n"+
			"config.defaults() RePlanMaxDepth (EVOLVE_ROUTER_REPLAN_DEPTH default).",
			cfg.ReplanDepth)
	}

	if cfg.PlanModel != "" {
		t.Errorf("RED: RouterConfig().PlanModel = %q, want \"\".\n"+
			"RouterPolicy.PlanModel default must be empty string (no override).",
			cfg.PlanModel)
	}

	if cfg.ProposeModel != "" {
		t.Errorf("RED: RouterConfig().ProposeModel = %q, want \"\".\n"+
			"RouterPolicy.ProposeModel default must be empty string (no override).",
			cfg.ProposeModel)
	}
}

// TestC37_006_WorktreePathStillRegistered is the no-repeat guard: verifies that
// EVOLVE_WORKTREE_PATH was NOT accidentally removed as part of the cluster sweep.
// Cycles 17, 18, and 19 all failed when a Builder removed WORKTREE_PATH —
// this predicate closes that regression surface for cycle 37.
//
// Covers AC6 (FORBIDDEN-REPEAT guard). BEHAVIORAL: calls flagregistry.Lookup —
// the test fails if Builder removes the row (Lookup returns ok=false).
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently in the registry (StatusInternal);
// this test is GREEN before Builder makes any changes. It stays GREEN only if
// Builder does NOT touch the WORKTREE_PATH row.
func TestC37_006_WorktreePathStillRegistered(t *testing.T) {
	if _, ok := flagregistry.Lookup("EVOLVE_WORKTREE_PATH"); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH was removed.\n"+
			"This flag is on the FORBIDDEN-REPEAT list (cycles 17/18/19 fail history).\n"+
			"Builder must NOT touch EVOLVE_WORKTREE_PATH in registry_table.go.",
			"EVOLVE_WORKTREE_PATH")
	}
}

// TestC37_008_ControlFlagsDocNoRemovedFlags verifies that the regenerated
// docs/architecture/control-flags.md no longer contains entries for the 6 removed
// flags. This checks that `evolve flags generate` was run (or equivalent) in the
// same diff as the registry row removals.
//
// Covers AC8. acs-predicate: config-check
//
// RED: control-flags.md currently has rows for all 6 flags (they are active in
// the registry). After the migration, the doc must be regenerated and all 6
// flag names must be absent.
func TestC37_008_ControlFlagsDocNoRemovedFlags(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range removedFlags {
		if !acsassert.FileNotContains(t, controlFlagsDoc, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must regenerate docs/architecture/control-flags.md after removing the\n"+
				"6 router flag rows (e.g. `evolve flags generate`) in the same diff.\n"+
				"File: %s", name, controlFlagsDoc)
		}
	}
}

// TestC37_NEG1_StaleConfigTestFilesDeleted verifies that the 3 stale test files
// in go/internal/config/ — which tested the now-deleted env override paths for
// EVOLVE_ROUTER_REPLAN, EVOLVE_ROUTING_JUDGE, and EVOLVE_ROUTER_RECON_DIGEST —
// have been deleted. This was the cycle-35 root cause: the migration was correct
// but these files still referenced the deleted env paths, causing go test ./... to
// fail with compile errors.
//
// Covers NEG1. BEHAVIORAL: os.Stat checks that the files are absent on disk
// (no FileExists helper needed — we expect the error case).
//
// RED: all 3 stale files currently exist on disk.
// GREEN after Builder deletes them in the same diff as the registry row removals.
func TestC37_NEG1_StaleConfigTestFilesDeleted(t *testing.T) {
	root := acsassert.RepoRoot(t)
	staleFiles := []string{
		"go/internal/config/router_replan_test.go",
		"go/internal/config/routing_judge_test.go",
		"go/internal/config/router_recon_test.go",
	}
	for _, rel := range staleFiles {
		abs := filepath.Join(root, rel)
		if _, err := os.Stat(abs); err == nil {
			t.Errorf("RED: stale test file %q still exists on disk.\n"+
				"This file tests the now-deleted env override path for the migrated flag.\n"+
				"Builder must DELETE this file in the same diff as the registry row removals\n"+
				"(this was the cycle-35 root cause: stale test files caused full-suite red).\n"+
				"Path: %s", rel, abs)
		}
	}
}

// ---------------------------------------------------------------------------
// unexplained-outcome-cycle-0 predicates (AC9, NEG2)
// ---------------------------------------------------------------------------

// phaseTimingEntry mirrors the JSON schema of core.phaseTimingEntry (the C1
// record written by recordPhaseOutcome). Kept local: cyclehealth must not
// import core, so we replicate the minimal schema needed to write a fixture.
type phaseTimingEntry struct {
	Phase       string `json:"phase"`
	Verdict     string `json:"verdict"`
	AbortReason string `json:"abort_reason,omitempty"`
}

// TestC37_009_EmptyWorkspaceClassifiesExplained verifies that when the cycle
// workspace has no phase-timing.json and no interaction-summary.json (the
// scenario produced by newCycleRun failing before any phase runs), the outcome
// is NOT FAILED_UNEXPLAINED.
//
// Covers AC9. BEHAVIORAL: directly calls cyclehealth.ClassifyOutcome on a
// temp dir with no files — the exact state the cycle-0 escaping path produces.
// The outcome must be FAILED_EXPLAINED (or any non-UNEXPLAINED outcome) after
// the fix routes the init-failure path through the C1 chokepoint.
//
// Builder can fix this in either of two ways:
//
//	(a) Modify cyclehealth.ClassifyOutcome to treat an empty/absent workspace
//	    as FAILED_EXPLAINED (initialization failed before any phase ran).
//	(b) Make orchestrator.RunCycle write a minimal phase-timing.json entry with
//	    abort_reason before returning the newCycleRun error, so ClassifyOutcome
//	    finds the C1 record and classifies it as FAILED_EXPLAINED.
//
// RED: currently ClassifyOutcome("empty-dir") returns FAILED_UNEXPLAINED
// because no timing file exists and none of the classifier's positive checks
// (ship PASS, salvage, abort_reason, FAIL verdict) match.
func TestC37_009_EmptyWorkspaceClassifiesExplained(t *testing.T) {
	emptyWorkspace := t.TempDir() // no files at all — the init-fail scenario
	outcome, detail := cyclehealth.ClassifyOutcome(emptyWorkspace)
	if outcome == cyclehealth.OutcomeFailedUnexplained {
		t.Errorf("RED: ClassifyOutcome(empty workspace) = %s (detail: %q).\n"+
			"An empty workspace is produced when newCycleRun fails before any phase runs\n"+
			"(the cycle-0 FAILED_UNEXPLAINED incident). After the fix, this must return\n"+
			"FAILED_EXPLAINED (or another non-UNEXPLAINED outcome) — the C1 chokepoint\n"+
			"must cover the init-failure escaping path (ADR-0044 §C1).\n"+
			"Builder must route the escaping path through outcome recording.\n"+
			"See: unexplained-outcome-cycle-0 triage item.", outcome, detail)
	}
}

// TestC37_NEG2_ShipPassWorkspaceClassifiesShipped verifies that a workspace
// containing a phase-timing.json with a ship PASS verdict still classifies as
// SHIPPED. This is the regression guard: the fix for AC9 must not break the
// primary classification path.
//
// Covers NEG2. BEHAVIORAL: constructs a minimal phase-timing.json fixture and
// calls ClassifyOutcome directly.
//
// PRE-EXISTING GREEN: the SHIPPED classification is already correct; this test
// stays GREEN and ensures the cycle-0 fix does not regress it.
func TestC37_NEG2_ShipPassWorkspaceClassifiesShipped(t *testing.T) {
	ws := t.TempDir()
	timingEntries := []phaseTimingEntry{
		{Phase: "ship", Verdict: "PASS"},
	}
	raw, err := json.Marshal(timingEntries)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "phase-timing.json"), raw, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	outcome, _ := cyclehealth.ClassifyOutcome(ws)
	if outcome != cyclehealth.OutcomeShipped {
		t.Errorf("REGRESSION: ClassifyOutcome with ship PASS timing = %s, want SHIPPED.\n"+
			"The fix for unexplained-outcome-cycle-0 must not break the primary\n"+
			"SHIPPED classification path.", outcome)
	}
}
