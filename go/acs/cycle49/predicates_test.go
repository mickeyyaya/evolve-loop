//go:build acs

package cycle49

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC49A_001_ForceFresh_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_FORCE_FRESH"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (force-fresh-cli-flag-49: bucket-4 migration).\n"+
			"The os.Getenv read must be removed from cmd_loop.go; replace with cfg.ForceFresh.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_FORCE_FRESH", f.Status, f.Cluster)
	}
}

// acs-predicate: config-check
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

// acs-predicate: config-check
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

// acs-predicate: config-check
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

// acs-predicate: config-check
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

	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code == 0 {
		t.Errorf("RED: %s is absent on disk but still in the git index.\n"+
			"Builder must `git rm` the file so it is untracked at ship.\n"+
			"Relative path: %s", rel, rel)
	}
}
