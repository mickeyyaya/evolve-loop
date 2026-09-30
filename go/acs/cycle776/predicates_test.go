//go:build acs

package cycle776

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	scoutPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/scout"
	buildPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/build"
	tddPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/phases/tdd"
	corePkg  = "github.com/mickeyyaya/evolve-loop/go/internal/core"
)

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

func TestC776_001_scout_prompt_renders_lane_scope(t *testing.T) {
	runGoTest(t, scoutPkg, "TestScout_ComposePrompt_RendersFleetScope")
}

func TestC776_002_scout_prompt_two_lanes_only_own_scope(t *testing.T) {
	runGoTest(t, scoutPkg, "TestScout_ComposePrompt_TwoLanesSeeOnlyOwnScope")
}

func TestC776_003_scout_prompt_scope_from_typed_envelope(t *testing.T) {
	runGoTest(t, scoutPkg, "TestScout_ComposePrompt_FleetScopeFromTypedEnvelope")
}

func TestC776_004_scout_prompt_unscoped_no_scope_line(t *testing.T) {
	runGoTest(t, scoutPkg, "TestScout_ComposePrompt_NoFleetScope_NoScopeLine")
}

func TestC776_005_build_prompt_renders_lane_scope(t *testing.T) {
	runGoTest(t, buildPkg, "TestBuild_ComposePrompt_RendersFleetScope")
}

func TestC776_006_build_prompt_unscoped_no_scope_line(t *testing.T) {
	runGoTest(t, buildPkg, "TestBuild_ComposePrompt_NoFleetScope_NoScopeLine")
}

func TestC776_007_tdd_prompt_renders_lane_scope(t *testing.T) {
	runGoTest(t, tddPkg, "TestTDD_ComposePrompt_RendersFleetScope")
}

func TestC776_008_tdd_prompt_unscoped_no_scope_line(t *testing.T) {
	runGoTest(t, tddPkg, "TestTDD_ComposePrompt_NoFleetScope_NoScopeLine")
}

func TestC776_009_goal_hash_mismatch_gate_regression(t *testing.T) {
	runGoTest(t, corePkg, "TestLaneScopePin_ScoutGoalHashMismatchAbortsBeforeTriage")
}

func TestC776_010_scout_prompt_instructs_goal_hash_echo(t *testing.T) {
	runGoTest(t, scoutPkg, "TestScout_ComposePrompt_LaneScoped_InstructsGoalHashEchoInDecisionTrace")
}
