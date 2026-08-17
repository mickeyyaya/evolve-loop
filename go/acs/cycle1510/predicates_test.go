//go:build acs

// Package cycle1510 materialises the cycle-1510 acceptance criteria for the two
// fleet-scoped carryover tasks pinned to this lane, both of which FAILED the
// cycle-1508 audit and were re-scoped by scout against the CURRENT tree:
//
//   - contract-correction-hash-freshness-classification → add the unexported
//     core.contractArtifactDetermined allowlist classifier (no consumer)
//   - contract-correction-verbatim-output-fidelity      → regression-lock
//     composeCorrection's verbatim inclusion + enumerate blocking-gate
//     amendments in build-report.md
//
// Predicate strategy. Both subjects are UNEXPORTED functions in
// internal/core, so a predicate in this package cannot call them directly.
// Each behavioural predicate therefore RUNS the real in-package unit tests as a
// subprocess and asserts on the exit code — the system under test genuinely
// executes, and adding a magic string to a source file cannot make it pass
// (the cycle-85 degenerate-predicate ban). Every invocation is narrowed with
// `-run` to specific test names and pinned to ONE named package via `go -C`,
// per the flaky-predicate-shape rules: no `./...` sweep, no wall-clock bound,
// no literal PID, no bare `git` resolving cwd from the process.
//
// 001/002 carry task 1 (positive allowlist + the fail-closed/negative axis);
// 003/004 carry task 2 (verbatim lock + the "production code UNCHANGED"
// constraint that keeps this task honest about being verification, not a
// change); 005 carries the build-report amendment-enumeration contract that
// closes inst-L1508b's process defect.
package cycle1510

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// corePkg is the ONE named package every predicate below tests. Never widened
// to ./... — internal/core is a known-slow suite, so each run is additionally
// narrowed by -run.
const corePkg = "./internal/core"

// runCoreTest executes the named core tests in the repo's go module and returns
// combined output plus the exit code. `go -C <dir>` pins the module root
// explicitly rather than inheriting the process cwd, which differs between the
// main tree, this worktree, and each fleet lane.
func runCoreTest(t *testing.T, runPattern string) (string, int) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir, "test", "-count=1", "-run", runPattern, corePkg)
	// SubprocessOutput reports a non-zero exit AS an error, so err alone cannot
	// distinguish "the tests failed" (the verdict this predicate wants) from
	// "the toolchain could not be launched" (an environment fault). code == -1
	// is the launch fault; every code >= 0 is a real `go test` verdict.
	if code < 0 {
		t.Fatalf("could not launch `go test -run %s %s` in %s: %v", runPattern, corePkg, goDir, err)
	}
	return out + "\n" + errOut, code
}

// workspaceDir resolves this cycle's run workspace. The predicate executes from
// the lane worktree, but phase reports are written to the MAIN tree's
// .evolve/runs/cycle-1510. Both are checked, worktree first, so the predicate
// works under a lane run and a console run alike.
func workspaceDir(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	candidates := []string{filepath.Join(root, ".evolve", "runs", "cycle-1510")}
	// A lane worktree lives at <project_root>/.evolve/worktrees/<lane>; strip
	// that suffix to reach the project root that owns the workspace.
	if i := strings.Index(root, string(filepath.Separator)+filepath.Join(".evolve", "worktrees")+string(filepath.Separator)); i >= 0 {
		candidates = append(candidates, filepath.Join(root[:i], ".evolve", "runs", "cycle-1510"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return candidates[0]
}

// TestC1510_001_ArtifactDeterminedClassifierAllowlistsTheSixCodes drives the
// real classifier through its in-package table test. RED until
// contractArtifactDetermined exists (the package does not compile), then GREEN
// only when all six artifact-determined codes return true.
func TestC1510_001_ArtifactDeterminedClassifierAllowlistsTheSixCodes(t *testing.T) {
	out, code := runCoreTest(t, "^TestContractArtifactDetermined_")
	if code != 0 {
		t.Errorf("contractArtifactDetermined contract FAILED (exit %d).\n%s", code, out)
	}
	// A -run pattern that matches nothing exits 0 with "no tests to run": prove
	// the tests were actually selected, so a deleted/renamed test cannot pass
	// this predicate by vacuity.
	if strings.Contains(out, "no tests to run") || strings.Contains(out, "[no test files]") {
		t.Errorf("no TestContractArtifactDetermined_* test executed — the classifier's contract is unverified, not satisfied.\n%s", out)
	}
}

// TestC1510_002_ArtifactDeterminedFailsClosedOnStrayAndUnknown is the NEGATIVE
// axis and the crux of inst-L1508a: the classifier must reject
// stray_in_worktree (repaired outside the watched bytes) and every unknown
// code. A denylist implementation passes 001 and fails here.
func TestC1510_002_ArtifactDeterminedFailsClosedOnStrayAndUnknown(t *testing.T) {
	out, code := runCoreTest(t,
		"^TestContractArtifactDetermined_StrayInWorktreeIsFalse$|^TestContractArtifactDetermined_Table$/^(stray_in_worktree|unknown_code|empty_string|case_variant_is_not_the_code|bracketed_rendering_is_not_a_code)$")
	if code != 0 {
		t.Errorf("fail-closed contract FAILED (exit %d) — an unchanged artifact hash must never be read as \"no new evidence\" for a location-class or unknown violation code.\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no fail-closed subtest executed — the negative axis is unverified.\n%s", out)
	}
}

// TestC1510_003_ComposeCorrectionCarriesReasonVerbatim runs the regression lock
// for task 2. Expected PRE-EXISTING GREEN: the property already holds on the
// pre-change code (retry_backoff.go:12-17). The predicate's job is to prove the
// LOCK exists and passes, not to prove a behaviour change.
func TestC1510_003_ComposeCorrectionCarriesReasonVerbatim(t *testing.T) {
	out, code := runCoreTest(t, "^TestComposeCorrection_")
	if code != 0 {
		t.Errorf("composeCorrection verbatim-fidelity lock FAILED (exit %d).\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no TestComposeCorrection_* test executed — the verbatim property is unlocked, which IS this task's deliverable.\n%s", out)
	}
}

// TestC1510_004_ComposeCorrectionProductionCodeUnchanged enforces the honesty
// constraint scout derived from inst-L1508b: because the verbatim property is
// green-from-birth, no product-code edit may claim to establish it. The
// deliverable is a test, so composeCorrection's own source must be untouched
// relative to the cycle's base commit.
func TestC1510_004_ComposeCorrectionProductionCodeUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	// `git -C` — never a bare `git`, whose repo resolves from process cwd.
	out, errOut, code, err := acsassert.SubprocessOutput(
		"git", "-C", root, "diff", "origin/main", "--", "go/internal/core/retry_backoff.go")
	if code < 0 {
		t.Fatalf("could not launch git diff in %s: %v", root, err)
	}
	if code > 1 {
		t.Fatalf("git diff failed (exit %d): %s", code, errOut)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("go/internal/core/retry_backoff.go was MODIFIED, but this task's verbatim criterion is green-from-birth on the pre-change code — a product edit here would be a tautologically-green claim (the exact cycle-1508 M1 defect). Diff:\n%s", out)
	}
}

// TestC1510_005_BuildReportEnumeratesBlockingGateAmendments closes
// inst-L1508b's SECOND defect: a blocking premise-challenge returned three
// amendments, two were adopted mechanically and the third — a re-scoping one —
// was silently dropped. The report must account for every amendment
// explicitly, or state on the record that none were returned. Silence is the
// failure mode.
func TestC1510_005_BuildReportEnumeratesBlockingGateAmendments(t *testing.T) {
	report := filepath.Join(workspaceDir(t), "build-report.md")
	if !acsassert.FileExists(t, report) {
		t.Fatalf("build-report.md absent at %s", report)
	}
	if !acsassert.FileMatchesRegex(t, report, `(?i)^#+\s*Amendments`) {
		t.Errorf("build-report.md has no `## Amendments` section. Every amendment returned by a blocking gate must be enumerated adopted/rejected+reason — and if none were returned, that must be stated explicitly. A silently dropped re-scoping amendment is the cycle-1508 M1 process defect.")
	}
	if !acsassert.FileContainsAny(report, "adopted", "Adopted", "none returned", "No amendments", "no amendments") {
		t.Errorf("build-report.md's amendment accounting names no disposition — expected each amendment marked adopted/rejected with a reason, or an explicit \"no amendments returned\".")
	}
}
