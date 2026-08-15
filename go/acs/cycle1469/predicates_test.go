//go:build acs

// Package cycle1469 materialises the cycle-1469 acceptance criteria for the two
// top_n tasks pinned to this lane. Both are the same root cause — a git-output
// path reader in go/internal/phases/ship that was never enrolled in the
// cycle-1108 quote-path contract:
//
//   - gitstage-collider-quotepath  → detectColliders (gitops.go) reads
//     `git diff --name-only` and `git status --porcelain` without
//     `-c core.quotePath=false` and with a naive strip-the-outer-quotes
//     decode, so a C-quoted incoming filename compares as escaped text that
//     exists on no disk and the collider is invisible to the ff-merge
//     pre-flight AND to the repair ladder.
//   - gitstage-rename-arrow-parse  → porcelainChangedPaths / stagedGonePaths
//     (manifest.go) split the whole payload on every " -> ", but git quotes
//     rather than escapes a filename containing that sequence (verified
//     against git 2.50.1: `?? "we -> ird.txt"`, and `core.quotePath=false`
//     does NOT suppress it), so one rename is torn into 3-4 unbalanced-quote
//     fragments that `git add` rejects rc=128 — failing the ENTIRE staging.
//
// Predicate strategy. Both fixes live in UNEXPORTED functions of an internal
// package, so the only honest behavioural driver is the package's own test
// binary: each predicate shells ONE named package (`./internal/phases/ship`)
// narrowed with `-run` to the RED contract this cycle authored, and asserts on
// its exit code. That is the sanctioned shape — never a `./...` sweep, never a
// wall-clock bound, never a bare `git` (every git call is `-C <root>`). No
// predicate here source-greps production code: adding a magic string to
// gitops.go/manifest.go cannot make any of them pass, only a real decode/
// tokenizer change can.
//
// 003 is the anti-regression half: the pre-existing quote-path and staging
// contracts (cycle-1067/1101/1108) must still pass, so a tokenizer rewrite that
// buys the quoted-arrow case by breaking plain renames fails the cycle.
package cycle1469

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// shipPkg is the ONE named package every predicate runs. Never widened to a
// `/...` sweep and never `./internal/core` or `./cmd/evolve` (the 40s+ suites)
// — flaky-predicate-shape rules.
const shipPkg = "./internal/phases/ship"

// redContractFiles are the RED contract this cycle authored. They must be
// present on disk AND shippable (not gitignored) — see assertShippable for why
// tracking itself is the wrong assertion at audit time (cycle-93).
var redContractFiles = []string{
	"go/internal/phases/ship/gitops_collider_quotepath_test.go",
	"go/internal/phases/ship/stage_quotepath_test.go",
}

// runShipTests executes `go test -C <root>/go <shipPkg> -run <pattern>` and
// returns combined output plus exit code. -C keeps the module root explicit
// instead of inheriting the predicate process's cwd (which differs between the
// main tree, a worktree, and each fleet lane).
func runShipTests(t *testing.T, root, pattern string) (string, int) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", filepath.Join(root, "go"), shipPkg, "-run", pattern, "-count=1")
	out := stdout + stderr
	if code == -1 {
		t.Fatalf("could not run `go test -run %s`: %v\n%s", pattern, err, out)
	}
	return out, code
}

// assertShippable is the cycle-93 pairing for a file-backed contract: the file
// must exist on disk AND be SHIPPABLE — i.e. not gitignored, since a gitignored
// test file is silently dropped at ship and the RED contract it encodes
// evaporates. Tracking itself is deliberately not asserted: predicates run at
// audit, before the ship stages the cycle's new files, so `ls-files` would
// false-RED on a legitimately new test file. `git check-ignore` exits 1 when
// the path is NOT ignored, which is the passing case.
func assertShippable(t *testing.T, root, rel string) {
	t.Helper()
	if !acsassert.FileExists(t, filepath.Join(root, rel)) {
		t.Fatalf("RED: %s missing on disk — the contract it encodes does not exist", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput(
		"git", "-C", root, "check-ignore", "-q", rel); code == 0 {
		t.Errorf("%s is gitignored — it will be dropped at ship and the RED contract it encodes evaporates", rel)
	}
}

// TestC1469_001_ColliderQuotePathDecodesGitOutput — gitstage-collider-quotepath.
// Drives the RED contract for detectColliders: quoted non-ASCII, quote-bearing
// and backslash-bearing incoming paths must compare as their literal
// repo-relative names (from BOTH the porcelain and the diff --name-only
// stream), a non-collider must stay unreported, and both reads must carry
// `-c core.quotePath=false` before the subcommand.
func TestC1469_001_ColliderQuotePathDecodesGitOutput(t *testing.T) {
	root := acsassert.RepoRoot(t)
	assertShippable(t, root, redContractFiles[0])

	out, code := runShipTests(t, root, "TestDetectColliders_QuotePath")
	if code != 0 {
		t.Errorf("collider quote-path contract RED (exit %d) — a C-quoted incoming filename still fails to compare as its literal path, so the ff-merge pre-flight and the repair ladder both miss a real untracked main-side collider:\n%s", code, out)
	}
	// A `-run` typo would exit 0 having run nothing; require the named tests
	// to have actually executed.
	for _, name := range []string{
		"TestDetectColliders_QuotePathDecodesPorcelainEntries",
		"TestDetectColliders_QuotePathDecodesDiffNameOnly",
		"TestDetectColliders_QuotePathNonCollidersStaySilent",
		"TestDetectColliders_QuotePathDisabledOnGitReads",
	} {
		if _, c := runShipTests(t, root, "^"+name+"$"); c != 0 {
			t.Errorf("%s did not pass (exit %d) — this predicate requires every named collider case to run and pass, not just the pattern to match nothing", name, c)
		}
	}
}

// TestC1469_002_RenameArrowParsedStructurally — gitstage-rename-arrow-parse.
// Drives the RED contract for porcelainChangedPaths/stagedGonePaths: a ` -> `
// inside a quoted filename is path CONTENT and must yield exactly two decoded
// endpoints, malformed quoted input must degrade verbatim, and ordinary
// unquoted renames must be byte-identical to pre-change behaviour.
func TestC1469_002_RenameArrowParsedStructurally(t *testing.T) {
	root := acsassert.RepoRoot(t)
	assertShippable(t, root, redContractFiles[1])

	for _, name := range []string{
		"TestPorcelainChangedPaths_QuotedRenameArrowKeepsBothEndpoints",
		"TestStagedGonePaths_QuotedRenameArrowSourceDecodes",
		"TestPorcelainChangedPaths_RenameArrowMalformedIsSafe",
		"TestPorcelainChangedPaths_OrdinaryRenameArrowUnchanged",
	} {
		out, code := runShipTests(t, root, "^"+name+"$")
		if code != 0 {
			t.Errorf("%s RED (exit %d) — a quoted rename endpoint holding the delimiter is still torn into unbalanced-quote fragments, which `git add` rejects rc=128 and which fails the whole staging:\n%s", name, code, out)
		}
		if !strings.Contains(out, name) && !strings.Contains(out, "ok ") {
			t.Errorf("%s produced no recognisable go-test output — the pattern may have matched nothing:\n%s", name, out)
		}
	}
}

// TestC1469_003_ExistingQuotePathAndStagingContractsHold — the anti-regression
// predicate, and the reason this cycle cannot be passed by loosening a parser.
// The cycle-1067 explicit-staging and cycle-1101/1108 quote-path contracts
// predate this cycle; a tokenizer or decode change that buys the quoted-arrow
// case by breaking plain renames, ASCII classification, or the ignored-path
// filter trades one rc=128 ship-killer for another and must fail here.
func TestC1469_003_ExistingQuotePathAndStagingContractsHold(t *testing.T) {
	root := acsassert.RepoRoot(t)

	for _, pattern := range []string{
		"TestPorcelainChangedPaths_QuotePath",
		"TestStageExplicitPaths_QuotePathDisabledOnGitReads",
		"TestDropIgnoredPaths_QuotePath",
		"TestShipDirect_CycleClass_StagesDeclaredPathsNotAddAll",
	} {
		out, code := runShipTests(t, root, pattern)
		if code != 0 {
			t.Errorf("pre-existing contract %q regressed (exit %d) — the quote-path fixes must not change ordinary staging or collider classification:\n%s", pattern, code, out)
		}
	}
}
