//go:build acs

// Package cycle766 materializes the cycle-766 acceptance criteria for the sole
// committed top_n task todo-fleet-lane-provisioning-split (triage-report.md
// ## top_n; scout's two leak-recovery proposals were DEFERRED by triage as
// out-of-scope for this fleet lane, so per R9.3 no predicates bind to them).
//
// Task source: inbox id fleet-lane-provisioning-split (weight 0.9, cycle-640
// incident): fleet provisioning never pinned ONE lane identity for the whole
// run — scout scouted lane A's goal while triage was handed lane B's
// fleet_scope, enabling the builder-task-binding failure class. Fix contract:
// pin the lane's scope (todo ids + goal hash) to <workspace>/lane-scope.json
// before any phase runs, inject THAT scope into every phase, and fail the
// scout→triage transition on a goal-hash mismatch.
//
// AC map (1:1), from the inbox item's acceptance[] list:
//
//	AC1 concurrent lanes each see ONLY their own scope, sourced from
//	    lane-scope.json → C766_001 (file injects to all phases),
//	    C766_002 (two lanes + cross-lane env drift; file must win),
//	    C766_003 (absent file keeps legacy env fallback),
//	    C766_004 (env-scoped run materializes lane-scope.json pre-phase)
//	AC2 scout-report goal-hash mismatch vs lane-scope.json fails the
//	    scout→triage transition with an explicit abort reason
//	    → C766_005 (mismatch aborts, triage never runs),
//	      C766_006 (match proceeds — no blanket abort),
//	      C766_007 (goal_hash-less report fails OPEN — no false aborts,
//	      the cycle-760..762 destruction class)
//	AC3 go test -race PASS on touched packages → every predicate runs the
//	    unit contract under -race (apicover runs in the repo-wide gate)
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit contract in internal/core, which EXERCISES the orchestrator through
// full RunCycle drives with recording fake runners and real workspace files —
// behavioral via subprocess, no source-grep predicates (cycle-85 rule). The
// `-v` + "--- PASS:" guard rejects a rename/no-tests-matched silent green.
// Adversarial axes embedded in the unit contract: negative (env drift must
// NOT leak through; mismatch must NOT proceed; match/keyless must NOT abort),
// edge (absent lane-scope.json, missing goal_hash key), semantic (pin vs
// inject vs coherence-gate are separate behaviors).
package cycle766

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

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

// AC1: a supervisor-provisioned lane-scope.json is the fleet_scope source for
// every phase's Context (scout/triage/tdd/build), no env needed.
func TestC766_001_lane_scope_file_injects_fleet_scope(t *testing.T) {
	runGoTest(t, corePkg, "TestLaneScopePin_FileInjectsFleetScopeToPhases")
}

// AC1 negative: two lanes with distinct lane-scope.json each see ONLY their
// own scope even when the env snapshot carries the OTHER lane's scope — the
// exact cycle-640 cross-lane drift must lose to the pinned file.
func TestC766_002_two_lanes_see_only_own_scope(t *testing.T) {
	runGoTest(t, corePkg, "TestLaneScopePin_TwoLanesSeeOnlyOwnScope")
}

// AC1 edge (back-compat): no lane-scope.json ⇒ legacy env fallback preserved,
// so the pin cannot break sequential / older-supervisor runs.
func TestC766_003_absent_file_falls_back_to_env(t *testing.T) {
	runGoTest(t, corePkg, "TestLaneScopePin_AbsentFileFallsBackToEnv")
}

// AC1 pin: an env-scoped run materializes lane-scope.json (todo_ids +
// goal_hash) into the run workspace before any phase output exists.
func TestC766_004_lane_scope_materialized_from_env(t *testing.T) {
	runGoTest(t, corePkg, "TestLaneScopePin_MaterializedFromEnvBeforePhases")
}

// AC2: scout-report goal_hash ≠ lane-scope.json goal_hash aborts the cycle at
// the scout→triage transition with an explicit "lane-scope goal-hash
// mismatch" reason; triage never runs.
func TestC766_005_goal_hash_mismatch_aborts_before_triage(t *testing.T) {
	runGoTest(t, corePkg, "TestLaneScopePin_ScoutGoalHashMismatchAbortsBeforeTriage")
}

// AC2 negative: a MATCHING goal hash proceeds to triage with verdict PASS —
// the coherence gate must not be a blanket abort.
func TestC766_006_goal_hash_match_proceeds(t *testing.T) {
	runGoTest(t, corePkg, "TestLaneScopePin_ScoutGoalHashMatchProceeds")
}

// AC2 fail-open edge: a scout report WITHOUT a goal_hash key proceeds — an
// over-strict gate would recreate the cycle-760..762 false-abort class.
func TestC766_007_goal_hash_absent_fails_open(t *testing.T) {
	runGoTest(t, corePkg, "TestLaneScopePin_ScoutReportWithoutGoalHashProceeds")
}
