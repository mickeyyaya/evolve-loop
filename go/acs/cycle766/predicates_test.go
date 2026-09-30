//go:build acs

package cycle766

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test reported no PASS for %s (renamed or not run?)\nstdout:\n%s", name, stdout)
	}
}

func TestC766_001_lane_scope_file_injects_fleet_scope(t *testing.T) {
	runGoTest(t, corePkg, "TestLaneScopePin_FileInjectsFleetScopeToPhases")
}

func TestC766_002_two_lanes_see_only_own_scope(t *testing.T) {
	runGoTest(t, corePkg, "TestLaneScopePin_TwoLanesSeeOnlyOwnScope")
}

func TestC766_003_absent_file_falls_back_to_env(t *testing.T) {
	runGoTest(t, corePkg, "TestLaneScopePin_AbsentFileFallsBackToEnv")
}

func TestC766_004_lane_scope_materialized_from_env(t *testing.T) {
	runGoTest(t, corePkg, "TestLaneScopePin_MaterializedFromEnvBeforePhases")
}

func TestC766_005_goal_hash_mismatch_aborts_before_triage(t *testing.T) {
	runGoTest(t, corePkg, "TestLaneScopePin_ScoutGoalHashMismatchAbortsBeforeTriage")
}

func TestC766_006_goal_hash_match_proceeds(t *testing.T) {
	runGoTest(t, corePkg, "TestLaneScopePin_ScoutGoalHashMatchProceeds")
}

func TestC766_007_goal_hash_absent_fails_open(t *testing.T) {
	runGoTest(t, corePkg, "TestLaneScopePin_ScoutReportWithoutGoalHashProceeds")
}
