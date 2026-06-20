//go:build acs

// Package cycle48 materializes the cycle-48 acceptance criteria for two tasks:
//
//	cache-prefix-v2-dead-field-48 — remove dead field EVOLVE_CACHE_PREFIX_V2:
//	  RunRequest.CachePrefixV2 is never read in Run(); env read at cmd_subagent.go:507
//	  has zero runtime effect. Pure no-op removal: build-time struct literal checks
//	  catch any missed test update before test execution.
//	  Lower FlagCeiling 56 → 55.
//
//	guards-log-di-48 — DI migration for EVOLVE_GUARDS_LOG:
//	  change appendGuardsLog(evolveDir, ...) to appendGuardsLog(logPath, ...)
//	  and compute logPath at the call site (cmd_guard.go:122).
//	  Lower FlagCeiling 55 → 54.
//
// AC map (1:1 with triage top_n for both tasks):
//
//	=== Task A: cache-prefix-v2-dead-field-48 ===
//	AC1  EVOLVE_CACHE_PREFIX_V2 absent from registry        → C48A_001 (behavioral: Lookup)
//	AC2  No prod env read for CACHE_PREFIX_V2               → C48A_002 (config-check, waiver)
//	AC3  CachePrefixV2 field absent from run.go             → C48A_003 (config-check, waiver)
//	AC4  FlagCeiling == 55                                  → C48A_004 (config-check, waiver)
//	AC5  cmd_subagent_env_test.go zero CACHE_PREFIX_V2 refs → C48A_005 (config-check, waiver)
//	AC6  go test ./internal/subagent/... PASS               → manual+checklist (Auditor)
//	AC7  go test ./cmd/evolve/... PASS                      → manual+checklist (Auditor)
//	AC8  go test ./internal/flagregistry/... PASS           → manual+checklist (Auditor)
//	AC9  flagreaders ACS guard PASS                         → manual+checklist (Auditor)
//	NEG  row count ≤ 55 after Task A flags removed          → C48A_NEG (behavioral: len)
//
//	=== Task B: guards-log-di-48 ===
//	AC1  EVOLVE_GUARDS_LOG absent from registry             → C48B_001 (behavioral: Lookup)
//	AC2  No prod os.Getenv read for GUARDS_LOG              → C48B_002 (config-check, waiver)
//	AC3  appendGuardsLog first param is logPath string      → C48B_003 (config-check, waiver)
//	AC4  Zero t.Setenv("EVOLVE_GUARDS_LOG") in test files   → C48B_004 (config-check, waiver)
//	AC5  FlagCeiling == 54                                  → C48B_005 (config-check, waiver)
//	AC6  docs_contract_test.go zero GUARDS_LOG refs         → C48B_006 (config-check, waiver)
//	AC7  go test ./cmd/evolve/... PASS                      → manual+checklist (Auditor)
//	AC8  go test ./internal/flagregistry/... PASS           → manual+checklist (Auditor)
//	AC9  flagreaders ACS guard PASS                         → manual+checklist (Auditor)
//	NEG  exact row count == 54 (final state after both)     → C48B_NEG (behavioral: len)
//
// Manual+checklist ACs (addressed to Auditor):
//
//	Task A AC6 (subagent tests pass):
//	  (a) exit 0: cd go && go test ./internal/subagent/...
//	  (b) no FAIL packages in output
//
//	Task A AC7 (cmd/evolve tests pass):
//	  (a) exit 0: cd go && go test ./cmd/evolve/...
//	  (b) no FAIL packages in output
//
//	Task A AC8 (flagregistry tests pass):
//	  (a) exit 0: cd go && go test ./internal/flagregistry/...
//	  (b) TestRegistry_FlagCeiling passes (FlagCeiling == 55 after Task A, == 54 after Task B)
//
//	Task A AC9 (flagreaders ACS guard):
//	  (a) go test -tags acs ./acs/regression/flagreaders/...
//	  (b) EVOLVE_CACHE_PREFIX_V2 does not appear as an orphan reader
//
//	Task B AC7 (cmd/evolve tests pass):
//	  (a) exit 0: cd go && go test ./cmd/evolve/...
//	  (b) no FAIL packages; TestAppendGuardsLog_* pass with DI path injection
//
//	Task B AC8 (flagregistry tests pass):
//	  (a) exit 0: cd go && go test ./internal/flagregistry/...
//	  (b) TestRegistry_FlagCeiling passes (FlagCeiling == 54)
//
//	Task B AC9 (flagreaders ACS guard):
//	  (a) go test -tags acs ./acs/regression/flagreaders/...
//	  (b) EVOLVE_GUARDS_LOG does not appear as an orphan reader
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C48A_001/C48B_001 — flags ABSENT from Lookup (any hit = still registered).
//	            C48A_NEG: row count ≤ 55 (upper bound, allows Task B to apply in same build).
//	            C48B_NEG: exact count == 54 (catches both over-removal <54 and under-removal >54).
//	Edge/OOD:   C48B_NEG exact count rejects both directions; C48A_NEG is one-sided upper bound.
//	Lexical:    Lookup / len / FileNotContains / FileContains / FileMatchesRegex — five distinct verbs.
//	Semantic:   registry-absence (2 flags), env-read-clean (2 files), struct-field-absent (run.go),
//	            DI-signature (cmd_guard.go), test-clean (2 test files), ceiling-const (2 values),
//	            docs-contract-clean (docs_contract_test.go), exact-row-count — 10 dimensions.
//
// Floor binding (R9.3): predicates authored ONLY for tasks in the triage top_n.
// Deferred tasks (EVOLVE_MODELCATALOG_AUTOREFRESH, EVOLVE_FORCE_FRESH, etc.) get zero predicates.
//
// 1:1 enforcement:
//
//	Task A: predicate=6 (C48A_001–005, C48A_NEG), manual+checklist=4 (AC6/AC7/AC8/AC9),
//	        unverifiable-remove=0 → total AC=10 ✓
//	Task B: predicate=7 (C48B_001–006, C48B_NEG), manual+checklist=3 (AC7/AC8/AC9),
//	        unverifiable-remove=0 → total AC=10 ✓
package cycle48

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// =============================================================================
// Task A — cache-prefix-v2-dead-field-48
// =============================================================================

// === Registry absence (behavioral: Lookup) ===

// TestC48A_001_CachePrefixV2_AbsentFromRegistry verifies that
// EVOLVE_CACHE_PREFIX_V2 is no longer registered after the dead field removal.
// RunRequest.CachePrefixV2 is never read in Run(); the env read at
// cmd_subagent.go:507 has zero runtime effect.
//
// Covers Task A AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
// Adding a source comment cannot satisfy this; the registry row must be absent.
//
// RED: EVOLVE_CACHE_PREFIX_V2 is currently registered at registry_table.go with
// Status=StatusActive, Cluster="Observability / Prompt Tuning".
func TestC48A_001_CachePrefixV2_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_CACHE_PREFIX_V2"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (cache-prefix-v2-dead-field-48).\n"+
			"RunRequest.CachePrefixV2 is never read in Run(); this env read has zero runtime effect.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_CACHE_PREFIX_V2", f.Status, f.Cluster)
	}
}

// === Prod-source clean (config-check waiver) ===

// TestC48A_002_CachePrefixV2_AbsentFromProdSource verifies that the env read
// envchain.Bool("EVOLVE_CACHE_PREFIX_V2", ...) has been removed from
// cmd_subagent.go, along with the struct field assignment and help-text references.
//
// acs-predicate: config-check
//
// RED: cmd_subagent.go:507 currently has:
//
//	cachePrefixV2: envchain.Bool("EVOLVE_CACHE_PREFIX_V2", nil, true),
//
// and cmd_subagent.go:44,311 have doc/help references to EVOLVE_CACHE_PREFIX_V2.
func TestC48A_002_CachePrefixV2_AbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_subagent.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_CACHE_PREFIX_V2"`) {
		t.Errorf("RED: cmd_subagent.go still contains the env read \"EVOLVE_CACHE_PREFIX_V2\".\n"+
			"Builder must delete:\n"+
			"  line 507: cachePrefixV2: envchain.Bool(\"EVOLVE_CACHE_PREFIX_V2\", nil, true),\n"+
			"  line 363: CachePrefixV2: flags.cachePrefixV2,\n"+
			"  lines 44,311: doc/help references to EVOLVE_CACHE_PREFIX_V2\n"+
			"and remove the cachePrefixV2 bool field from subagentRunFlags.\n"+
			"File: %s", f)
	}
}

// === Dead struct field removal (config-check waiver) ===

// TestC48A_003_CachePrefixV2_FieldAbsentFromRunRequest verifies that the
// CachePrefixV2 bool field has been removed from RunRequest in run.go.
// The field was declared but never read inside the Run() function body.
//
// acs-predicate: config-check
//
// RED: go/internal/subagent/run.go:47 currently has:
//
//	CachePrefixV2          bool   // EVOLVE_CACHE_PREFIX_V2 (default true)
func TestC48A_003_CachePrefixV2_FieldAbsentFromRunRequest(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "subagent", "run.go")
	if !acsassert.FileNotContains(t, f, "CachePrefixV2") {
		t.Errorf("RED: go/internal/subagent/run.go still contains CachePrefixV2.\n"+
			"Builder must remove the dead struct field:\n"+
			"  line 47: CachePrefixV2 bool // EVOLVE_CACHE_PREFIX_V2 (default true)\n"+
			"and any comment references to CACHE_PREFIX_V2 (lines 24, 148).\n"+
			"The field is never read inside Run(); removing it is a pure no-op.\n"+
			"File: %s", f)
	}
}

// === FlagCeiling after Task A (config-check waiver) ===

// TestC48A_004_FlagCeilingIs55 verifies that the FlagCeiling ratchet constant has
// been updated to 55 after removing EVOLVE_CACHE_PREFIX_V2 (56 − 1 = 55).
//
// acs-predicate: config-check
//
// RED: registry_ceiling_test.go currently has FlagCeiling = 56.
func TestC48A_004_FlagCeilingIs55(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	ceilingFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_ceiling_test.go")
	if !acsassert.FileContains(t, ceilingFile, "FlagCeiling = 55") {
		t.Errorf("RED: registry_ceiling_test.go does not contain 'FlagCeiling = 55'.\n"+
			"Builder must lower the FlagCeiling constant to 55 in the same diff as\n"+
			"removing the EVOLVE_CACHE_PREFIX_V2 registry row (56 − 1 = 55).\n"+
			"NOTE: Task B will lower it further to 54 — this predicate accepts 55 as the\n"+
			"intermediate state; TestC48B_005 and TestRegistry_FlagCeiling enforce the final 54.\n"+
			"File: %s", ceilingFile)
	}
}

// === Test-file clean (config-check waiver) ===

// TestC48A_005_SubagentEnvTest_NoCachePrefixV2 verifies that
// cmd_subagent_env_test.go no longer references CACHE_PREFIX_V2 or cachePrefixV2.
// After removing the struct field, the test must not set or reference this env var.
//
// acs-predicate: config-check
//
// RED: cmd_subagent_env_test.go currently references cachePrefixV2 (struct field)
// and EVOLVE_CACHE_PREFIX_V2 (env key) in both test functions.
func TestC48A_005_SubagentEnvTest_NoCachePrefixV2(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_subagent_env_test.go")
	if !acsassert.FileNotContains(t, f, "CACHE_PREFIX_V2") {
		t.Errorf("RED: cmd_subagent_env_test.go still references CACHE_PREFIX_V2.\n"+
			"Builder must remove all EVOLVE_CACHE_PREFIX_V2 t.Setenv lines and\n"+
			"any cachePrefixV2 struct literal field from both test functions.\n"+
			"File: %s", f)
	}
	if !acsassert.FileNotContains(t, f, "cachePrefixV2") {
		t.Errorf("RED: cmd_subagent_env_test.go still references cachePrefixV2 (struct field).\n"+
			"Builder must remove CachePrefixV2: true / cachePrefixV2 from all struct literals.\n"+
			"File: %s", f)
	}
}

// === Negative: upper-bound row count after Task A (behavioral) ===

// TestC48A_NEG_RowCountAtMost55 verifies that after Task A the registry row count
// has dropped from 56 to at most 55. A ≤ 55 check (rather than exact == 55) allows
// Task B to also be applied in the same build without this predicate re-failing on
// the further-reduced count of 54.
//
// BEHAVIORAL: calls len(flagregistry.All) — the production count.
//
// RED: registry currently has 56 rows (FlagCeiling=56); 56 > 55 fails.
func TestC48A_NEG_RowCountAtMost55(t *testing.T) {
	got := len(flagregistry.All)
	if got > 55 {
		t.Errorf("RED: len(flagregistry.All) = %d, want ≤ 55 (56 − 1 Task A flag).\n"+
			"Builder must remove exactly this 1 row from registry_table.go:\n"+
			"  EVOLVE_CACHE_PREFIX_V2\n"+
			"Current count %d exceeds 55 — Task A flag not yet removed.",
			got, got)
	}
}

// =============================================================================
// Task B — guards-log-di-48
// =============================================================================

// === Registry absence (behavioral: Lookup) ===

// TestC48B_001_GuardsLog_AbsentFromRegistry verifies that EVOLVE_GUARDS_LOG is
// no longer registered after the DI migration. appendGuardsLog's first param
// changes from evolveDir to logPath; the call site at cmd_guard.go:122 computes
// logPath := filepath.Join(evolveDir, "guards.log") directly.
//
// Covers Task B AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
//
// RED: EVOLVE_GUARDS_LOG is currently registered at registry_table.go with
// Status=StatusInternal.
func TestC48B_001_GuardsLog_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_GUARDS_LOG"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (guards-log-di-48: DI migration).\n"+
			"The env read must be removed from appendGuardsLog; call site must compute logPath directly.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_GUARDS_LOG", f.Status, f.Cluster)
	}
}

// === Prod-source clean (config-check waiver) ===

// TestC48B_002_GuardsLog_AbsentFromProdSource verifies that os.Getenv("EVOLVE_GUARDS_LOG")
// has been removed from cmd_guard.go. After the DI migration, the call site
// (line 122) computes the path with filepath.Join(evolveDir, "guards.log").
//
// acs-predicate: config-check
//
// RED: cmd_guard.go:45 currently has:
//
//	logPath := os.Getenv("EVOLVE_GUARDS_LOG")
func TestC48B_002_GuardsLog_AbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_guard.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_GUARDS_LOG"`) {
		t.Errorf("RED: cmd_guard.go still contains the env read \"EVOLVE_GUARDS_LOG\".\n"+
			"Builder must delete line 45: logPath := os.Getenv(\"EVOLVE_GUARDS_LOG\")\n"+
			"and the fallback block (lines 46–48), replacing with logPath computed at the\n"+
			"call site in runGuard:\n"+
			"  logPath := filepath.Join(evolveDir, \"guards.log\")\n"+
			"  appendGuardsLog(logPath, name, dec.Allow, dec.Reason)\n"+
			"File: %s", f)
	}
}

// === DI signature check (config-check waiver) ===

// TestC48B_003_AppendGuardsLog_HasLogPathParam verifies that appendGuardsLog now
// accepts logPath as its first parameter (instead of evolveDir). The signature
// must NOT have the old os.Getenv-based internal path computation.
//
// acs-predicate: config-check
//
// RED: cmd_guard.go:39 currently has:
//
//	func appendGuardsLog(evolveDir, guardName string, allow bool, reason string) {
func TestC48B_003_AppendGuardsLog_HasLogPathParam(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_guard.go")
	if !acsassert.FileMatchesRegex(t, f, `func appendGuardsLog\(logPath[^)]*\)`) {
		t.Errorf("RED: cmd_guard.go does not contain an appendGuardsLog signature with logPath param.\n"+
			"Builder must change the signature from:\n"+
			"  func appendGuardsLog(evolveDir, guardName string, allow bool, reason string)\n"+
			"to:\n"+
			"  func appendGuardsLog(logPath, guardName string, allow bool, reason string)\n"+
			"and update the 2 test call sites in cmd_guard_test.go to pass logPath directly.\n"+
			"File: %s", f)
	}
}

// === Test-file clean (config-check waiver) ===

// TestC48B_004_GuardTest_NoSetenvGuardsLog verifies that cmd_guard_test.go no
// longer uses t.Setenv("EVOLVE_GUARDS_LOG", ...) to inject the log path.
// After the DI migration, tests pass the path directly as the first argument
// to appendGuardsLog(logPath, guardName, allow, reason).
//
// acs-predicate: config-check
//
// RED: cmd_guard_test.go:81,103 currently has t.Setenv("EVOLVE_GUARDS_LOG", ...).
func TestC48B_004_GuardTest_NoSetenvGuardsLog(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_guard_test.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_GUARDS_LOG"`) {
		t.Errorf("RED: cmd_guard_test.go still references \"EVOLVE_GUARDS_LOG\".\n"+
			"Builder must replace t.Setenv(\"EVOLVE_GUARDS_LOG\", ...) with direct path injection:\n"+
			"  TestAppendGuardsLog_EnvOverride: remove t.Setenv; pass custom directly as first arg\n"+
			"  TestAppendGuardsLog_UnwritablePathSilent: remove t.Setenv; pass unwritable path directly\n"+
			"File: %s", f)
	}
}

// === FlagCeiling after Task B (config-check waiver) ===

// TestC48B_005_FlagCeilingIs54 verifies that the FlagCeiling ratchet constant has
// been updated to 54 after removing EVOLVE_GUARDS_LOG (55 − 1 = 54, combining both
// tasks: 56 − 2 = 54).
//
// acs-predicate: config-check
//
// RED: registry_ceiling_test.go currently has FlagCeiling = 56.
func TestC48B_005_FlagCeilingIs54(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	ceilingFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_ceiling_test.go")
	if !acsassert.FileContains(t, ceilingFile, "FlagCeiling = 54") {
		t.Errorf("RED: registry_ceiling_test.go does not contain 'FlagCeiling = 54'.\n"+
			"Builder must lower the FlagCeiling constant to 54 in the Task B diff\n"+
			"(55 − 1 = 54 after removing EVOLVE_GUARDS_LOG row).\n"+
			"File: %s", ceilingFile)
	}
}

// === Docs-contract clean (config-check waiver) ===

// TestC48B_006_DocsContractTest_NoGuardsLog verifies that docs_contract_test.go
// no longer has an entry for EVOLVE_GUARDS_LOG. After the DI migration removes the
// flag from the registry, its docs_contract entry must also be deleted.
//
// acs-predicate: config-check
//
// RED: docs_contract_test.go:69 currently has:
//
//	"EVOLVE_GUARDS_LOG": true, // observability shunt
func TestC48B_006_DocsContractTest_NoGuardsLog(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "docs_contract_test.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_GUARDS_LOG"`) {
		t.Errorf("RED: docs_contract_test.go still contains \"EVOLVE_GUARDS_LOG\".\n"+
			"Builder must remove the line: \"EVOLVE_GUARDS_LOG\": true, // observability shunt\n"+
			"from the map in TestAllFlagsInRegistryAreDocumented (line 69).\n"+
			"Leaving it causes TestAllFlagsInRegistryAreDocumented to fail (registry no longer has the flag).\n"+
			"File: %s", f)
	}
}

// === Exact row count — final state (behavioral: negative / edge) ===

// TestC48B_NEG_ExactRowCountIs54 verifies that after both tasks are implemented the
// total registry count is exactly 54. This is the strongest invariant: over-removal
// (<54) AND under-removal (>54) both fail. It subsumes C48A_NEG's upper-bound check.
//
// BEHAVIORAL: calls len(flagregistry.All) — the production count.
//
// RED: registry currently has 56 rows (FlagCeiling=56); 56 ≠ 54 fails.
func TestC48B_NEG_ExactRowCountIs54(t *testing.T) {
	got := len(flagregistry.All)
	if got != 54 {
		t.Errorf("RED: len(flagregistry.All) = %d, want 54 (56 − 2 removed flags).\n"+
			"Builder must remove exactly 2 rows from registry_table.go:\n"+
			"  Task A: EVOLVE_CACHE_PREFIX_V2\n"+
			"  Task B: EVOLVE_GUARDS_LOG\n"+
			"Over-removal (<54) and under-removal (>54) both fail. Current count: %d",
			got, got)
	}
}
