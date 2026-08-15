//go:build acs

// Package cycle1473 materialises the acceptance criteria for the single task
// this lane committed in triage `## top_n`:
//
//   - gitstage-deterministic-classification → classify a `git add` failure from
//     the CAPTURED git stderr before assigning the recovery class, so git's own
//     deterministic fatals stop burning the transient retry budget.
//
// The three other items in this lane's fleet scope (ship-addall-staging-surface,
// ship-push-only-recovery, gitstage-collider-quotepath) are triage `## deferred`
// — already delivered and audit-accepted in cycle 1469 — so per R9.3 they get
// ZERO predicates here.
//
// Predicate strategy. The classifier is an UNEXPORTED seam inside
// go/internal/phases/ship, reachable only through its production caller
// stageExplicitPaths, so these predicates drive the in-package reachability
// contract (stage_classify_stderr_test.go) as a subprocess and assert on its
// exit code. Neither predicate greps production source — a magic string added to
// gitops.go cannot make either pass; only a classifier that stageExplicitPaths
// actually consults can (the cycle-85 degenerate-predicate ban).
//
// Flaky-shape compliance: each predicate runs ONE named package
// (./internal/phases/ship — not a `/...` sweep, and not one of the 40s+ suites),
// narrowed further with -run, with cmd.Dir set explicitly so the invocation is
// independent of the lane's process cwd.
package cycle1473

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// runShipTests runs the named -run pattern against the ship package in the
// worktree and returns combined output plus the exit code.
func runShipTests(t *testing.T, runPattern string) (string, int) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	cmd := exec.Command("go", "test", "./internal/phases/ship", "-run", runPattern, "-count=1")
	cmd.Dir = goDir
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("could not run `go test ./internal/phases/ship -run %s` in %s: %v", runPattern, goDir, err)
		}
	}
	return string(out), code
}

// TestC1473_001_GitStageDeterministicClassification is the primary predicate and
// the task's declared verifiableBy: every stderr fixture in the reachability
// contract must land in its declared recovery class when driven through
// stageExplicitPaths. RED today — the deterministic rows (rc=128 Invalid path /
// outside repository / pathspec-did-not-match, rc=1 gitignore advice) all
// currently return "transient".
func TestC1473_001_GitStageDeterministicClassification(t *testing.T) {
	out, code := runShipTests(t, "TestStageFailureClassification")
	if code != 0 {
		t.Errorf("`go test ./internal/phases/ship -run TestStageFailureClassification -count=1` exit=%d, want 0 — deterministic git-stage fatals are not classified from captured git_stderr yet.\n%s", code, out)
	}
}

// TestC1473_002_TransientShapesSurviveClassification is the load-bearing
// negative. A classifier that simply reclassified every `git add` failure as
// non-transient would pass predicate 001's deterministic rows while deleting the
// retry that index-lock contention DEPENDS on (fleet lanes contend on .git/index
// constantly) — and would also let the Go-composed error text drive routing,
// breaking the trust boundary the inbox record calls out explicitly. This
// predicate runs only the transient-preservation subtests, so it fails loudly on
// exactly that over-reach.
func TestC1473_002_TransientShapesSurviveClassification(t *testing.T) {
	const pattern = "TestStageFailureClassification/(rc128_index_lock_contention_stays_transient|unrecognised_stderr_degrades_to_transient|empty_stderr_degrades_to_transient|go_error_text_alone_does_not_classify)"
	out, code := runShipTests(t, pattern)
	if code != 0 {
		t.Errorf("transient-preservation subtests exit=%d, want 0 — index-lock contention, unknown shapes, and Go-composed error text must NOT be reclassified.\n%s", code, out)
	}
	// A pattern that matched nothing would exit 0 while proving nothing.
	if strings.Contains(out, "warning: no tests to run") || strings.Contains(out, "[no tests to run]") {
		t.Errorf("the -run pattern matched no subtests — this predicate proved nothing:\n%s", out)
	}
}

// TestC1473_003_TwoStrikesRouterSurvives guards the cycle-1440 router already in
// production: the new stderr classifier must sit in FRONT of the two-strikes
// memo, not replace it. An unclassifiable refusal keeps its first retry and
// still escalates to precondition on the second consecutive same-pathspec
// attempt.
func TestC1473_003_TwoStrikesRouterSurvives(t *testing.T) {
	out, code := runShipTests(t, "TestStageRefusal|TestStageFailureClassification_TwoStrikesStillApplies")
	if code != 0 {
		t.Errorf("two-strikes router tests exit=%d, want 0 — the stderr classifier must not replace the cycle-1440 refusal memo.\n%s", code, out)
	}
}
