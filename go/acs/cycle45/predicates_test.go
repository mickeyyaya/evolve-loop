//go:build acs

// Package cycle45 materializes the cycle-45 acceptance criteria for two tasks:
//
//	Task 1: gobin-planworkspace-di-45 — remove 2 flags from the operator registry:
//	  EVOLVE_GO_BIN_TEST    → Bucket 3 (DI): goBinFn func() string param injected into
//	                         defaultSimulationRunner; removes os.Getenv("EVOLVE_GO_BIN_TEST")
//	  EVOLVE_PLAN_WORKSPACE → Bucket 6 (CLI flag): --workspace flag on plan-and-execute;
//	                         removes os.Getenv("EVOLVE_PLAN_WORKSPACE")
//	Lower FlagCeiling 65 → 63.
//
//	Task 2: codex-version-compact-di-45 — remove 2 more flags:
//	  EVOLVE_CODEX_VERSION_PATH → Bucket 3 (DI via pkg-level var): codexVersionPathFn;
//	                             removes os.Getenv("EVOLVE_CODEX_VERSION_PATH")
//	  EVOLVE_COMPACT_PROMPTS    → Bucket 1 (Config Object): Options.CompactPrompts bool;
//	                             removes envchain.Bool("EVOLVE_COMPACT_PROMPTS", req.Env, false)
//	Lower FlagCeiling 63 → 61.
//
// AC map (1:1 with triage top_n):
//
//	Task 1 (gobin-planworkspace-di-45):
//	 AC1  EVOLVE_GO_BIN_TEST absent from registry         → C45_001 (behavioral: Lookup)
//	 AC2  EVOLVE_PLAN_WORKSPACE absent from registry      → C45_002 (behavioral: Lookup)
//	 AC3  No prod Getenv for EVOLVE_GO_BIN_TEST           → C45_003 (config-check, waiver)
//	 AC4  No prod Getenv for EVOLVE_PLAN_WORKSPACE        → C45_004 (config-check, waiver)
//	 AC5  PLAN_WORKSPACE removed from allowedUndocumented → C45_005 (config-check, waiver)
//	 AC6  go test ./internal/releasepreflight/... passes  → manual+checklist (Auditor)
//
//	Task 2 (codex-version-compact-di-45):
//	 AC1  EVOLVE_CODEX_VERSION_PATH absent from registry  → C45B_001 (behavioral: Lookup)
//	 AC2  EVOLVE_COMPACT_PROMPTS absent from registry     → C45B_002 (behavioral: Lookup)
//	 AC3  No prod Getenv for EVOLVE_CODEX_VERSION_PATH    → C45B_003 (config-check, waiver)
//	 AC4  No envchain.Bool for EVOLVE_COMPACT_PROMPTS     → C45B_004 (config-check, waiver)
//	 AC5  FlagCeiling == 61                               → C45_006 (config-check, waiver)
//	 AC6  go test ./internal/bridge/... ./internal/phases/runner/... → manual+checklist
//	 AC7  control-flags.md clean (4 flags absent)         → C45_007 (config-check, waiver)
//	 NEG  Exact registry count == 61                      → C45_NEG (behavioral: len)
//
// ACs with manual+checklist disposition:
//
//	AC6-Task1 (releasepreflight suite passes):
//	  Checklist for Auditor:
//	  (a) exit 0 from `cd go && go test ./internal/releasepreflight/... ./cmd/evolve/... ./internal/flagregistry/...`
//	  (b) no FAIL packages in output
//	  (c) `go build ./...` exits 0 from go/
//
//	AC6-Task2 (bridge + runner suite passes):
//	  Checklist for Auditor:
//	  (a) exit 0 from `cd go && go test ./internal/bridge/... ./internal/phases/runner/... ./internal/flagregistry/...`
//	  (b) no FAIL packages in output
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C45_001/002/C45B_001/002 — all 4 flags ABSENT from Lookup (any hit = flag still registered).
//	            C45_NEG_ExactRowCountIs61 — registry EXACTLY 61; over- or under-removal fails.
//	Edge/OOD:   C45_NEG_ExactRowCountIs61 catches both <61 (over-removal) and >61 (under-removal).
//	Lexical:    Lookup / len / FileNotContains / FileContains — distinct assertion verbs.
//	Semantic:   registry-absence (4 flags), exact-row-count, prod-source-clean (4 files),
//	            docs-contract-hygiene, ceiling-const, control-flags-doc — 6 distinct dimensions.
//
// Floor binding (R9.3): predicates authored ONLY for gobin-planworkspace-di-45 and
// codex-version-compact-di-45 (both in triage top_n). Deferred tasks get zero predicates.
//
// 1:1 enforcement:
//
//	predicate count: 11 funcs (C45_001-007, C45B_001-004, C45_NEG)
//	manual+checklist: 2 (AC6-Task1, AC6-Task2 — checklist addressed to Auditor above)
//	unverifiable-remove: 0
//	Total AC count: 13; every AC has exactly one disposition row.
package cycle45

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// allRemovedFlags is the canonical list of 4 env flags removed in cycle 45.
var allRemovedFlags = []string{
	"EVOLVE_GO_BIN_TEST",
	"EVOLVE_PLAN_WORKSPACE",
	"EVOLVE_CODEX_VERSION_PATH",
	"EVOLVE_COMPACT_PROMPTS",
}

// === Task 1: gobin-planworkspace-di-45 ===

// TestC45_001_GoBinTestAbsentFromRegistry verifies that EVOLVE_GO_BIN_TEST is no
// longer registered in the flag registry after the DI injection migration.
//
// Covers Task1 AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
// A source edit alone cannot satisfy this; the registry row must be absent.
//
// RED: EVOLVE_GO_BIN_TEST is currently registered at registry_table.go:31.
func TestC45_001_GoBinTestAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_GO_BIN_TEST"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (gobin-planworkspace-di-45: DI inject).\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_GO_BIN_TEST", f.Status, f.Cluster)
	}
}

// TestC45_002_PlanWorkspaceAbsentFromRegistry verifies that EVOLVE_PLAN_WORKSPACE is
// no longer registered in the flag registry after the CLI flag migration.
//
// Covers Task1 AC2. BEHAVIORAL: calls flagregistry.Lookup().
//
// RED: EVOLVE_PLAN_WORKSPACE is currently registered at registry_table.go:49.
func TestC45_002_PlanWorkspaceAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_PLAN_WORKSPACE"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (gobin-planworkspace-di-45: CLI flag).\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_PLAN_WORKSPACE", f.Status, f.Cluster)
	}
}

// TestC45_003_GoBinTestAbsentFromProdSource verifies that os.Getenv("EVOLVE_GO_BIN_TEST")
// has been deleted from releasepreflight.go and replaced with DI injection.
//
// acs-predicate: config-check
//
// RED: releasepreflight.go:215 currently has: goBin := os.Getenv("EVOLVE_GO_BIN_TEST").
func TestC45_003_GoBinTestAbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "releasepreflight", "releasepreflight.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_GO_BIN_TEST"`) {
		t.Errorf("RED: releasepreflight.go still contains the env read \"EVOLVE_GO_BIN_TEST\".\n"+
			"Builder must delete: goBin := os.Getenv(\"EVOLVE_GO_BIN_TEST\") (line 215)\n"+
			"and replace defaultSimulationRunner's env read with a goBinFn func() string param.\n"+
			"File: %s", f)
	}
}

// TestC45_004_PlanWorkspaceAbsentFromProdSource verifies that os.Getenv("EVOLVE_PLAN_WORKSPACE")
// has been deleted from cmd_plan_and_execute.go and replaced with a --workspace CLI flag.
//
// acs-predicate: config-check
//
// RED: cmd_plan_and_execute.go:70 currently has: os.Getenv("EVOLVE_PLAN_WORKSPACE").
func TestC45_004_PlanWorkspaceAbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_plan_and_execute.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_PLAN_WORKSPACE"`) {
		t.Errorf("RED: cmd_plan_and_execute.go still contains the env read \"EVOLVE_PLAN_WORKSPACE\".\n"+
			"Builder must delete the os.Getenv(\"EVOLVE_PLAN_WORKSPACE\") call (line 70)\n"+
			"and add a --workspace flag to the plan-and-execute flag set.\n"+
			"File: %s", f)
	}
}

// TestC45_005_PlanWorkspaceRemovedFromDocsContract verifies that EVOLVE_PLAN_WORKSPACE
// has been removed from the allowedUndocumented map in docs_contract_test.go.
//
// acs-predicate: config-check
//
// RED: docs_contract_test.go:58 currently has "EVOLVE_PLAN_WORKSPACE": true in the map.
func TestC45_005_PlanWorkspaceRemovedFromDocsContract(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "docs_contract_test.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_PLAN_WORKSPACE"`) {
		t.Errorf("RED: docs_contract_test.go still has \"EVOLVE_PLAN_WORKSPACE\" in allowedUndocumented.\n"+
			"Builder must remove the entry \"EVOLVE_PLAN_WORKSPACE\": true (line 58).\n"+
			"After removal from the registry the allowedUndocumented entry is stale.\n"+
			"File: %s", f)
	}
}

// === Task 2: codex-version-compact-di-45 ===

// TestC45B_001_CodexVersionPathAbsentFromRegistry verifies that EVOLVE_CODEX_VERSION_PATH
// is no longer registered in the flag registry after the DI var migration.
//
// Covers Task2 AC1. BEHAVIORAL: calls flagregistry.Lookup().
//
// RED: EVOLVE_CODEX_VERSION_PATH is currently registered at registry_table.go:20.
func TestC45B_001_CodexVersionPathAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_CODEX_VERSION_PATH"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (codex-version-compact-di-45: DI var).\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_CODEX_VERSION_PATH", f.Status, f.Cluster)
	}
}

// TestC45B_002_CompactPromptsAbsentFromRegistry verifies that EVOLVE_COMPACT_PROMPTS
// is no longer registered in the flag registry after the Config Object migration.
//
// Covers Task2 AC2. BEHAVIORAL: calls flagregistry.Lookup().
//
// RED: EVOLVE_COMPACT_PROMPTS is currently registered at registry_table.go:22.
func TestC45B_002_CompactPromptsAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_COMPACT_PROMPTS"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (codex-version-compact-di-45: Config Object).\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_COMPACT_PROMPTS", f.Status, f.Cluster)
	}
}

// TestC45B_003_CodexVersionPathAbsentFromProdSource verifies that os.Getenv("EVOLVE_CODEX_VERSION_PATH")
// has been deleted from codex_pretrust.go and replaced with codexVersionPathFn.
//
// acs-predicate: config-check
//
// RED: codex_pretrust.go:153 currently has: os.Getenv("EVOLVE_CODEX_VERSION_PATH").
func TestC45B_003_CodexVersionPathAbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "bridge", "codex_pretrust.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_CODEX_VERSION_PATH"`) {
		t.Errorf("RED: codex_pretrust.go still contains the env read \"EVOLVE_CODEX_VERSION_PATH\".\n"+
			"Builder must delete the os.Getenv(\"EVOLVE_CODEX_VERSION_PATH\") call (line 153)\n"+
			"and add: var codexVersionPathFn func() (string, error) = defaultCodexVersionPath.\n"+
			"File: %s", f)
	}
}

// TestC45B_004_CompactPromptsAbsentFromProdSource verifies that the envchain.Bool call
// for "EVOLVE_COMPACT_PROMPTS" has been deleted from runner.go and replaced with
// b.compactPrompts (sourced from opts.CompactPrompts in New()).
//
// acs-predicate: config-check
//
// RED: runner.go:277 currently has: envchain.Bool("EVOLVE_COMPACT_PROMPTS", req.Env, false).
func TestC45B_004_CompactPromptsAbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "phases", "runner", "runner.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_COMPACT_PROMPTS"`) {
		t.Errorf("RED: runner.go still contains the envchain.Bool(\"EVOLVE_COMPACT_PROMPTS\") call (line 277).\n"+
			"Builder must replace it with b.compactPrompts (sourced from opts.CompactPrompts in New()).\n"+
			"File: %s", f)
	}
}

// === Shared: FlagCeiling + control-flags doc ===

// TestC45_007_ControlFlagsDocClean verifies that the regenerated
// docs/architecture/control-flags.md no longer contains entries for any of the 4 removed flags.
//
// acs-predicate: config-check
//
// RED: control-flags.md currently has rows for all 4 flags.
// After migration, Builder must run `evolve flags generate` and commit the updated doc.
func TestC45_007_ControlFlagsDocClean(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range allRemovedFlags {
		if !acsassert.FileNotContains(t, controlFlagsDoc, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must regenerate docs/architecture/control-flags.md after removing\n"+
				"all 4 flag rows (run `evolve flags generate` in the same diff).\n"+
				"File path: %s", name, controlFlagsDoc)
		}
	}
}
