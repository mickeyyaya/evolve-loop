//go:build acs

// Package cycle52 materializes the cycle-52 acceptance criteria for two tasks:
//
//	skip-preflight-cli-52 — remove EVOLVE_SKIP_PREFLIGHT and EVOLVE_SKIP_PREFLIGHT_BOOT
//	  from the flag registry; replace both os.Getenv reads in cmd_loop_preflight.go
//	  with cfg.SkipPreflight / cfg.SkipPreflightBoot fields on loopConfig; add
//	  --skip-preflight / --skip-preflight-boot CLI flags in cmd_loop_args.go;
//	  migrate tests from t.Setenv/os.Setenv to explicit CLI args;
//	  lower FlagCeiling 50→48.
//
//	ship-auto-confirm-split-const-52 — remove EVOLVE_SHIP_AUTO_CONFIRM from registry;
//	  define split-const `const envShipAutoConfirm = "EVOLVE_" + "SHIP_AUTO_CONFIRM"`
//	  with SSOT IPC-protocol comment in verify.go; replace two literal occurrences;
//	  lower FlagCeiling 48→47.
//
// AC map (1:1 with triage top_n for both tasks):
//
//	=== Task A: skip-preflight-cli-52 ===
//	AC1  EVOLVE_SKIP_PREFLIGHT absent from registry           → C52A_001 (behavioral: Lookup)
//	AC2  EVOLVE_SKIP_PREFLIGHT_BOOT absent from registry      → C52A_002 (behavioral: Lookup)
//	AC3  No prod os.Getenv(SKIP_PREFLIGHT) in preflight.go   → C52A_003 (config-check, waiver)
//	AC4  No prod os.Getenv(SKIP_PREFLIGHT_BOOT) in preflight → C52A_004 (config-check, waiver)
//	AC5  loopConfig has SkipPreflight bool field              → C52A_005 (config-check, waiver, FileMatchesRegex)
//	AC6  loopConfig has SkipPreflightBoot bool field          → C52A_006 (config-check, waiver, FileMatchesRegex)
//	AC7  FlagCeiling == 48 (intermediate after Task A)        → C52A_007 (config-check, waiver)
//	AC8  No t.Setenv/os.Setenv for either flag in cmd/evolve  → C52A_008 (config-check, waiver)
//	     tests (manual+checklist: cmd/evolve suite green)     → manual+checklist (Auditor)
//	     flagreaders ACS guard green                          → manual+checklist (Auditor)
//	NEG  row count ≤ 48 after Task A (allows B to reach 47)  → C52A_NEG (behavioral: len)
//
//	=== Task B: ship-auto-confirm-split-const-52 ===
//	AC1  EVOLVE_SHIP_AUTO_CONFIRM absent from registry        → C52B_001 (behavioral: Lookup)
//	AC2  split-const envShipAutoConfirm in verify.go          → C52B_002 (config-check, waiver)
//	AC3  no bare literal in verify.go prod reader             → C52B_003 (config-check, waiver)
//	AC4  FlagCeiling == 47 (final)                            → C52B_004 (config-check, waiver)
//	NEG  exact row count == 47                                → C52B_NEG (behavioral: len)
//
// Manual+checklist ACs (addressed to Auditor):
//
//	Task A (cmd/evolve test suite green):
//	  (a) exit 0: cd go && go test ./cmd/evolve/... -count=1
//	  (b) no FAIL packages in output
//	  (c) runLoop calls that relied on TestMain global os.Setenv pass --skip-preflight explicitly
//	  (d) cmd_loop_preflight_test.go tests use cfg.SkipPreflight=true not env override
//
//	Task A (flagreaders ACS guard green):
//	  (a) exit 0: go test -tags acs ./acs/regression/flagreaders/... -count=1
//	  (b) EVOLVE_SKIP_PREFLIGHT absent from non-test, non-registry Go prod files:
//	      grep -rn '"EVOLVE_SKIP_PREFLIGHT"' go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches
//	  (c) EVOLVE_SKIP_PREFLIGHT_BOOT absent from same set → 0 matches
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C52A_001/002/C52B_001 — flags ABSENT from Lookup (any hit = still registered).
//	            C52A_NEG: row count ≤ 48 (upper bound; prevents Task B from failing A's check).
//	            C52B_NEG: exact count == 47 (catches over-removal <47 AND under-removal >47).
//	Edge/OOD:   C52B_NEG exact count rejects both directions.
//	            C52A_005/006 FileMatchesRegex with \s+ tolerates gofmt column-alignment (cycle-51 lesson).
//	Lexical:    Lookup / len / FileNotContains / FileMatchesRegex / FileContains — 5 distinct verbs.
//	Semantic:   registry-absence (3 flags), env-read-clean (1 file × 2 vars), struct-fields (2),
//	            test-setenv-clean (3 test files), ceiling-const (2 values: 48/47),
//	            split-const-present (1), bare-literal-absent (1), exact-row-count (1) — 9 dimensions.
//
// Floor binding (R9.3): predicates authored ONLY for tasks in the triage top_n.
// Deferred tasks (EVOLVE_WORKTREE_PATH, EVOLVE_STRICT_AUDIT, etc.) get zero predicates.
//
// Cycle-51 lesson applied: struct-field assertions use FileMatchesRegex with \s+ (not
// FileContains with exact single space). gofmt column-aligns struct fields with multiple
// spaces when a longer sibling field is present; FileContains("SkipPreflight bool") fails
// when gofmt produces "SkipPreflight     bool". FileMatchesRegex with `SkipPreflight\s+bool`
// tolerates any whitespace count.
//
// 1:1 enforcement:
//
//	Task A: predicate=7 (C52A_001–008, C52A_NEG), manual+checklist=2 (suite/flagreaders),
//	        pre-existing-GREEN=0, unverifiable-remove=0 → total AC=9 ✓
//	Task B: predicate=4 (C52B_001–004, C52B_NEG), manual+checklist=0,
//	        pre-existing-GREEN=0, unverifiable-remove=0 → total AC=5 ✓
package cycle52

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// =============================================================================
// Task A — skip-preflight-cli-52
// =============================================================================

// === Registry absence (behavioral: Lookup) ===

// TestC52A_001_SkipPreflight_AbsentFromRegistry verifies that EVOLVE_SKIP_PREFLIGHT
// is no longer registered after the CLI flag migration. The sole production reader
// at cmd_loop_preflight.go:50 is replaced by cfg.SkipPreflight on loopConfig.
//
// Covers Task A AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
// Adding a source comment cannot satisfy this; the registry row must be absent.
//
// RED: EVOLVE_SKIP_PREFLIGHT is currently registered at registry_table.go:52
// with Status=StatusActive, Cluster="Readiness Gate (pre-batch)".
func TestC52A_001_SkipPreflight_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_SKIP_PREFLIGHT"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (skip-preflight-cli-52: bucket-4 CLI flag migration).\n"+
			"The os.Getenv read must be removed from cmd_loop_preflight.go:50; replace with cfg.SkipPreflight.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_SKIP_PREFLIGHT", f.Status, f.Cluster)
	}
}

// TestC52A_002_SkipPreflightBoot_AbsentFromRegistry verifies that EVOLVE_SKIP_PREFLIGHT_BOOT
// is no longer registered after the CLI flag migration. The sole production reader
// at cmd_loop_preflight.go:27 is replaced by cfg.SkipPreflightBoot on loopConfig.
//
// Covers Task A AC2. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
//
// RED: EVOLVE_SKIP_PREFLIGHT_BOOT is currently registered at registry_table.go:53
// with Status=StatusActive, Cluster="Readiness Gate (pre-batch)".
func TestC52A_002_SkipPreflightBoot_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_SKIP_PREFLIGHT_BOOT"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (skip-preflight-cli-52: bucket-4 CLI flag migration).\n"+
			"The os.Getenv read must be removed from cmd_loop_preflight.go:27; replace with cfg.SkipPreflightBoot.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_SKIP_PREFLIGHT_BOOT", f.Status, f.Cluster)
	}
}

// === Prod-source clean (config-check waiver) ===

// TestC52A_003_SkipPreflight_AbsentFromPreflightGo verifies that
// os.Getenv("EVOLVE_SKIP_PREFLIGHT") has been removed from cmd_loop_preflight.go.
// After the CLI flag migration the function reads cfg.SkipPreflight instead.
//
// acs-predicate: config-check
//
// RED: cmd_loop_preflight.go:50 currently contains:
//
//	if os.Getenv("EVOLVE_SKIP_PREFLIGHT") == "1" {
func TestC52A_003_SkipPreflight_AbsentFromPreflightGo(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_preflight.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_SKIP_PREFLIGHT"`) {
		t.Errorf("RED: cmd_loop_preflight.go still contains the env read \"EVOLVE_SKIP_PREFLIGHT\".\n"+
			"Builder must:\n"+
			"  1. Add SkipPreflight bool to loopConfig in cmd_loop.go\n"+
			"  2. Add --skip-preflight flag var in cmd_loop_args.go (next to --force-fresh)\n"+
			"  3. Replace os.Getenv(\"EVOLVE_SKIP_PREFLIGHT\") == \"1\" with cfg.SkipPreflight\n"+
			"  4. Update the log message at :51 to say \"(--skip-preflight)\" instead of the env var name\n"+
			"Precedent: --force-fresh migration (cycle-49 EVOLVE_FORCE_FRESH → cmd_loop_args.go:54).\n"+
			"File: %s", f)
	}
}

// TestC52A_004_SkipPreflightBoot_AbsentFromPreflightGo verifies that
// os.Getenv("EVOLVE_SKIP_PREFLIGHT_BOOT") has been removed from cmd_loop_preflight.go.
// After the migration the function reads cfg.SkipPreflightBoot instead.
//
// acs-predicate: config-check
//
// RED: cmd_loop_preflight.go:27 currently contains:
//
//	SkipBoot: os.Getenv("EVOLVE_SKIP_PREFLIGHT_BOOT") == "1",
func TestC52A_004_SkipPreflightBoot_AbsentFromPreflightGo(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_preflight.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_SKIP_PREFLIGHT_BOOT"`) {
		t.Errorf("RED: cmd_loop_preflight.go still contains the env read \"EVOLVE_SKIP_PREFLIGHT_BOOT\".\n"+
			"Builder must:\n"+
			"  1. Add SkipPreflightBoot bool to loopConfig in cmd_loop.go\n"+
			"  2. Add --skip-preflight-boot flag var in cmd_loop_args.go (next to --skip-preflight)\n"+
			"  3. Replace os.Getenv(\"EVOLVE_SKIP_PREFLIGHT_BOOT\") == \"1\" with cfg.SkipPreflightBoot\n"+
			"File: %s", f)
	}
}

// === loopConfig struct fields (config-check waiver — FileMatchesRegex, cycle-51 lesson) ===

// TestC52A_005_LoopConfig_HasSkipPreflightField verifies that loopConfig (defined in
// cmd_loop.go) has a SkipPreflight bool field. This is the DI seam replacing the
// os.Getenv read.
//
// IMPORTANT (cycle-51 lesson): uses FileMatchesRegex with `SkipPreflight\s+bool` —
// NOT FileContains with a single-space string. When SkipPreflightBoot is a sibling
// field with a longer name, gofmt column-aligns "SkipPreflight" with 5+ spaces before
// "bool". FileContains("SkipPreflight bool") would fail on gofmt-formatted output.
//
// acs-predicate: config-check
//
// RED: cmd_loop.go currently has ForceFresh bool as the last bool field;
// SkipPreflight bool is not present.
func TestC52A_005_LoopConfig_HasSkipPreflightField(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go")
	if !acsassert.FileMatchesRegex(t, f, `SkipPreflight\s+bool`) {
		t.Errorf("RED: cmd_loop.go does not contain 'SkipPreflight <whitespace> bool' on loopConfig.\n"+
			"Builder must add an unexported bool field to loopConfig (cmd_loop.go):\n"+
			"  SkipPreflight bool  // --skip-preflight: bypass the whole readiness gate\n"+
			"Cycle-51 lesson: DO NOT use FileContains with exact single space — gofmt\n"+
			"column-aligns struct fields when SkipPreflightBoot (longer) is a sibling.\n"+
			"Precedent: ForceFresh bool field (cmd_loop.go:74, --force-fresh migration cycle-49).\n"+
			"File: %s", f)
	}
}

// TestC52A_006_LoopConfig_HasSkipPreflightBootField verifies that loopConfig has a
// SkipPreflightBoot bool field — the DI seam replacing the EVOLVE_SKIP_PREFLIGHT_BOOT
// os.Getenv read at cmd_loop_preflight.go:27.
//
// IMPORTANT (cycle-51 lesson): uses FileMatchesRegex with `SkipPreflightBoot\s+bool`.
// SkipPreflightBoot is the longer sibling that CAUSES gofmt to pad SkipPreflight.
// The regex handles any column-alignment gofmt produces.
//
// acs-predicate: config-check
//
// RED: cmd_loop.go does not have a SkipPreflightBoot bool field.
func TestC52A_006_LoopConfig_HasSkipPreflightBootField(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go")
	if !acsassert.FileMatchesRegex(t, f, `SkipPreflightBoot\s+bool`) {
		t.Errorf("RED: cmd_loop.go does not contain 'SkipPreflightBoot <whitespace> bool' on loopConfig.\n"+
			"Builder must add:\n"+
			"  SkipPreflightBoot bool  // --skip-preflight-boot: run cheap checks but skip real bridge-boot\n"+
			"alongside the SkipPreflight bool field (Task A AC6).\n"+
			"File: %s", f)
	}
}

// === FlagCeiling after Task A (config-check waiver) ===

// === Test-file migration — zero Setenv for either flag in cmd/evolve (config-check waiver) ===

// TestC52A_008_CmdEvolveTests_NoSetenvSkipPreflight verifies that all cmd/evolve test
// files that previously called t.Setenv/os.Setenv("EVOLVE_SKIP_PREFLIGHT"...) have been
// migrated to pass --skip-preflight as a CLI arg instead.
//
// Files checked:
//   - cmd_loop_preflight_test.go (t.Setenv at :34, :56, :84)
//   - cmd_loop_failbreaker_test.go (t.Setenv at :159)
//   - main_test.go (os.Setenv at :30 in TestMain)
//
// Note: cmd_loop_preflight_test.go re-enables the real preflight per-test; after migration
// those tests drive the behavior via cfg.SkipPreflight/SkipPreflightBoot on loopConfig
// or via --skip-preflight args passed to runLoop(). main_test.go TestMain drops the
// global os.Setenv; all runLoop() calls that relied on it receive "--skip-preflight" arg.
//
// acs-predicate: config-check
//
// RED: three files currently contain t.Setenv/os.Setenv("EVOLVE_SKIP_PREFLIGHT"...).
func TestC52A_008_CmdEvolveTests_NoSetenvSkipPreflight(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	cmdDir := filepath.Join(root, "go", "cmd", "evolve")
	files := []string{
		"cmd_loop_preflight_test.go",
		"cmd_loop_failbreaker_test.go",
		"main_test.go",
	}
	for _, name := range files {
		p := filepath.Join(cmdDir, name)
		if !acsassert.FileNotContains(t, p, `"EVOLVE_SKIP_PREFLIGHT"`) {
			t.Errorf("RED: %s still contains t.Setenv/os.Setenv(\"EVOLVE_SKIP_PREFLIGHT\"...) call.\n"+
				"Builder must migrate env-based test setup to CLI arg injection:\n"+
				"  - In main_test.go TestMain: remove os.Setenv(\"EVOLVE_SKIP_PREFLIGHT\", \"1\")\n"+
				"    and add \"--skip-preflight\" to all runLoop() helper calls that relied on it.\n"+
				"  - In cmd_loop_preflight_test.go: replace t.Setenv(\"EVOLVE_SKIP_PREFLIGHT\",\"1\")\n"+
				"    with cfg.SkipPreflight=true on the loopConfig, or pass --skip-preflight arg.\n"+
				"  - In cmd_loop_failbreaker_test.go: same pattern.\n"+
				"File: %s", name, p)
		}
	}
}

// === Negative: upper-bound row count after Task A (behavioral) ===

// TestC52A_NEG_RowCountAtMost48 verifies that after Task A the registry row count
// has dropped from 50 to at most 48. A ≤ 48 check (rather than exact == 48) allows
// Task B to further reduce to 47 without this predicate re-failing.
//
// BEHAVIORAL: calls len(flagregistry.All) — the production count.
//
// RED: registry currently has 50 rows (FlagCeiling=50); 50 > 48 fails.
func TestC52A_NEG_RowCountAtMost48(t *testing.T) {
	got := len(flagregistry.All)
	if got > 48 {
		t.Errorf("RED: len(flagregistry.All) = %d, want ≤ 48 (50 − 2 Task A flags).\n"+
			"Builder must remove exactly these 2 rows from registry_table.go:\n"+
			"  EVOLVE_SKIP_PREFLIGHT\n"+
			"  EVOLVE_SKIP_PREFLIGHT_BOOT\n"+
			"Current count %d exceeds 48 — Task A flags not yet removed.",
			got, got)
	}
}

// =============================================================================
// Task B — ship-auto-confirm-split-const-52
// =============================================================================

// === Registry absence (behavioral: Lookup) ===

// TestC52B_001_ShipAutoConfirm_AbsentFromRegistry verifies that EVOLVE_SHIP_AUTO_CONFIRM
// is no longer registered after the split-const migration. The sole production reader
// at verify.go:239 is replaced by the split-const envShipAutoConfirm.
//
// Covers Task B AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
//
// RED: EVOLVE_SHIP_AUTO_CONFIRM is currently registered at registry_table.go:51
// with Status=StatusActive, Cluster="Workflow Defaults".
func TestC52B_001_ShipAutoConfirm_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_SHIP_AUTO_CONFIRM"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (ship-auto-confirm-split-const-52: bucket-5 split-const).\n"+
			"The literal string must be replaced with the split-const envShipAutoConfirm in verify.go.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_SHIP_AUTO_CONFIRM", f.Status, f.Cluster)
	}
}

// === Split-const present in verify.go (config-check waiver) ===

// TestC52B_002_VerifyGo_HasSplitConst verifies that verify.go defines the split-const
// `const envShipAutoConfirm = "EVOLVE_" + "SHIP_AUTO_CONFIRM"` with the IPC-protocol
// comment. The split-const pattern (bucket-5) makes the flag invisible to the
// flagreaders guard while preserving the IPC handoff contract between
// releasepipeline/rollback (setters) and ship/verify.go (reader).
//
// Precedent: cycle-49 EVOLVE_LANE split-const (identical pattern).
//
// acs-predicate: config-check
//
// RED: verify.go currently has the bare literal "EVOLVE_SHIP_AUTO_CONFIRM" at :239.
// No split-const is defined.
func TestC52B_002_VerifyGo_HasSplitConst(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "phases", "ship", "verify.go")
	if !acsassert.FileMatchesRegex(t, f, `envShipAutoConfirm\s*=\s*"EVOLVE_"\s*\+\s*"SHIP_AUTO_CONFIRM"`) {
		t.Errorf("RED: verify.go does not contain the split-const definition envShipAutoConfirm.\n"+
			"Builder must add at the top of verify.go (after package+imports):\n"+
			"  // SSOT IPC-protocol-allowed: releasepipeline/rollback→ship subprocess\n"+
			"  const envShipAutoConfirm = \"EVOLVE_\" + \"SHIP_AUTO_CONFIRM\"\n"+
			"And replace the two literal occurrences at :239 and :270 with envShipAutoConfirm.\n"+
			"Precedent: cycle-49 EVOLVE_LANE split-const (SSOT IPC comment required).\n"+
			"File: %s", f)
	}
}

// === No bare literal in verify.go (config-check waiver) ===

// TestC52B_003_VerifyGo_NoBareEnvLiteral verifies that the bare string literal
// "EVOLVE_SHIP_AUTO_CONFIRM" no longer appears as a direct string in verify.go.
// After the split-const migration, only the split-const definition should reference
// the flag name indirectly (split across two string literals).
//
// Note: this check uses FileNotContains on the fully-assembled string, which will
// NOT match the split-const `"EVOLVE_" + "SHIP_AUTO_CONFIRM"` — they are distinct
// string tokens. This distinguishes "still using the literal" from "correctly using split-const".
//
// acs-predicate: config-check
//
// RED: verify.go:239 currently has opts.envBool("EVOLVE_SHIP_AUTO_CONFIRM") and
// :270 has Set EVOLVE_SHIP_AUTO_CONFIRM=1 in the error message string.
func TestC52B_003_VerifyGo_NoBareEnvLiteral(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "phases", "ship", "verify.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_SHIP_AUTO_CONFIRM"`) {
		t.Errorf("RED: verify.go still contains the bare literal \"EVOLVE_SHIP_AUTO_CONFIRM\".\n"+
			"Builder must replace ALL occurrences with the split-const envShipAutoConfirm:\n"+
			"  :239  opts.envBool(\"EVOLVE_SHIP_AUTO_CONFIRM\") → opts.envBool(envShipAutoConfirm)\n"+
			"  :270  error message Set EVOLVE_SHIP_AUTO_CONFIRM=1 → use envShipAutoConfirm in fmt.Sprintf\n"+
			"File: %s", f)
	}
}

// === FlagCeiling after both tasks (config-check waiver) ===

// === Exact row count — final state (behavioral: negative / edge) ===
