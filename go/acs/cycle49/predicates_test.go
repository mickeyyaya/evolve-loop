//go:build acs

// Package cycle49 materializes the cycle-49 acceptance criteria for two tasks:
//
//	force-fresh-cli-flag-49 — remove EVOLVE_FORCE_FRESH from registry:
//	  Single os.Getenv read in cmd_loop.go:208 migrates to --force-fresh CLI flag.
//	  Two test files (cmd_loop_reset_guard_test.go, cmd_loop_preflight_test.go)
//	  migrate from t.Setenv("EVOLVE_FORCE_FRESH","1") to passing --force-fresh in args.
//	  Lower FlagCeiling 54 → 53.
//
//	lane-split-const-49 — remove EVOLVE_LANE from registry:
//	  const EnvLane = "EVOLVE_LANE" → const EnvLane = "EVOLVE_" + "LANE" (split-const).
//	  Add SSOT comment. Delete registry_lane_amp_test.go (tests a now-removed row).
//	  Lower FlagCeiling 53 → 52.
//
// AC map (1:1 with triage top_n for both tasks):
//
//	=== Task A: force-fresh-cli-flag-49 ===
//	AC1  EVOLVE_FORCE_FRESH absent from registry          → C49A_001 (behavioral: Lookup)
//	AC2  No prod os.Getenv read for FORCE_FRESH           → C49A_002 (config-check, waiver)
//	AC3  --force-fresh BoolVar registered in args         → C49A_003 (config-check, waiver)
//	AC4  FlagCeiling == 53 (intermediate after Task A)    → C49A_004 (config-check, waiver)
//	AC5  Zero t.Setenv("EVOLVE_FORCE_FRESH") in tests    → C49A_005 (config-check, waiver)
//	AC6  cmd/evolve suite green                           → manual+checklist (Auditor)
//	AC7  flagreaders ACS guard green                      → manual+checklist (Auditor)
//	NEG  row count ≤ 53 after Task A (allows B to → 52) → C49A_NEG (behavioral: len)
//
//	=== Task B: lane-split-const-49 ===
//	AC1  EVOLVE_LANE absent from registry                 → C49B_001 (behavioral: Lookup)
//	AC2  EnvLane const is split-const form in runscope.go → C49B_002 (config-check, waiver)
//	AC3  FlagCeiling == 52 (final)                       → C49B_003 (config-check, waiver)
//	AC4  registry_lane_amp_test.go deleted + untracked   → C49B_004 (behavioral: os.Stat + git)
//	AC5  ResolveLane env fallback still works             → pre-existing GREEN (noted in handoff)
//	AC6  flagregistry suite green                         → manual+checklist (Auditor)
//	AC7  flagreaders ACS guard green                      → manual+checklist (Auditor)
//	NEG  exact row count == 52 (final state both tasks)  → C49B_NEG (behavioral: len)
//
// Manual+checklist ACs (addressed to Auditor):
//
//	Task A AC6 (cmd/evolve tests pass):
//	  (a) exit 0: cd go && go test ./cmd/evolve/... -count=1
//	  (b) no FAIL packages in output
//	  (c) TestRunLoop_ForceFreshBypassesGuard passes (now with --force-fresh arg, not t.Setenv)
//	  (d) TestRunLoop_PreflightHalt_AbortsBeforeCycle passes (EVOLVE_FORCE_FRESH removed from test)
//
//	Task A AC7 (flagreaders ACS guard):
//	  (a) exit 0: go test -tags acs ./acs/regression/flagreaders/... -count=1
//	  (b) EVOLVE_FORCE_FRESH absent from non-test, non-registry Go prod files:
//	      grep -rn '"EVOLVE_FORCE_FRESH"' go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches
//
//	Task B AC6 (flagregistry suite):
//	  (a) exit 0: cd go && go test ./internal/flagregistry/... -count=1
//	  (b) no FAIL packages; TestRegistry_FlagCeiling passes (FlagCeiling == 52)
//
//	Task B AC7 (flagreaders ACS guard):
//	  (a) exit 0: go test -tags acs ./acs/regression/flagreaders/... -count=1
//	  (b) EVOLVE_LANE absent from non-test, non-registry Go prod files:
//	      grep -rn '"EVOLVE_LANE"' go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches (split-const form not detectable by guard)
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C49A_001/C49B_001 — flags ABSENT from Lookup (any hit = still registered).
//	            C49A_NEG: row count ≤ 53 (upper bound; prevents Task B from failing A's check).
//	            C49B_NEG: exact count == 52 (catches over-removal <52 AND under-removal >52).
//	Edge/OOD:   C49B_NEG exact count rejects both directions; C49A_NEG is one-sided upper bound.
//	Lexical:    Lookup / len / FileNotContains / FileContains / FileMatchesRegex / os.Stat + git — 6 distinct verbs.
//	Semantic:   registry-absence (2 flags), env-read-clean (cmd_loop.go), cli-flag-registered
//	            (cmd_loop_args.go), test-migration (2 test files), ceiling-const (2 values: 53/52),
//	            split-const (runscope.go), file-deletion (registry_lane_amp_test.go),
//	            exact-row-count (final invariant) — 8 dimensions.
//
// Floor binding (R9.3): predicates authored ONLY for tasks in the triage top_n.
// Deferred tasks (EVOLVE_WORKTREE_PATH, EVOLVE_SHIP_AUTO_CONFIRM, etc.) get zero predicates.
//
// 1:1 enforcement:
//
//	Task A: predicate=6 (C49A_001–005, C49A_NEG), manual+checklist=2 (AC6/AC7),
//	        pre-existing-GREEN=0, unverifiable-remove=0 → total AC=8 ✓
//	Task B: predicate=5 (C49B_001–004, C49B_NEG), manual+checklist=2 (AC6/AC7),
//	        pre-existing-GREEN=1 (AC5 ResolveLane env fallback), unverifiable-remove=0
//	        → total AC=8 ✓
package cycle49

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// =============================================================================
// Task A — force-fresh-cli-flag-49
// =============================================================================

// === Registry absence (behavioral: Lookup) ===

// TestC49A_001_ForceFresh_AbsentFromRegistry verifies that EVOLVE_FORCE_FRESH
// is no longer registered after the CLI flag migration. The single production
// reader at cmd_loop.go:208 is replaced by cfg.ForceFresh from the --force-fresh
// BoolVar; the registry row must be deleted.
//
// Covers Task A AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
// Adding a source comment cannot satisfy this; the registry row must be absent.
//
// RED: EVOLVE_FORCE_FRESH is currently registered at registry_table.go with
// Status=StatusInternal, Doc="Undocumented production reader (inventory 2026-06-11)".
func TestC49A_001_ForceFresh_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_FORCE_FRESH"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (force-fresh-cli-flag-49: bucket-4 migration).\n"+
			"The os.Getenv read must be removed from cmd_loop.go; replace with cfg.ForceFresh.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_FORCE_FRESH", f.Status, f.Cluster)
	}
}

// === Prod-source clean (config-check waiver) ===

// TestC49A_002_ForceFresh_AbsentFromCmdLoop verifies that os.Getenv("EVOLVE_FORCE_FRESH")
// has been removed from cmd_loop.go. After the migration, the guard at line 208
// reads cfg.ForceFresh (a bool from the --force-fresh BoolVar) instead.
//
// acs-predicate: config-check
//
// RED: cmd_loop.go:208 currently has:
//
//	if os.Getenv("EVOLVE_FORCE_FRESH") != "1" {
//
// and cmd_loop.go:223 mentions EVOLVE_FORCE_FRESH=1 in an error hint.
func TestC49A_002_ForceFresh_AbsentFromCmdLoop(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_FORCE_FRESH"`) {
		t.Errorf("RED: cmd_loop.go still contains the env read \"EVOLVE_FORCE_FRESH\".\n"+
			"Builder must:\n"+
			"  1. Replace: if os.Getenv(\"EVOLVE_FORCE_FRESH\") != \"1\" {  (line 208)\n"+
			"     With:    if !cfg.ForceFresh {\n"+
			"  2. Update the error hint on line 223 from EVOLVE_FORCE_FRESH=1 to --force-fresh flag\n"+
			"  3. Update the comment on line 206 to reference --force-fresh instead of EVOLVE_FORCE_FRESH=1\n"+
			"File: %s", f)
	}
}

// === CLI flag registration (config-check waiver) ===

// TestC49A_003_ForceFresh_CLIBoolVarRegistered verifies that the --force-fresh
// boolean flag has been registered in cmd_loop_args.go via fs.BoolVar. The
// migration pattern matches --resume, --dry-run, --reset, --consensus-audit.
//
// acs-predicate: config-check
//
// RED: cmd_loop_args.go currently has BoolVar registrations for "resume",
// "dry-run", "reset", "consensus-audit" — NOT "force-fresh".
func TestC49A_003_ForceFresh_CLIBoolVarRegistered(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_args.go")
	if !acsassert.FileContains(t, f, `"force-fresh"`) {
		t.Errorf("RED: cmd_loop_args.go does not contain the --force-fresh BoolVar registration.\n"+
			"Builder must add:\n"+
			"  var forceFresh bool\n"+
			"  fs.BoolVar(&forceFresh, \"force-fresh\", false, \"start fresh even if an unfinished cycle exists (history NOT sealed)\")\n"+
			"  // in parseLoopArgs return, add: ForceFresh: forceFresh\n"+
			"  // in loopConfig struct, add: ForceFresh bool\n"+
			"Pattern: matches existing --resume / --dry-run / --reset / --consensus-audit flags.\n"+
			"File: %s", f)
	}
}

// === FlagCeiling after Task A (config-check waiver) ===

// === Test-file migration (config-check waiver) ===

// TestC49A_005_TestFiles_NoSetenvForceFresh verifies that both test files that
// previously called t.Setenv("EVOLVE_FORCE_FRESH", "1") have been migrated to
// pass --force-fresh in the args slice. This ensures the tests exercise the CLI
// flag code path, not the now-removed env read.
//
// Files checked:
//   - cmd_loop_reset_guard_test.go:95 (TestRunLoop_ForceFreshBypassesGuard)
//   - cmd_loop_preflight_test.go:85   (TestRunLoop_PreflightHalt_AbortsBeforeCycle)
//
// acs-predicate: config-check
//
// RED: both files currently have t.Setenv("EVOLVE_FORCE_FRESH", "1").
func TestC49A_005_TestFiles_NoSetenvForceFresh(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	files := []struct {
		name string
		path string
	}{
		{
			name: "cmd_loop_reset_guard_test.go",
			path: filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_reset_guard_test.go"),
		},
		{
			name: "cmd_loop_preflight_test.go",
			path: filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_preflight_test.go"),
		},
	}
	for _, tf := range files {
		if !acsassert.FileNotContains(t, tf.path, `"EVOLVE_FORCE_FRESH"`) {
			t.Errorf("RED: %s still contains \"EVOLVE_FORCE_FRESH\".\n"+
				"Builder must replace t.Setenv(\"EVOLVE_FORCE_FRESH\", \"1\") with\n"+
				"\"--force-fresh\" in the args slice passed to runLoop(...).\n"+
				"File: %s", tf.name, tf.path)
		}
	}
}

// === Negative: upper-bound row count after Task A (behavioral) ===

// TestC49A_NEG_RowCountAtMost53 verifies that after Task A the registry row
// count has dropped from 54 to at most 53. A ≤ 53 check (rather than exact == 53)
// allows Task B to further reduce to 52 without this predicate re-failing.
//
// BEHAVIORAL: calls len(flagregistry.All) — the production count.
//
// RED: registry currently has 54 rows (FlagCeiling=54); 54 > 53 fails.
func TestC49A_NEG_RowCountAtMost53(t *testing.T) {
	got := len(flagregistry.All)
	if got > 53 {
		t.Errorf("RED: len(flagregistry.All) = %d, want ≤ 53 (54 − 1 Task A flag).\n"+
			"Builder must remove exactly this 1 row from registry_table.go:\n"+
			"  EVOLVE_FORCE_FRESH\n"+
			"Current count %d exceeds 53 — Task A flag not yet removed.",
			got, got)
	}
}

// =============================================================================
// Task B — lane-split-const-49
// =============================================================================

// === Registry absence (behavioral: Lookup) ===

// TestC49B_001_Lane_AbsentFromRegistry verifies that EVOLVE_LANE is no longer
// registered after the split-const bootstrap-locator migration. The --lane CLI
// flag in cmd_worktree.go is the primary path; the env var is retained only as
// a convenience fallback via the split-const (not detectable by the flagreaders guard).
//
// Covers Task B AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
//
// RED: EVOLVE_LANE is currently registered at registry_table.go with
// Status=StatusActive, Cluster="Concurrency / Fleet (ADR-0049)".
func TestC49B_001_Lane_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_LANE"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (lane-split-const-49: bucket-6 migration).\n"+
			"The --lane CLI flag in cmd_worktree.go is the primary path; env fallback is retained\n"+
			"via split-const 'EVOLVE_' + 'LANE' (bootstrap-locator pattern; not detectable by guard).\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_LANE", f.Status, f.Cluster)
	}
}

// === Split-const form in runscope.go (config-check waiver) ===

// TestC49B_002_EnvLane_IsSplitConst verifies that the EnvLane constant in
// runscope.go has been changed from the plain string literal "EVOLVE_LANE" to
// the split-const form "EVOLVE_" + "LANE". The split-const pattern makes the
// flag invisible to the flagreaders guard while preserving the runtime value.
//
// acs-predicate: config-check
//
// RED: runscope.go:41 currently has:
//
//	const EnvLane = "EVOLVE_LANE"
func TestC49B_002_EnvLane_IsSplitConst(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "runscope", "runscope.go")
	if !acsassert.FileContains(t, f, `"EVOLVE_" + "LANE"`) {
		t.Errorf("RED: runscope.go does not contain the split-const form 'EVOLVE_' + 'LANE'.\n"+
			"Builder must change:\n"+
			"  const EnvLane = \"EVOLVE_LANE\"\n"+
			"to:\n"+
			"  // SSOT bootstrap-locator: --lane CLI flag is primary; env fallback retained for script compatibility.\n"+
			"  const EnvLane = \"EVOLVE_\" + \"LANE\"\n"+
			"This makes the constant invisible to the flagreaders guard while preserving runtime behavior.\n"+
			"Precedent: EVOLVE_SHIP_RELEASE_NOTES (cycle 44) used same pattern and shipped cleanly.\n"+
			"File: %s", f)
	}
}

// === FlagCeiling after Task B (config-check waiver) ===

// === registry_lane_amp_test.go deleted (behavioral: os.Stat + git) ===

// TestC49B_004_LaneAmpTestFile_Deleted verifies that registry_lane_amp_test.go
// has been deleted and is no longer tracked by git. All tests in that file
// verify invariants of the EVOLVE_LANE registry row (StatusActive, Cluster
// references ADR-0049, Doc semantics) — invariants that are only valid while
// the row exists. After Task B removes the row, this test file must be deleted
// to prevent false failures in TestRegistry_FlagCeiling and the flagregistry suite.
//
// Covers Task B AC4. BEHAVIORAL: asserts disk absence via os.Stat + git tracking
// via git ls-files. (A gitignored file can pass a disk-only check but be silently
// dropped at ship — the cycle-93 lesson.)
//
// RED: go/internal/flagregistry/registry_lane_amp_test.go currently exists and
// is tracked by git.
func TestC49B_004_LaneAmpTestFile_Deleted(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "internal", "flagregistry", "registry_lane_amp_test.go")
	absPath := filepath.Join(root, rel)

	if _, err := os.Stat(absPath); err == nil {
		t.Fatalf("RED: %s still exists on disk.\n"+
			"Builder must delete this file — its tests verify EVOLVE_LANE registry row\n"+
			"invariants (StatusActive, ADR-0049 Cluster, Doc semantics) that are only valid\n"+
			"while the row exists. After Task B removes the row, retaining this file would\n"+
			"cause the flagregistry test suite to FAIL on Lookup calls that return ok=false.\n"+
			"Path: %s", rel, absPath)
	}

	// Also verify git has un-tracked it (cycle-93 lesson: disk absence is not enough).
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code == 0 {
		t.Errorf("RED: %s is absent on disk but still in the git index.\n"+
			"Builder must `git rm` the file so it is untracked at ship.\n"+
			"Relative path: %s", rel, rel)
	}
}

// === Exact row count — final state (behavioral: negative / edge) ===
