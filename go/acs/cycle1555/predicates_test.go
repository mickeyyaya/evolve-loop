//go:build acs

// Package cycle1555 materialises the cycle-1555 acceptance criteria for the
// two fleet-scoped tasks pinned to this lane (live inbox record
// `red-first-deliverable-reds-main`):
//
//   - ship-added-test-red-gate          → a newly added Go test that fails in
//     the lane worktree must block ship with CodeRepoContractGate; selection
//     is bounded to newly-added `_test.go` files (modified/non-test excluded).
//   - ship-added-test-production-proof  → the block fires through the real
//     production caller (Phase.runNative), before any git/ship action, while
//     an honest `t.Skip` reproducer stays non-blocking.
//
// The live defect. Three lanes reached main with an intentionally red
// reproduction test because the existing repo-contract scanner
// (go/internal/phases/ship/repocontract.go) only runs FOUR fixed guard
// packages — it has no consumer for an arbitrary new test file added by the
// shipping diff itself. `runRepoContractGate`/`Phase.runNative` are the sole
// pre-push boundary (repocontract.go, ship.go:144); a detector that only
// exists as a helper and is never reached from there is the exact class of
// unconsumed signal this cycle's inbox record calls out.
//
// Predicate strategy — behavioral, never source-grep (the cycle-85
// degenerate-predicate ban). Each predicate DRIVES the system under test by
// running the named Go unit test in ONE named package with `-run` narrowing
// (per the flaky-predicate-shape rules: no `./...` sweeps, no wall-clock
// bounds, no literal PIDs, cmd.Dir always set explicitly) and asserting the
// `--- PASS: <TestName>` line is present. The PASS-line assertion is the
// anti-vacuous guard: `go test -run '^TestDoesNotExist$'` exits 0 with
// "no tests to run", so exit-code-only checking would pass on an EMPTY repo.
//
// Predicate map:
//
//	001 — a newly added failing test blocks ship w/ CodeRepoContractGate (T1 AC1)
//	002 — selection ignores modified tests, non-test files, empty diffs (T1 AC2/AC3)
//	003 — the four pre-existing gate tests are still GREEN (anti-weakening)
//	004 — WIRING PROOF: Phase.runNative stops before git/ship on a real added red test (T2 AC1)
//	005 — a `t.Skip`-ped newly added test does NOT block at the gate (T2 AC2)
package cycle1555

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const shipPkg = "./internal/phases/ship"

// requirePassing runs ONE named package with `-run` narrowed to the named
// tests and requires a `--- PASS:` line for EVERY name. Both halves are
// load-bearing: exit 0 alone is satisfied by "no tests to run" (a missing test
// would otherwise read as success), and a PASS line alone cannot appear if the
// package fails to build.
func requirePassing(t *testing.T, pkg string, names ...string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	pattern := "^(" + strings.Join(names, "|") + ")$"
	cmd := exec.Command("go", "test", "-count=1", "-v", "-run", pattern, pkg)
	// cmd.Dir explicitly: the predicate must resolve the module from the lane
	// worktree, never from the process cwd (which differs per fleet lane).
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	text := string(out)
	for _, name := range names {
		if !strings.Contains(text, "--- PASS: "+name) {
			t.Errorf("%s %s: no `--- PASS: %s` line (test missing, failing, or package did not build)\nrun error: %v\noutput:\n%s",
				pkg, pattern, name, err, tail(text))
			return
		}
	}
	if err != nil {
		t.Errorf("%s %s: go test exited non-zero (%v)\noutput:\n%s", pkg, pattern, err, tail(text))
	}
}

// tail trims predicate failure output to the last 60 lines so a broken build
// does not flood the audit log.
func tail(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > 60 {
		lines = append([]string{fmt.Sprintf("... (%d earlier lines elided)", len(lines)-60)}, lines[len(lines)-60:]...)
	}
	return strings.Join(lines, "\n")
}

// TestC1555_001_AddedFailingTestBlocksShip — T1 AC1, the crux reproduction.
// A newly added, genuinely failing Go test outside the four fixed suites must
// be caught by the real scanner and fail the ship with the DEDICATED code,
// naming the failing test.
func TestC1555_001_AddedFailingTestBlocksShip(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_NewlyAddedFailingTestBlocksShip")
}

// TestC1555_002_SelectionBoundedToNewlyAddedTestFiles — T1 AC2/AC3. A modified
// (not added) test going red, a newly added non-test `.go` file, and an empty
// candidate set must never trip the gate — an over-eager detector is exactly
// as dangerous as an under-eager one.
func TestC1555_002_SelectionBoundedToNewlyAddedTestFiles(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_AddedTestSelectionIgnoresModifiedAndNonTestFiles")
}

// TestC1555_003_ExistingGateContractPreserved — anti-weakening. The four
// pre-existing gate behaviours must survive this cycle's addition unmodified.
func TestC1555_003_ExistingGateContractPreserved(t *testing.T) {
	requirePassing(t, shipPkg,
		"TestRepoContractGate_OffSkips",
		"TestRepoContractGate_EnforceGreenPasses",
		"TestRepoContractGate_EnforceRedFailsWithDedicatedCode",
		"TestRepoContractGate_UnknownStageFailsTowardEnforce",
	)
}

// TestC1555_004_ProductionCallerStopsBeforeShip — T2 AC1, the WIRING PROOF.
// Drives the PRODUCTION caller (Phase.runNative) with a REAL newly added
// failing test — not a faked pack outcome — so a detector correct only in
// isolation, or never reached from the native ship path, cannot pass this.
func TestC1555_004_ProductionCallerStopsBeforeShip(t *testing.T) {
	requirePassing(t, shipPkg, "TestPhaseRunNative_NewlyAddedFailingTestPreventsRun")
}

// TestC1555_005_SkippedReproducerDoesNotBlock — T2 AC2, the negative case.
// An honestly `t.Skip`-ped newly added test must not be classified as a
// failure at the repo-contract gate when driven through Phase.runNative.
func TestC1555_005_SkippedReproducerDoesNotBlock(t *testing.T) {
	requirePassing(t, shipPkg, "TestRunNative_AddedSkippedTestDoesNotBlockShip")
}
