//go:build acs

// Package cycle1513 materialises the acceptance criteria for this lane's single
// fleet-scoped task, `contract-correction-verbatim-output-fidelity`: reland the
// cycle-1510 salvage regression lock for `composeCorrection`
// (go/internal/core/retry_backoff_test.go) with ZERO product-code change.
//
// Honest framing, stated up front (inst-L1508b, the lesson this whole todo
// carries): the property under lock — the rejection reason survives
// `composeCorrection` byte-for-byte — ALREADY HOLDS on unmodified source
// (retry_backoff.go:12-17 concatenates with `+`). The deliverable is therefore
// the LOCK, not a behaviour change, and these predicates are PRE-EXISTING GREEN
// in this worktree: the salvage snapshot commit 893ebcd2 is already an ancestor
// of this lane's branch, so the test file and its eval are already tracked here.
// They are absent from origin/main, so the cycle's ship is what actually lands
// them — which is exactly the state these predicates pin. See test-report.md
// §RED Run Output for the executed evidence.
//
// Predicate strategy — every load-bearing assertion runs a real subprocess and
// asserts on its exit code / output (the cycle-85 grep-only ban):
//
//   - 001 asserts the locked test file is present AND git-TRACKED (disk presence
//     alone passes for a gitignored file silently dropped at ship, cycle-93).
//   - 002 is the crux: it RUNS the three locked tests and asserts each named
//     test — and every subtest of the verbatim table — actually reported PASS.
//     A stub file with the right name, or a `-run` pattern matching nothing,
//     fails this; exit 0 alone is not accepted as evidence.
//   - 003 is the anti-tautology guard: `git diff origin/main` on the PRODUCT
//     file must be empty. A "fix" that edits composeCorrection to satisfy its
//     own lock is precisely the cycle-1508 defect this task exists to close.
//   - 004 asserts the landed file is gofmt-clean.
//
// Roots: RepoRoot is the worktree (where the lock lands and where the ship
// commit is taken from), which is the correct root for all four.
package cycle1513

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// lockRelPath is the relanded regression lock (scout-report Task 1 targetFiles).
const lockRelPath = "go/internal/core/retry_backoff_test.go"

// productRelPath is the file the lock pins. It MUST NOT change this cycle.
const productRelPath = "go/internal/core/retry_backoff.go"

// corePkg is the single named package the locked tests live in. Every
// invocation below narrows it with -run: internal/core is a known-slow suite and
// an unnarrowed shell of it is a fleet-load flake generator.
const corePkg = "./internal/core"

// lockedTests are the three test funcs the salvage snapshot contributes.
var lockedTests = []string{
	"TestComposeCorrection_CarriesReasonVerbatim",
	"TestComposeCorrection_FramingSurroundsTheReason",
	"TestComposeCorrection_EmptyReasonStillProducesADirective",
}

// verbatimSubtests are the table rows of the verbatim case. They are asserted
// individually so a lock degraded to a single happy-path row (or to an empty
// table, which still reports the parent as PASS) cannot satisfy the criterion.
var verbatimSubtests = []string{
	"single_line_with_code_token",
	"multiline_multi_violation_summarize_rendering",
	"unicode_and_punctuation",
	"trailing_and_leading_whitespace_is_preserved",
	"percent_and_backslash_are_not_format_interpreted",
}

// TestC1513_001_LockFileLandedAndTracked pins AC1: the regression lock exists on
// disk in this worktree AND is tracked by git, so the ship commit carries it.
func TestC1513_001_LockFileLandedAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, lockRelPath)) {
		t.Fatalf("RED: %s missing on disk under %s — the salvage reland did not happen", lockRelPath, root)
	}
	_, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", lockRelPath)
	if err != nil || code != 0 {
		t.Errorf("RED: %s is UNTRACKED (git ls-files exit=%d err=%v stderr=%q) — an untracked lock is dropped at ship and locks nothing",
			lockRelPath, code, err, strings.TrimSpace(stderr))
	}
}

// TestC1513_002_LockedTestsExecuteAndPass pins AC2 and is this cycle's crux: it
// runs the locked tests for real and requires each named test AND each verbatim
// table row to report PASS. Exit code alone is deliberately not sufficient — a
// -run pattern that matches nothing also exits 0.
func TestC1513_002_LockedTestsExecuteAndPass(t *testing.T) {
	root := acsassert.RepoRoot(t)
	pattern := "^(" + strings.Join(lockedTests, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-v", "-run", pattern, corePkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: locked tests did not pass (exit=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
	for _, name := range lockedTests {
		if !strings.Contains(stdout, fmt.Sprintf("--- PASS: %s ", name)) {
			t.Errorf("RED: %s did not report PASS — it is missing, renamed, or never ran.\nstdout:\n%s", name, stdout)
		}
	}
	for _, sub := range verbatimSubtests {
		want := fmt.Sprintf("--- PASS: %s/%s ", lockedTests[0], sub)
		if !strings.Contains(stdout, want) {
			t.Errorf("RED: verbatim table row %q did not report PASS — the lock has been narrowed to a weaker table.\nstdout:\n%s", sub, stdout)
		}
	}
}

// TestC1513_003_ProductFileUntouched pins AC4, the anti-tautology guard: the
// locked product file must be byte-identical to origin/main. This is the
// cycle-1508 defect in predicate form — a lock is worthless if the same commit
// is free to move the thing it locks.
func TestC1513_003_ProductFileUntouched(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"git", "-C", root, "diff", "origin/main", "--", productRelPath)
	if err != nil || code != 0 {
		t.Fatalf("could not diff %s against origin/main (exit=%d err=%v stderr=%q)", productRelPath, code, err, strings.TrimSpace(stderr))
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("RED: %s CHANGED vs origin/main — this is a test-only reland; a product edit makes the lock tautological.\ndiff:\n%s",
			productRelPath, stdout)
	}
}

// TestC1513_004_LockFileGofmtClean pins AC5: the relanded file is gofmt-clean,
// so the reland cannot red the repo-wide format check.
func TestC1513_004_LockFileGofmtClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput("gofmt", "-l", filepath.Join(root, lockRelPath))
	if err != nil || code != 0 {
		t.Fatalf("gofmt failed to run (exit=%d err=%v stderr=%q)", code, err, strings.TrimSpace(stderr))
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("RED: gofmt -l reported %s as unformatted", strings.TrimSpace(stdout))
	}
}
