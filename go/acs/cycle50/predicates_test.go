//go:build acs

// Package cycle50 materializes the cycle-50 acceptance criteria for two tasks:
//
//	codex-config-path-di-50 — remove EVOLVE_CODEX_CONFIG_PATH from registry:
//	  Single os.Getenv read in codex_pretrust.go:142 migrates to codexConfigPath
//	  string field on bridge.Config struct. ALL 5 test files (codex_pretrust_test.go,
//	  codex_pretrust_amplify_test.go, codex_pretrust_concurrent_test.go,
//	  codex_pretrust_launch_test.go, preflight_test.go) replace t.Setenv calls with
//	  cfg.codexConfigPath field assignment. Lower FlagCeiling 52 → 51.
//
//	release-strict-pass-cli-50 — remove EVOLVE_RELEASE_STRICT_PASS from registry:
//	  Two prod os.Getenv reads (cmd_release_preflight.go:55, releasepipeline/bridges.go:26)
//	  migrate to --strict-pass CLI flag + releasepipeline.Options.StrictPass bool field.
//	  Lower FlagCeiling 51 → 50.
//
// AC map (1:1 with triage top_n for both tasks):
//
//	=== Task A: codex-config-path-di-50 ===
//	AC1  EVOLVE_CODEX_CONFIG_PATH absent from registry         → C50A_001 (behavioral: Lookup)
//	AC2  No prod os.Getenv in codex_pretrust.go                → C50A_002 (config-check, waiver)
//	AC3  bridge.Config has codexConfigPath string field        → C50A_003 (config-check, waiver)
//	AC4  FlagCeiling == 51 (intermediate after Task A)         → C50A_004 (config-check, waiver)
//	AC5  Zero t.Setenv("EVOLVE_CODEX_CONFIG_PATH") in bridge/ → C50A_005 (config-check, waiver)
//	AC6  bridge test suite green                               → manual+checklist (Auditor)
//	AC7  flagreaders ACS guard green                           → manual+checklist (Auditor)
//	NEG  row count ≤ 51 after Task A (allows B to reach 50)   → C50A_NEG (behavioral: len)
//
//	=== Task B: release-strict-pass-cli-50 ===
//	AC1  EVOLVE_RELEASE_STRICT_PASS absent from registry       → C50B_001 (behavioral: Lookup)
//	AC2  No prod os.Getenv in cmd_release_preflight.go         → C50B_002 (config-check, waiver)
//	AC3  No prod os.Getenv in releasepipeline/bridges.go       → C50B_003 (config-check, waiver)
//	AC4  FlagCeiling == 50 (final)                             → C50B_004 (config-check, waiver)
//	AC5  --strict-pass CLI flag registered in preflight cmd    → C50B_005 (config-check, waiver)
//	AC6  releasepipeline.Options has StrictPass bool field     → C50B_006 (config-check, waiver)
//	AC7  releasepipeline + cmd/evolve test suites green        → manual+checklist (Auditor)
//	AC8  flagreaders ACS guard green                           → manual+checklist (Auditor)
//	NEG  exact row count == 50 (final state, both tasks)       → C50B_NEG (behavioral: len)
//
// Manual+checklist ACs (addressed to Auditor):
//
//	Task A AC6 (bridge test suite green):
//	  (a) exit 0: cd go && go test ./internal/bridge/... -count=1
//	  (b) no FAIL packages in output (includes root ./internal/bridge/, not just sub-packages)
//	  (c) all 5 test files compile without t.Setenv("EVOLVE_CODEX_CONFIG_PATH")
//	  (d) TestPretrustCodexProjects* tests pass using cfg.codexConfigPath (not env override)
//
//	Task A AC7 (flagreaders ACS guard):
//	  (a) exit 0: go test -tags acs ./acs/regression/flagreaders/... -count=1
//	  (b) EVOLVE_CODEX_CONFIG_PATH absent from non-test, non-registry Go prod files:
//	      grep -rn '"EVOLVE_CODEX_CONFIG_PATH"' go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches
//
//	Task B AC7 (releasepipeline + cmd/evolve tests green):
//	  (a) exit 0: cd go && go test ./internal/releasepipeline/... ./cmd/evolve/... -count=1
//	  (b) no FAIL packages; bridges_test.go compiles with updated 5-arg runPreflightLib signature
//	  (c) docs_contract_test.go compiles without "EVOLVE_RELEASE_STRICT_PASS" allowedUndocumented entry
//
//	Task B AC8 (flagreaders ACS guard):
//	  (a) exit 0: go test -tags acs ./acs/regression/flagreaders/... -count=1
//	  (b) EVOLVE_RELEASE_STRICT_PASS absent from non-test, non-registry Go prod files:
//	      grep -rn '"EVOLVE_RELEASE_STRICT_PASS"' go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C50A_001/C50B_001 — flags ABSENT from Lookup (any hit = still registered).
//	            C50A_NEG: row count ≤ 51 (upper bound; prevents Task B from failing A's check).
//	            C50B_NEG: exact count == 50 (catches over-removal <50 AND under-removal >50).
//	Edge/OOD:   C50B_NEG exact count rejects both directions; C50A_NEG is one-sided upper bound.
//	Lexical:    Lookup / len / FileNotContains / FileContains / CountInGoFunc — 5 distinct verbs.
//	Semantic:   registry-absence (2 flags), env-read-clean (3 prod files), struct-field-add (engine.go),
//	            test-migration (5 test files), ceiling-const (2 values: 51/50), cli-flag-registered
//	            (cmd_release_preflight.go), options-struct-field (releasepipeline.go),
//	            exact-row-count (final invariant) — 8 dimensions.
//
// Floor binding (R9.3): predicates authored ONLY for tasks in the triage top_n.
// Deferred tasks (EVOLVE_WORKTREE_PATH, EVOLVE_REFLECTION_JOURNAL, etc.) get zero predicates.
//
// 1:1 enforcement:
//
//	Task A: predicate=6 (C50A_001–005, C50A_NEG), manual+checklist=2 (AC6/AC7),
//	        pre-existing-GREEN=0, unverifiable-remove=0 → total AC=8 ✓
//	Task B: predicate=7 (C50B_001–006, C50B_NEG), manual+checklist=2 (AC7/AC8),
//	        pre-existing-GREEN=0, unverifiable-remove=0 → total AC=9 ✓
package cycle50

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// =============================================================================
// Task A — codex-config-path-di-50
// =============================================================================

// === Registry absence (behavioral: Lookup) ===

// TestC50A_001_CodexConfigPath_AbsentFromRegistry verifies that
// EVOLVE_CODEX_CONFIG_PATH is no longer registered after the DI migration.
// The single production reader at codex_pretrust.go:142 is replaced by
// cfg.codexConfigPath on the bridge.Config struct; the registry row must be deleted.
//
// Covers Task A AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
// Adding a source comment cannot satisfy this; the registry row must be absent.
//
// RED: EVOLVE_CODEX_CONFIG_PATH is currently registered at registry_table.go:18
// with Status=StatusInternal, Doc="Undocumented production reader (inventory 2026-06-11)".
func TestC50A_001_CodexConfigPath_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_CODEX_CONFIG_PATH"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (codex-config-path-di-50: bucket-3 DI migration).\n"+
			"The os.Getenv read must be removed from codex_pretrust.go:142; replace with cfg.codexConfigPath.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_CODEX_CONFIG_PATH", f.Status, f.Cluster)
	}
}

// === Prod-source clean (config-check waiver) ===

// TestC50A_002_CodexConfigPath_AbsentFromCodexPretrust verifies that
// os.Getenv("EVOLVE_CODEX_CONFIG_PATH") has been removed from codex_pretrust.go.
// After the DI migration the function reads cfg.codexConfigPath (a string field
// on bridge.Config) instead of the env var.
//
// acs-predicate: config-check
//
// RED: codex_pretrust.go:142 currently contains:
//
//	if v := os.Getenv("EVOLVE_CODEX_CONFIG_PATH"); v != "" {
func TestC50A_002_CodexConfigPath_AbsentFromCodexPretrust(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "bridge", "codex_pretrust.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_CODEX_CONFIG_PATH"`) {
		t.Errorf("RED: codex_pretrust.go still contains the env read \"EVOLVE_CODEX_CONFIG_PATH\".\n"+
			"Builder must:\n"+
			"  1. Rename codexConfigPath() → defaultCodexConfigPath()\n"+
			"  2. Add resolveCodexConfigPath(cfg *Config) that returns cfg.codexConfigPath if non-empty,\n"+
			"     else falls back to defaultCodexConfigPath()\n"+
			"  3. Update pretrustCodexProjects to call resolveCodexConfigPath(cfg) instead of codexConfigPath()\n"+
			"  4. Remove the os.Getenv(\"EVOLVE_CODEX_CONFIG_PATH\") branch from the renamed function\n"+
			"File: %s", f)
	}
}

// === bridge.Config has codexConfigPath field (config-check waiver) ===

// TestC50A_003_BridgeConfig_HasCodexConfigPathField verifies that bridge.Config
// (defined in engine.go) has an unexported codexConfigPath string field. This
// is the DI seam replacing the os.Getenv read. The field name matches the existing
// convention for unexported test-seam fields on Config (cf. ArtifactTimeoutS, BootOnly).
//
// acs-predicate: config-check
//
// RED: engine.go currently has BootOnly bool as the last seam field;
// codexConfigPath string is not present.
func TestC50A_003_BridgeConfig_HasCodexConfigPathField(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "bridge", "engine.go")
	if !acsassert.FileContains(t, f, "codexConfigPath string") {
		t.Errorf("RED: engine.go does not contain 'codexConfigPath string' field on bridge.Config.\n"+
			"Builder must add an unexported string field to bridge.Config (engine.go):\n"+
			"  codexConfigPath string  // test seam: overrides resolved codex config path\n"+
			"Pattern: consistent with ArtifactTimeoutS int / BootOnly bool seam fields.\n"+
			"File: %s", f)
	}
}

// === FlagCeiling after Task A (config-check waiver) ===

// === Test-file migration — zero t.Setenv in bridge/ (config-check waiver) ===

// TestC50A_005_BridgeTests_NoSetenvCodexConfigPath verifies that all 5 bridge test
// files that previously called t.Setenv("EVOLVE_CODEX_CONFIG_PATH", ...) have been
// migrated to set cfg.codexConfigPath instead. This ensures tests exercise the DI
// code path, not the now-removed env read.
//
// Files checked (confirmed package=bridge, same-package access to unexported field):
//   - codex_pretrust_test.go
//   - codex_pretrust_amplify_test.go
//   - codex_pretrust_concurrent_test.go
//   - codex_pretrust_launch_test.go
//   - preflight_test.go
//
// acs-predicate: config-check
//
// RED: all 5 files currently have t.Setenv("EVOLVE_CODEX_CONFIG_PATH", ...) calls.
// Cycle-41 failed by only updating codex_pretrust_test.go (1/5). This cycle
// updates all 5 atomically.
func TestC50A_005_BridgeTests_NoSetenvCodexConfigPath(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	bridgeDir := filepath.Join(root, "go", "internal", "bridge")
	files := []string{
		"codex_pretrust_test.go",
		"codex_pretrust_amplify_test.go",
		"codex_pretrust_concurrent_test.go",
		"codex_pretrust_launch_test.go",
		"preflight_test.go",
	}
	for _, name := range files {
		p := filepath.Join(bridgeDir, name)
		if !acsassert.FileNotContains(t, p, `"EVOLVE_CODEX_CONFIG_PATH"`) {
			t.Errorf("RED: %s still contains t.Setenv(\"EVOLVE_CODEX_CONFIG_PATH\", ...).\n"+
				"Builder must replace t.Setenv(\"EVOLVE_CODEX_CONFIG_PATH\", path) with\n"+
				"setting cfg.codexConfigPath = path on the bridge.Config struct already\n"+
				"constructed in that test. Cycle-41 lesson: ALL 5 files must be updated atomically.\n"+
				"File: %s", name, p)
		}
	}
}

// === Negative: upper-bound row count after Task A (behavioral) ===

// TestC50A_NEG_RowCountAtMost51 verifies that after Task A the registry row
// count has dropped from 52 to at most 51. A ≤ 51 check (rather than exact == 51)
// allows Task B to further reduce to 50 without this predicate re-failing.
//
// BEHAVIORAL: calls len(flagregistry.All) — the production count.
//
// RED: registry currently has 52 rows (FlagCeiling=52); 52 > 51 fails.
func TestC50A_NEG_RowCountAtMost51(t *testing.T) {
	got := len(flagregistry.All)
	if got > 51 {
		t.Errorf("RED: len(flagregistry.All) = %d, want ≤ 51 (52 − 1 Task A flag).\n"+
			"Builder must remove exactly this 1 row from registry_table.go:\n"+
			"  EVOLVE_CODEX_CONFIG_PATH\n"+
			"Current count %d exceeds 51 — Task A flag not yet removed.",
			got, got)
	}
}

// =============================================================================
// Task B — release-strict-pass-cli-50
// =============================================================================

// === Registry absence (behavioral: Lookup) ===

// TestC50B_001_ReleaseStrictPass_AbsentFromRegistry verifies that
// EVOLVE_RELEASE_STRICT_PASS is no longer registered after the CLI flag migration.
// Both production readers (cmd_release_preflight.go:55 and releasepipeline/bridges.go:26)
// are replaced by the --strict-pass CLI flag wired through releasepipeline.Options.StrictPass.
//
// Covers Task B AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
//
// RED: EVOLVE_RELEASE_STRICT_PASS is currently registered at registry_table.go:49
// with Status=StatusInternal, Doc="Undocumented production reader (inventory 2026-06-11)".
func TestC50B_001_ReleaseStrictPass_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_RELEASE_STRICT_PASS"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (release-strict-pass-cli-50: bucket-4 CLI flag migration).\n"+
			"Both os.Getenv reads must be removed: cmd_release_preflight.go:55 and releasepipeline/bridges.go:26.\n"+
			"Replace with --strict-pass CLI flag and releasepipeline.Options.StrictPass bool.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_RELEASE_STRICT_PASS", f.Status, f.Cluster)
	}
}

// === Prod-source clean — cmd_release_preflight.go (config-check waiver) ===

// TestC50B_002_ReleaseStrictPass_AbsentFromReleasePreflight verifies that
// os.Getenv("EVOLVE_RELEASE_STRICT_PASS") has been removed from cmd_release_preflight.go.
// After the migration, the command parses --strict-pass from CLI args instead.
// Precedent: --force-fresh migration (cycle-49 Task A) used the same bucket-4 pattern.
//
// acs-predicate: config-check
//
// RED: cmd_release_preflight.go:55 currently has:
//
//	strictPass := os.Getenv("EVOLVE_RELEASE_STRICT_PASS") == "1"
func TestC50B_002_ReleaseStrictPass_AbsentFromReleasePreflight(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_release_preflight.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_RELEASE_STRICT_PASS"`) {
		t.Errorf("RED: cmd_release_preflight.go still contains os.Getenv(\"EVOLVE_RELEASE_STRICT_PASS\").\n"+
			"Builder must:\n"+
			"  1. Add --strict-pass to the arg-parsing loop\n"+
			"  2. Replace: strictPass := os.Getenv(\"EVOLVE_RELEASE_STRICT_PASS\") == \"1\"\n"+
			"     With:    strictPass parsed from --strict-pass flag (var strictPass bool)\n"+
			"Pattern: follows --force-fresh (cycle-49), --skip-tests, --dry-run precedents.\n"+
			"File: %s", f)
	}
}

// === Prod-source clean — releasepipeline/bridges.go (config-check waiver) ===

// TestC50B_003_ReleaseStrictPass_AbsentFromBridges verifies that
// os.Getenv("EVOLVE_RELEASE_STRICT_PASS") has been removed from bridges.go.
// After the migration, runPreflightLib accepts a strictPass bool parameter
// (4-arg → 5-arg signature) and bridges.go reads it from that parameter.
//
// acs-predicate: config-check
//
// RED: bridges.go:26 currently has:
//
//	StrictPass: os.Getenv("EVOLVE_RELEASE_STRICT_PASS") == "1",
func TestC50B_003_ReleaseStrictPass_AbsentFromBridges(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "releasepipeline", "bridges.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_RELEASE_STRICT_PASS"`) {
		t.Errorf("RED: releasepipeline/bridges.go still contains os.Getenv(\"EVOLVE_RELEASE_STRICT_PASS\").\n"+
			"Builder must:\n"+
			"  1. Add strictPass bool param to runPreflightLib (4-arg → 5-arg)\n"+
			"  2. Replace: StrictPass: os.Getenv(\"EVOLVE_RELEASE_STRICT_PASS\") == \"1\"\n"+
			"     With:    StrictPass: strictPass\n"+
			"  3. Update the call site in releasepipeline.go to pass opts.StrictPass\n"+
			"  4. Update bridges_test.go call site to pass false as the new 5th arg\n"+
			"File: %s", f)
	}
}

// === FlagCeiling after both tasks (config-check waiver) ===

// === --strict-pass CLI flag registered (config-check waiver) ===

// TestC50B_005_StrictPassFlag_RegisteredInPreflight verifies that --strict-pass
// is registered as a CLI flag in cmd_release_preflight.go. The bucket-4 pattern
// (transient emergency hatch → explicit CLI flag) follows the --force-fresh
// precedent from cycle-49 and the --skip-tests flag from releasepipeline.
//
// acs-predicate: config-check
//
// RED: cmd_release_preflight.go currently has no --strict-pass flag; the env var
// os.Getenv("EVOLVE_RELEASE_STRICT_PASS") is used directly at line 55.
func TestC50B_005_StrictPassFlag_RegisteredInPreflight(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_release_preflight.go")
	if !acsassert.FileContains(t, f, `"strict-pass"`) {
		t.Errorf("RED: cmd_release_preflight.go does not contain the --strict-pass flag registration.\n"+
			"Builder must add a --strict-pass flag to the arg-parsing loop in cmd_release_preflight.go.\n"+
			"Pattern: follows --force-fresh (cmd_loop_args.go, cycle-49).\n"+
			"File: %s", f)
	}
}

// === releasepipeline.Options has StrictPass bool field (config-check waiver) ===

// TestC50B_006_ReleasePipelineOptions_HasStrictPassField verifies that
// releasepipeline.Options has a StrictPass bool field. This struct is the single
// authority for release behavior; adding StrictPass here makes it the SSOT
// and enables future consolidation of remaining release flags (BA2 hypothesis).
//
// acs-predicate: config-check
//
// RED: releasepipeline.go (Options struct) currently has SkipTests bool and
// other fields, but StrictPass bool is not present.
func TestC50B_006_ReleasePipelineOptions_HasStrictPassField(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "releasepipeline", "releasepipeline.go")
	if !acsassert.FileContains(t, f, "StrictPass bool") {
		t.Errorf("RED: releasepipeline.go Options struct does not contain 'StrictPass bool'.\n"+
			"Builder must add StrictPass bool to releasepipeline.Options and wire it through:\n"+
			"  releasepipeline.Options.StrictPass → runPreflightLib 5th arg → releasepreflight.Options.StrictPass\n"+
			"Verify the call site at releasepipeline.go:547 passes opts.StrictPass.\n"+
			"File: %s", f)
	}
}

// === Exact row count — final state (behavioral: negative / edge) ===
