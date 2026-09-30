//go:build acs

package cycle48

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC48A_001_CachePrefixV2_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_CACHE_PREFIX_V2"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (cache-prefix-v2-dead-field-48).\n"+
			"RunRequest.CachePrefixV2 is never read in Run(); this env read has zero runtime effect.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_CACHE_PREFIX_V2", f.Status, f.Cluster)
	}
}

// acs-predicate: config-check
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

// acs-predicate: config-check
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

// acs-predicate: config-check
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

func TestC48B_001_GuardsLog_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_GUARDS_LOG"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (guards-log-di-48: DI migration).\n"+
			"The env read must be removed from appendGuardsLog; call site must compute logPath directly.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_GUARDS_LOG", f.Status, f.Cluster)
	}
}

// acs-predicate: config-check
func TestC48B_002_GuardsLog_AbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "cli", "guardcmd", "guard.go")
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

// acs-predicate: config-check
func TestC48B_003_AppendGuardsLog_HasLogPathParam(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "cli", "guardcmd", "guard.go")
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

// acs-predicate: config-check
func TestC48B_004_GuardTest_NoSetenvGuardsLog(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "cli", "guardcmd", "guard_test.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_GUARDS_LOG"`) {
		t.Errorf("RED: cmd_guard_test.go still references \"EVOLVE_GUARDS_LOG\".\n"+
			"Builder must replace t.Setenv(\"EVOLVE_GUARDS_LOG\", ...) with direct path injection:\n"+
			"  TestAppendGuardsLog_EnvOverride: remove t.Setenv; pass custom directly as first arg\n"+
			"  TestAppendGuardsLog_UnwritablePathSilent: remove t.Setenv; pass unwritable path directly\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
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
