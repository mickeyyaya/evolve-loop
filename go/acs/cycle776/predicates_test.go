//go:build acs

// Package cycle776 materializes the cycle-776 acceptance criteria for the sole
// committed top_n task fleet-lane-provisioning-split (triage-report.md
// ## top_n; scout's three unrelated proposals were DEFERRED by triage as
// out-of-lane-scope, so per R9.3 no predicates bind to them).
//
// Task source: inbox id fleet-lane-provisioning-split (weight 0.9, cycle-640
// incident). Cycle-766 landed the pin (lane-scope.json → Context["fleet_scope"]
// for every phase) and the scout→triage goal-hash coherence gate. The residual
// slice — proven live by THIS run, whose scout prompt carried no lane scope and
// whose scout consequently scouted three out-of-scope tasks — is the PROMPT
// layer: only triage renders the scope into what the LLM actually reads.
//
// AC map (1:1), from the inbox item's acceptance[] list:
//
//	AC1 concurrent lanes each see ONLY their own scope in scout/triage/build
//	    prompts (fixture asserts injected scope matches lane-scope.json)
//	    → C776_001 (scout renders), C776_002 (scout two-lane isolation,
//	      negative), C776_003 (scout typed-envelope source),
//	      C776_004 (scout unscoped edge — no over-render),
//	      C776_005 (build renders + foreign-id negative),
//	      C776_006 (build unscoped edge),
//	      C776_007 (tdd renders + foreign-id negative),
//	      C776_008 (tdd unscoped edge).
//	    Triage rendering is pre-existing GREEN (triage.go +
//	    triage_phaseio_test.go); Context-level injection from lane-scope.json
//	    is pre-existing GREEN (cycle-766, core/lanescope_pin_test.go).
//	AC2 scout-report goal-hash mismatch vs lane-scope.json fails the
//	    scout→triage transition with abort_reason (no silent proceed)
//	    → gate itself pre-existing GREEN, re-bound as regression by
//	      C776_009; the NEW teeth are C776_010 (lane-scoped scout prompt
//	      must instruct the Decision Trace goal_hash echo — without it the
//	      gate fails open forever, exactly what happened this run).
//	AC3 go test -race PASS on touched packages; apicover clean → every
//	    predicate runs its unit contract under -race (apicover runs in the
//	    repo-wide gate).
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit contract, which EXERCISES ComposePrompt / RunCycle behaviorally — no
// source-grep predicates (cycle-85 rule). The `-v` + "--- PASS:" guard rejects
// a rename/no-tests-matched silent green. Adversarial axes: negative
// (foreign lane ids must NOT appear; mismatch must NOT proceed), edge
// (unscoped prompt must NOT over-render), semantic (render vs isolation vs
// gate-teeth are separate behaviors).
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

// runGoTest executes the named unit test under -race and requires an explicit
// verbose PASS marker so the predicate fails on: compile failure, test
// failure, a race report, a missing package, OR the test not existing
// (rename gaming).
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

// AC1: scout's composed prompt carries a fleet_scope directive naming every
// assigned todo id.
func TestC776_001_scout_prompt_renders_lane_scope(t *testing.T) {
	runGoTest(t, scoutPkg, "TestScout_ComposePrompt_RendersFleetScope")
}

// AC1 negative: two lanes each render ONLY their own ids in the scout prompt —
// the other lane's id must be absent (cycle-640 cross-lane drift).
func TestC776_002_scout_prompt_two_lanes_only_own_scope(t *testing.T) {
	runGoTest(t, scoutPkg, "TestScout_ComposePrompt_TwoLanesSeeOnlyOwnScope")
}

// AC1 typed source: at phaseio enforce the scope renders from the typed
// envelope with no Context map (mirror of triage's dual read).
func TestC776_003_scout_prompt_scope_from_typed_envelope(t *testing.T) {
	runGoTest(t, scoutPkg, "TestScout_ComposePrompt_FleetScopeFromTypedEnvelope")
}

// AC1 edge: an unscoped (sequential) scout prompt carries NO fleet_scope line.
func TestC776_004_scout_prompt_unscoped_no_scope_line(t *testing.T) {
	runGoTest(t, scoutPkg, "TestScout_ComposePrompt_NoFleetScope_NoScopeLine")
}

// AC1: build's composed prompt renders the lane scope; foreign ids absent.
func TestC776_005_build_prompt_renders_lane_scope(t *testing.T) {
	runGoTest(t, buildPkg, "TestBuild_ComposePrompt_RendersFleetScope")
}

// AC1 edge: an unscoped build prompt carries NO fleet_scope line.
func TestC776_006_build_prompt_unscoped_no_scope_line(t *testing.T) {
	runGoTest(t, buildPkg, "TestBuild_ComposePrompt_NoFleetScope_NoScopeLine")
}

// AC1: tdd's composed prompt renders the lane scope; foreign ids absent.
func TestC776_007_tdd_prompt_renders_lane_scope(t *testing.T) {
	runGoTest(t, tddPkg, "TestTDD_ComposePrompt_RendersFleetScope")
}

// AC1 edge: an unscoped tdd prompt carries NO fleet_scope line.
func TestC776_008_tdd_prompt_unscoped_no_scope_line(t *testing.T) {
	runGoTest(t, tddPkg, "TestTDD_ComposePrompt_NoFleetScope_NoScopeLine")
}

// AC2 regression re-bind (pre-existing GREEN, cycle-766): scout-report
// goal_hash ≠ lane-scope.json goal_hash aborts before triage with an explicit
// mismatch reason — no silent proceed.
func TestC776_009_goal_hash_mismatch_gate_regression(t *testing.T) {
	runGoTest(t, corePkg, "TestLaneScopePin_ScoutGoalHashMismatchAbortsBeforeTriage")
}

// AC2 teeth: a lane-scoped scout prompt instructs the Decision Trace to echo
// the pinned goal_hash — without the echo the coherence gate never fires
// (this very run's scout omitted goal_hash and the gate failed open).
func TestC776_010_scout_prompt_instructs_goal_hash_echo(t *testing.T) {
	runGoTest(t, scoutPkg, "TestScout_ComposePrompt_LaneScoped_InstructsGoalHashEchoInDecisionTrace")
}
