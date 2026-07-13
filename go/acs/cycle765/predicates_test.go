//go:build acs

// Package cycle765 materializes the cycle-765 acceptance criteria for the sole
// committed top_n task width-scaled-binding-retry (triage-report.md ## top_n;
// the scout's two proposals were DROPPED by triage as out-of-scope for this
// fleet lane, so per R9.3 no predicates bind to them).
//
// Task source: inbox id width-scaled-binding-retry (weight 0.93, cycle-759
// incident): ship failed AUDIT_BINDING_HEAD_MOVED because a sibling landed
// during the audit→ship gap, and a FIXED recovery budget of 2 aborted a clean
// cycle — with N lanes racing one main the budget must scale max(2, width+1)
// for contention-class codes, with jittered backoff so siblings don't
// re-collide in lockstep, while non-contention transients keep the constant
// budget.
//
// AC map (1:1), from the inbox item's acceptance[] list:
//
//	AC1 contention budget scales with fleet width      → C765_001 (+ C765_004
//	    pinning GIT_FLEET_REBASE_NEEDED via the pure classifier)
//	AC2 jittered backoff between re-audits             → C765_002
//	AC3 non-contention transients keep constant budget → C765_003
//	AC4 go test -race PASS                             → every predicate runs
//	    the unit contract under -race (apicover runs in the repo-wide gate)
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit contract in internal/core, which EXERCISES the orchestrator's ship
// recovery through full RunCycle drives against a persistently-failing ship
// runner — behavioral via subprocess, no source-grep predicates (cycle-85
// rule). The `-v` + "--- PASS:" guard rejects a rename/no-tests-matched
// silent green. The unit contract embeds the adversarial axes: negative
// (transient must NOT scale; garbage/negative width must NOT scale), edge
// (width absent/1/0), semantic (budget scaling vs jitter vs classification
// are separate behaviors).
package cycle765

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

// AC1: recovery budget for contention-class ship errors scales with fleet
// width (max(2, width+1)) — includes the negative/edge sub-cases (width
// absent, 1, garbage, negative stay at the constant budget).
func TestC765_001_ContentionBudgetScalesWithFleetWidth(t *testing.T) {
	runGoTest(t, corePkg, "TestShipRecovery_ContentionBudgetScalesWithFleetWidth")
}

// AC2: jittered positive bounded backoff between contention re-audit attempts
// (anti-lockstep: distinct durations across attempts).
func TestC765_002_JitteredBackoffBetweenReaudits(t *testing.T) {
	runGoTest(t, corePkg, "TestShipRecovery_JitteredBackoffBetweenReaudits")
}

// AC3 (negative): non-contention transients keep the CONSTANT budget even at
// width 5 — scaling must not leak beyond the contention class.
func TestC765_003_NonContentionTransientKeepsConstantBudget(t *testing.T) {
	runGoTest(t, corePkg, "TestShipRecovery_NonContentionTransientKeepsConstantBudget")
}

// AC1b: the pure budget classifier pins GIT_FLEET_REBASE_NEEDED as
// contention-class (a live rebase cannot be staged in a unit cycle) and the
// width floor (0/1 → constant 2); integrity/transient codes never scale.
func TestC765_004_BudgetClassifierScalesOnlyContentionCodes(t *testing.T) {
	runGoTest(t, corePkg, "TestShipRecovery_BudgetClassifierScalesOnlyContentionCodes")
}
