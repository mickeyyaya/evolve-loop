//go:build acs

// Package cycle39 materializes the cycle-39 acceptance criteria for TWO tasks
// (both in triage ## top_n):
//
//  1. legacyflags-phase-enable-cluster-39 — migrate 6 phase-enable flags
//     (REQUIRE_INTENT, TRIAGE_DISABLE, PLAN_REVIEW, TEST_PHASE_ENABLED,
//     BUILD_PLANNER, SWARM_PLANNER) from the `legacyFlags` env-map in config.go
//     to `WorkflowPolicy.PhaseEnables` in policy.json (config-as-code).
//     Delete the legacyFlags var and its iteration loop. Thread REQUIRE_INTENT
//     via o.workflowConfig.PhaseEnables["intent"] in cyclerun.go. Fix
//     routingtest/bricks.go IntentRequired() to use PhaseEnabled pattern.
//     Remove 6 registry rows. Lower FlagCeiling 80→74 (intermediate).
//
//  2. consensus-audit-config-39 — migrate EVOLVE_CONSENSUS_AUDIT (1 flag)
//     from os.Getenv in cmd_consensus_dispatch.go to
//     WorkflowPolicy.ConsensusAuditEnabled. Remove the redundant IPC write in
//     cmd_loop_args.go:270. Remove 1 registry row. Lower FlagCeiling 74→73
//     (FINAL).
//
// Both tasks ship in the same cycle audit. The FINAL state (73 flags,
// FlagCeiling=73) is what the audit validates; intermediate state (74 after
// Task 1 alone) has no separate predicate — same pattern as cycle-38.
//
// AC map (1:1 with triage top_n tasks):
//
//	legacyflags-phase-enable-cluster-39:
//	  AC1  6 legacy flags absent from Lookup          → C39_001 (behavioral)
//	  AC2  len(All)==74                               → INTERMEDIATE; superseded by
//	                                                     T2 AC2_CA (count=73 FINAL)
//	  AC3  FlagCeiling==74                            → INTERMEDIATE; superseded by
//	                                                     T2 AC3_CA (ceiling=73 FINAL)
//	  AC4  no prod env reads for 6 flags (anti-gaming)→ C39_004 (config-check, waiver)
//	  AC5  legacyFlags var deleted from config.go     → C39_005 (config-check, waiver)
//	  AC6  WorkflowPolicy.PhaseEnables resolves       → C39_006 (behavioral, compile-fail RED)
//	  AC7  WORKTREE_PATH still registered             → C39_007 (behavioral, PRE-EXISTING GREEN)
//	  AC8  flagreaders guard green                    → manual+checklist (see below)
//	  AC9  control-flags.md drops 6 rows              → C39_009 (config-check, waiver)
//	  NEG1 IntentRequired() uses PhaseEnabled pattern → C39_NEG1 (config-check, waiver)
//	  FULL go test ./... green                        → manual+checklist (see below)
//
//	consensus-audit-config-39:
//	  AC1_CA  CONSENSUS_AUDIT absent from Lookup      → C39_CA_001 (behavioral)
//	  AC2_CA  len(All)==73 (FINAL after both tasks)   → C39_CA_002 (behavioral, count)
//	  AC3_CA  FlagCeiling==73 (FINAL)                 → C39_CA_003 (config-check, waiver)
//	  AC4_CA  no prod env reads for CONSENSUS_AUDIT   → C39_CA_004 (config-check, waiver)
//	  AC5_CA  ConsensusAuditEnabled defaults true     → C39_CA_005 (behavioral, compile-fail RED)
//	  NEG1_CA IPC write removed from cmd_loop_args.go → C39_CA_NEG1 (config-check, waiver)
//	  FULL_CA go test ./... green                     → manual+checklist (see below)
//
// ACs with manual+checklist disposition:
//
//	AC8 / AC8_CA (flagreaders guard): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) exit 0 from `cd go && go test -tags acs ./acs/regression/flagreaders/...`;
//	    (b) none of the 7 flag name strings appear in any non-test, non-registry Go file:
//	        grep -rn '"EVOLVE_REQUIRE_INTENT"\|"EVOLVE_TRIAGE_DISABLE"\|"EVOLVE_PLAN_REVIEW"\|
//	          "EVOLVE_TEST_PHASE_ENABLED"\|"EVOLVE_BUILD_PLANNER"\|"EVOLVE_SWARM_PLANNER"\|
//	          "EVOLVE_CONSENSUS_AUDIT"' go/ --include='*.go' |
//	          grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches.
//	    NOTE: bricks.go intentionally NOT excluded — NEG1 predicate (C39_NEG1) already
//	    verifies the env injection is removed from bricks.go.
//
//	FULL / FULL_CA (go test ./... clean):
//	    Checklist for Auditor:
//	    (a) exit 0 from `cd go && go test ./... -count=1`;
//	    (b) no stale env-path test files in go/internal/config/ referencing the 6 deleted flags;
//	    (c) routingtest package compiles with updated IntentRequired() using PhaseEnabled;
//	    (d) cmd_consensus_dispatch.go compiles with policy.Load call replacing os.Getenv.
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C39_001 — 6 flags must be ABSENT from Lookup (any miss returns ok=true → fail).
//	            C39_004 — 6 flag string literals must be ABSENT from config.go/cyclerun.go
//	            (anti-gaming: registry row removal without env-read deletion is the cycle-8 pattern).
//	            C39_NEG1 — env injection must be ABSENT from bricks.go.
//	            C39_CA_001 — CONSENSUS_AUDIT must be ABSENT from Lookup.
//	            C39_CA_NEG1 — IPC write must be ABSENT from cmd_loop_args.go.
//	Edge/OOD:   C39_CA_002 checks EXACT count 73; over-removal (<73) and under-removal (>73) fail.
//	Lexical:    Lookup / len / FileNotContains / FileContains / WorkflowPolicy field access /
//	            WorkflowConfig() resolver / policy.Policy{} zero-value — distinct verbs.
//	Semantic:   registry-absence (7 flags), no-env-reads (multi-file anti-gaming), struct-field
//	            existence (2 new API surfaces), worktree-path-guard, no-doc-entries (7 flags),
//	            row-count, ceiling-const, ipc-write-absent — 9 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for committed top_n tasks
// (legacyflags-phase-enable-cluster-39, consensus-audit-config-39). Deferred tasks
// (STRICT_AUDIT, Dynamic Routing cluster, StatusInternal ~34 flags) get zero predicates.
//
// 1:1 enforcement:
//
//	Task 1: predicate=7, manual+checklist=2 (AC8, FULL), unverifiable-remove=0
//	        AC2+AC3 are INTERMEDIATE and superseded by T2 FINAL predicates → not counted separately
//	        → task 1 ACs: AC1(pred), AC2(intermediate/superseded), AC3(intermediate/superseded),
//	          AC4(pred), AC5(pred), AC6(pred), AC7(pred), AC8(manual), AC9(pred), NEG1(pred), FULL(manual)
//	Task 2: predicate=5, manual+checklist=1 (FULL_CA), unverifiable-remove=0
//	        → task 2 ACs: AC1_CA(pred), AC2_CA(pred), AC3_CA(pred), AC4_CA(pred), AC5_CA(pred),
//	          NEG1_CA(pred), FULL_CA(manual)
package cycle39

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// legacyRemovedFlags is the canonical list of 6 flags that cycle-39 Task 1 removes
// from the legacyFlags map in config.go and from the registry.
var legacyRemovedFlags = []string{
	"EVOLVE_REQUIRE_INTENT",
	"EVOLVE_TRIAGE_DISABLE",
	"EVOLVE_PLAN_REVIEW",
	"EVOLVE_TEST_PHASE_ENABLED",
	"EVOLVE_BUILD_PLANNER",
	"EVOLVE_SWARM_PLANNER",
}

// ---------------------------------------------------------------------------
// Task 1: legacyflags-phase-enable-cluster-39
// ---------------------------------------------------------------------------

// TestC39_001_LegacyFlagsAbsentFromRegistry verifies that all 6 legacy phase-enable
// flags are no longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1. BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production
// SSOT. A source edit alone cannot satisfy this; the registry row must be absent for
// Lookup to return ok=false.
//
// RED: all 6 flags are currently registered (FlagCeiling=80); each Lookup returns
// (flag, true).
func TestC39_001_LegacyFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range legacyRemovedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go\n"+
				"(legacyflags-phase-enable-cluster-39: migrate to WorkflowPolicy.PhaseEnables).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC39_004_NoProdLegacyFlagEnvReadsInSource verifies that the 6 legacy flag name
// string literals have been deleted from their production source files:
//   - config.go: the legacyFlags map var (lines 306–324) where each flag name is a
//     map key string literal, AND the for-loop iteration block (lines ~557–568) which
//     reads env[flag] for each flag in the map.
//   - cyclerun.go: the direct req.Env["EVOLVE_REQUIRE_INTENT"] read at line 240 in
//     newCycleRun (the only flag with a direct read outside the map iteration).
//
// Covers AC4 (and AC5 by extension: if the map key strings are gone, the var is deleted).
// Anti-gaming (cycle-8 split-const lesson): removing registry rows without deleting the
// env reads is the split-const hiding pattern.
//
// acs-predicate: config-check
//
// RED: config.go currently contains all 6 flag name literals in legacyFlags map
// (lines 307–323). cyclerun.go contains "EVOLVE_REQUIRE_INTENT" at line 240.
func TestC39_004_NoProdLegacyFlagEnvReadsInSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	configFile := filepath.Join(root, "go", "internal", "config", "config.go")
	cyclerunFile := filepath.Join(root, "go", "internal", "core", "cyclerun.go")

	// All 6 flag names must be absent from config.go (removes legacyFlags map + for-loop).
	for _, name := range legacyRemovedFlags {
		if !acsassert.FileNotContains(t, configFile, name) {
			t.Errorf("RED: config.go still contains the string %q.\n"+
				"Builder must delete the legacyFlags map entry for %q AND the for-loop\n"+
				"iteration at lines ~557–568 that reads env[flag] (cycle-8 anti-gaming:\n"+
				"removing the registry row without deleting the env read is split-const hiding).\n"+
				"File: %s", name, name, configFile)
		}
	}

	// EVOLVE_REQUIRE_INTENT also has a direct read in cyclerun.go:240.
	// Builder must replace it with o.workflowConfig.PhaseEnables["intent"] == "on".
	const directRead = "EVOLVE_REQUIRE_INTENT"
	if !acsassert.FileNotContains(t, cyclerunFile, directRead) {
		t.Errorf("RED: cyclerun.go still contains the string %q.\n"+
			"Builder must replace envchain.BoolValue(req.Env[\"EVOLVE_REQUIRE_INTENT\"], false)\n"+
			"with o.workflowConfig.PhaseEnables[\"intent\"] == \"on\" in newCycleRun (line ~240).\n"+
			"File: %s", directRead, cyclerunFile)
	}
}

// TestC39_005_LegacyFlagsVarDeletedFromConfig verifies that the legacyFlags variable
// declaration and the legacyFlag type are no longer present in config.go. This
// complements AC4 by asserting the MAP CONSTRUCT itself is gone (not just individual
// entries). If Builder deletes individual entries but leaves an empty legacyFlags map,
// the for-loop would still compile — this predicate catches that case.
//
// Covers AC5. acs-predicate: config-check
//
// RED: config.go currently has `var legacyFlags = map[string]legacyFlag{` at line 306.
func TestC39_005_LegacyFlagsVarDeletedFromConfig(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	configFile := filepath.Join(root, "go", "internal", "config", "config.go")

	// The map declaration line must be absent.
	if !acsassert.FileNotContains(t, configFile, "legacyFlags = map[string]legacyFlag{") {
		t.Errorf("RED: config.go still contains the legacyFlags map declaration.\n"+
			"Builder must delete the entire `var legacyFlags = map[string]legacyFlag{...}`\n"+
			"block (lines 306–324) from config.go. An empty map would still compile but\n"+
			"leaves dead code; the map and its for-loop must both be deleted.\n"+
			"File: %s", configFile)
	}

	// The for-loop referencing the map must be absent.
	if !acsassert.FileNotContains(t, configFile, "for flag, lf := range legacyFlags") {
		t.Errorf("RED: config.go still contains the legacyFlags for-loop.\n"+
			"Builder must delete the `for flag, lf := range legacyFlags { ... }` block\n"+
			"(lines ~557–568) from config.go in the same diff as deleting the map var.\n"+
			"File: %s", configFile)
	}
}

// TestC39_006_WorkflowPolicyPhaseEnablesResolves verifies that:
//  1. policy.WorkflowPolicy has a PhaseEnables map[string]string field.
//  2. policy.WorkflowConfig has a PhaseEnables map[string]string field.
//  3. The WorkflowConfig() resolver propagates p.Workflow.PhaseEnables into c.PhaseEnables.
//  4. The zero-value default (absent Workflow block) yields nil/empty PhaseEnables.
//
// Covers AC6. BEHAVIORAL (compile-fail RED): directly accesses WorkflowPolicy{PhaseEnables: ...}
// and WorkflowConfig.PhaseEnables. Until Builder adds these fields and wires them in
// WorkflowConfig(), this test FAILS TO COMPILE — a compile failure IS the RED state.
//
// RED: WorkflowPolicy (policy.go:462-473) and WorkflowConfig (policy.go:476-486) currently
// have NO PhaseEnables field. This test does not compile.
func TestC39_006_WorkflowPolicyPhaseEnablesResolves(t *testing.T) {
	// Direct struct field access — compile-fail RED until Builder adds PhaseEnables.
	cfg := policy.Policy{
		Workflow: &policy.WorkflowPolicy{
			PhaseEnables: map[string]string{"intent": "on"},
		},
	}.WorkflowConfig()

	if cfg.PhaseEnables["intent"] != "on" {
		t.Errorf("RED: WorkflowConfig().PhaseEnables[%q] = %q, want \"on\".\n"+
			"Builder must add PhaseEnables map[string]string to WorkflowPolicy AND\n"+
			"WorkflowConfig, then wire it in WorkflowConfig() resolver:\n"+
			"  c.PhaseEnables = p.Workflow.PhaseEnables",
			"intent", cfg.PhaseEnables["intent"])
	}

	// Zero-value default: absent Workflow block must yield nil/empty PhaseEnables
	// (no override — existing config.Load defaults remain authoritative).
	dflt := policy.Policy{}.WorkflowConfig()
	if len(dflt.PhaseEnables) != 0 {
		t.Errorf("RED: policy.Policy{}.WorkflowConfig().PhaseEnables has %d entries, want 0.\n"+
			"An absent/empty Workflow block must not override phase enables.\n"+
			"When Workflow is nil, c.PhaseEnables must be nil (Go zero value of map).",
			len(dflt.PhaseEnables))
	}
}

// TestC39_007_WorktreePathStillRegistered is the no-repeat guard: verifies that
// EVOLVE_WORKTREE_PATH was NOT accidentally removed as part of the cluster sweep.
// Cycles 17, 18, and 19 all failed when a Builder removed WORKTREE_PATH —
// this predicate closes that regression surface for cycle 39.
//
// Covers AC7 (FORBIDDEN-REPEAT guard). BEHAVIORAL: calls flagregistry.Lookup —
// the test fails if Builder removes the row (Lookup returns ok=false).
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently in the registry (StatusInternal);
// this test is GREEN before Builder makes any changes. It stays GREEN only if
// Builder does NOT touch the WORKTREE_PATH row.
func TestC39_007_WorktreePathStillRegistered(t *testing.T) {
	if _, ok := flagregistry.Lookup("EVOLVE_WORKTREE_PATH"); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH was removed.\n"+
			"This flag is on the FORBIDDEN-REPEAT list (cycles 17/18/19 fail history).\n"+
			"Builder must NOT touch EVOLVE_WORKTREE_PATH in registry_table.go.",
			"EVOLVE_WORKTREE_PATH")
	}
}

// TestC39_009_ControlFlagsDocNoLegacyFlagRows verifies that the regenerated
// docs/architecture/control-flags.md no longer contains entries for the 6 removed
// legacy phase-enable flags. This checks that `evolve flags generate` (or equivalent)
// was run in the same diff as the registry row removals.
//
// Covers AC9. acs-predicate: config-check
//
// RED: control-flags.md currently has rows for all 6 flags (they are active in the
// registry). After the migration, the doc must be regenerated and all 6 flag names
// must be absent.
func TestC39_009_ControlFlagsDocNoLegacyFlagRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range legacyRemovedFlags {
		if !acsassert.FileNotContains(t, controlFlagsDoc, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must regenerate docs/architecture/control-flags.md after removing\n"+
				"the 6 legacy flag rows (e.g. `evolve flags generate`) in the same diff.\n"+
				"File: %s", name, controlFlagsDoc)
		}
	}
}

// TestC39_NEG1_IntentRequiredUsesPhaseEnabledNotEnvInjection verifies that
// routingtest/bricks.go IntentRequired() no longer injects EVOLVE_REQUIRE_INTENT
// into the scenario env map. After the migration, IntentRequired() must use the
// PhaseEnabled brick pattern (s.Enable["intent"] = config.EnableOn) instead of
// the env injection (s.Env["EVOLVE_REQUIRE_INTENT"] = "1") which bypassed the
// routing engine's PhaseEnable: s.Enable path.
//
// Covers NEG1. Anti-gaming: bricks.go with env injection tests the deleted env
// override path rather than the new PhaseEnables policy path.
//
// acs-predicate: config-check
//
// RED: bricks.go currently contains `s.Env["EVOLVE_REQUIRE_INTENT"] = "1"` at line 121.
func TestC39_NEG1_IntentRequiredUsesPhaseEnabledNotEnvInjection(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	bricksFile := filepath.Join(root, "go", "internal", "routingtest", "bricks.go")

	// The env injection string literal must be absent.
	if !acsassert.FileNotContains(t, bricksFile, `"EVOLVE_REQUIRE_INTENT"`) {
		t.Errorf("RED: bricks.go still contains the string literal \"EVOLVE_REQUIRE_INTENT\".\n"+
			"Builder must change IntentRequired() from:\n"+
			"  s.Env[\"EVOLVE_REQUIRE_INTENT\"] = \"1\"  (env injection, now deleted code path)\n"+
			"to:\n"+
			"  s.Enable[\"intent\"] = config.EnableOn  (PhaseEnabled pattern; engine uses s.Enable directly)\n"+
			"Routing engine uses PhaseEnable: s.Enable (engine.go:52); env injection was already\n"+
			"disconnected from routing behavior — this is a correctness fix, not just cleanup.\n"+
			"File: %s", bricksFile)
	}
}

// ---------------------------------------------------------------------------
// Task 2: consensus-audit-config-39
// ---------------------------------------------------------------------------

// TestC39_CA_001_ConsensusAuditAbsentFromRegistry verifies that EVOLVE_CONSENSUS_AUDIT
// is no longer registered after Builder removes its row from registry_table.go.
//
// Covers T2 AC1_CA. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
//
// RED: EVOLVE_CONSENSUS_AUDIT is currently in registry_table.go (StatusActive,
// Cluster="Workflow Defaults"); Lookup returns (flag, true).
func TestC39_CA_001_ConsensusAuditAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_CONSENSUS_AUDIT"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove the EVOLVE_CONSENSUS_AUDIT row from registry_table.go\n"+
			"(consensus-audit-config-39: migrate to WorkflowPolicy.ConsensusAuditEnabled).\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_CONSENSUS_AUDIT", f.Status, f.Cluster)
	}
}

// TestC39_CA_002_RegistryRowCountIs73 verifies that after removing all 7 rows
// (6 legacyFlags + 1 CONSENSUS_AUDIT across both tasks) the total registry count
// is exactly 73.
//
// Covers T2 AC2_CA (FINAL state after both tasks). BEHAVIORAL: calls
// len(flagregistry.All) — the production count. Over-removal (<73) and
// under-removal (>73) both fail.
//
// NOTE: T1 AC2 (count=74) is the INTERMEDIATE state after Task 1 alone. Since
// the audit suite runs after Builder finishes BOTH tasks, this predicate checks
// the FINAL state. Same pattern as cycle-38 (C38_GC_002 checked FINAL 80).
//
// RED: registry currently has 80 rows (FlagCeiling=80).
func TestC39_CA_002_RegistryRowCountIs73(t *testing.T) {
	got := len(flagregistry.All)
	if got != 73 {
		t.Errorf("RED: len(flagregistry.All) = %d, want 73 (80 − 7 removed flags).\n"+
			"Builder must remove exactly 7 rows from registry_table.go:\n"+
			"  Task 1: EVOLVE_REQUIRE_INTENT, EVOLVE_TRIAGE_DISABLE, EVOLVE_PLAN_REVIEW,\n"+
			"          EVOLVE_TEST_PHASE_ENABLED, EVOLVE_BUILD_PLANNER, EVOLVE_SWARM_PLANNER\n"+
			"  Task 2: EVOLVE_CONSENSUS_AUDIT\n"+
			"Current count: %d", got, got)
	}
}

// TestC39_CA_003_FlagCeilingConstIs73 verifies that the FlagCeiling ratchet constant
// has been updated to 73 in registry_ceiling_test.go (FINAL state after both tasks).
//
// Covers T2 AC3_CA (FINAL state). The ratchet prevents accidental registry growth;
// lowering it by 7 (80−7=73) is mandatory alongside the row removals.
//
// NOTE: T1 AC3 (FlagCeiling=74) is the INTERMEDIATE state. The audit validates the
// FINAL ceiling of 73.
//
// acs-predicate: config-check
//
// RED: registry_ceiling_test.go currently has FlagCeiling = 80.
func TestC39_CA_003_FlagCeilingConstIs73(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	ceilingFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_ceiling_test.go")
	if !acsassert.FileContains(t, ceilingFile, "FlagCeiling = 73") {
		t.Errorf("RED: registry_ceiling_test.go does not contain 'FlagCeiling = 73'.\n"+
			"Builder must lower the FlagCeiling constant to 73 in the same diff as\n"+
			"removing all 7 registry rows (80 − 6 legacyFlags − 1 CONSENSUS_AUDIT = 73).\n"+
			"File: %s", ceilingFile)
	}
}

// TestC39_CA_004_NoProdConsensusAuditEnvReads verifies that the os.Getenv string
// literal for EVOLVE_CONSENSUS_AUDIT has been deleted from cmd_consensus_dispatch.go.
//
// Covers T2 AC4_CA. Anti-gaming (cycle-8 split-const lesson): Builder cannot remove
// the registry row while leaving the os.Getenv("EVOLVE_CONSENSUS_AUDIT") call site.
// cmd_consensus_dispatch.go must load policy.Load(...) instead.
//
// acs-predicate: config-check
//
// RED: cmd_consensus_dispatch.go currently has ConsensusEnvOff: os.Getenv("EVOLVE_CONSENSUS_AUDIT") == "0"
// at line 31. The quoted string literal "EVOLVE_CONSENSUS_AUDIT" must be absent after migration.
func TestC39_CA_004_NoProdConsensusAuditEnvReads(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	dispatchFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_consensus_dispatch.go")
	literal := `"EVOLVE_CONSENSUS_AUDIT"`
	if !acsassert.FileNotContains(t, dispatchFile, literal) {
		t.Errorf("RED: cmd_consensus_dispatch.go still contains the string literal %s.\n"+
			"Builder must replace os.Getenv(\"EVOLVE_CONSENSUS_AUDIT\") == \"0\" with:\n"+
			"  pol, _ := policy.Load(filepath.Join(projectRoot, \".evolve\", \"policy.json\"))\n"+
			"  ConsensusEnvOff: !pol.WorkflowConfig().ConsensusAuditEnabled\n"+
			"(cycle-8 anti-gaming: removing the registry row without deleting the os.Getenv\n"+
			"call is the split-const hiding pattern).\n"+
			"File: %s", literal, dispatchFile)
	}
}

// TestC39_CA_005_ConsensusAuditEnabledDefaultsTrue verifies that:
//  1. policy.WorkflowPolicy has a ConsensusAuditEnabled *bool field.
//  2. policy.WorkflowConfig has a ConsensusAuditEnabled bool field.
//  3. The WorkflowConfig() resolver defaults ConsensusAuditEnabled to true when
//     the Workflow block is absent or ConsensusAuditEnabled is nil.
//  4. An explicit *false pointer in WorkflowPolicy disables it (returns false).
//
// Covers T2 AC5_CA. BEHAVIORAL (compile-fail RED): directly accesses
// WorkflowConfig.ConsensusAuditEnabled. Until Builder adds these fields to
// WorkflowPolicy and WorkflowConfig and wires the resolver, this test FAILS
// TO COMPILE — a compile failure IS the RED state.
//
// RED: WorkflowPolicy (policy.go:462-473) does not have ConsensusAuditEnabled.
// WorkflowConfig (policy.go:476-486) does not have ConsensusAuditEnabled.
// This test does not compile.
func TestC39_CA_005_ConsensusAuditEnabledDefaultsTrue(t *testing.T) {
	// Zero-value default: absent Workflow block must resolve ConsensusAuditEnabled=true
	// (matches current behavior: ConsensusEnvOff defaults to false when env is "" or "1").
	dflt := policy.Policy{}.WorkflowConfig()
	if !dflt.ConsensusAuditEnabled {
		t.Errorf("RED: policy.Policy{}.WorkflowConfig().ConsensusAuditEnabled = false, want true.\n" +
			"Builder must add ConsensusAuditEnabled bool to WorkflowConfig with default=true\n" +
			"in WorkflowConfig() resolver (nil ConsensusAuditEnabled in WorkflowPolicy → on).")
	}

	// Explicit false override must be respected (operator can disable via policy.json).
	f := false
	disabled := policy.Policy{
		Workflow: &policy.WorkflowPolicy{
			ConsensusAuditEnabled: &f,
		},
	}.WorkflowConfig()
	if disabled.ConsensusAuditEnabled {
		t.Errorf("RED: ConsensusAuditEnabled with *bool=false pointer = true, want false.\n" +
			"Builder must honor explicit ConsensusAuditEnabled=false in policy.json:\n" +
			"  if p.Workflow.ConsensusAuditEnabled != nil {\n" +
			"    c.ConsensusAuditEnabled = *p.Workflow.ConsensusAuditEnabled\n" +
			"  } else { c.ConsensusAuditEnabled = true }")
	}

	// Explicit true override must also be preserved.
	tr := true
	enabled := policy.Policy{
		Workflow: &policy.WorkflowPolicy{
			ConsensusAuditEnabled: &tr,
		},
	}.WorkflowConfig()
	if !enabled.ConsensusAuditEnabled {
		t.Errorf("RED: ConsensusAuditEnabled with *bool=true pointer = false, want true.\n" +
			"Builder must propagate explicit ConsensusAuditEnabled=true from WorkflowPolicy\n" +
			"through to WorkflowConfig via the pointer dereference in the resolver.")
	}
}

// TestC39_CA_NEG1_NoConsensusAuditIPCWrite verifies that the redundant IPC write
// out["EVOLVE_CONSENSUS_AUDIT"] = "1" has been removed from cmd_loop_args.go.
//
// Covers T2 NEG1_CA. This line was functionally a no-op (ConsensusEnvOff defaults
// to false when env is "" or "1"; the IPC write always set it to "1" = default).
// After the migration, it is dead code that references a deleted env path and must
// be removed.
//
// acs-predicate: config-check
//
// RED: cmd_loop_args.go currently has out["EVOLVE_CONSENSUS_AUDIT"] = "1" at line 270.
func TestC39_CA_NEG1_NoConsensusAuditIPCWrite(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	loopArgsFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_args.go")
	if !acsassert.FileNotContains(t, loopArgsFile, `"EVOLVE_CONSENSUS_AUDIT"`) {
		t.Errorf("RED: cmd_loop_args.go still contains the string literal \"EVOLVE_CONSENSUS_AUDIT\".\n"+
			"Builder must delete the line `out[\"EVOLVE_CONSENSUS_AUDIT\"] = \"1\"` at line ~270.\n"+
			"This IPC write was a no-op (default ConsensusEnvOff=false when env==\"\" or \"1\").\n"+
			"After migration, consensus audit is controlled via WorkflowPolicy.ConsensusAuditEnabled\n"+
			"in policy.json, not env injection.\n"+
			"File: %s", loopArgsFile)
	}
}
