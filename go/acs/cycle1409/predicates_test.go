//go:build acs

// Package cycle1409 materialises the cycle-1409 acceptance criteria for the two
// fleet-scoped tasks pinned to this lane (inbox item
// `repocontract-gate-false-red-swallowed-diag`):
//
//   - repocontract-gate-infra-ambiguity-classification → distinguish a genuine
//     repo-contract RED from an infra flake; retry once; new distinct ship code
//   - repocontract-scan-log-persistence                → tee scanner output to a
//     run artifact unconditionally + name the failing tests in the ship error
//
// The live defect. `defaultRepoContractTest` (go/internal/phases/ship/
// repocontract.go:48) returns `cmd.Run()`'s error verbatim, and
// `runRepoContractGate` wraps ANY non-nil error as CodeRepoContractGate. A
// build-cache contention / OOM-kill / module-fetch flake is therefore
// indistinguishable from a genuine red guard suite, and neither `cmd.Stdout`
// nor `cmd.Stderr` is teed to a run artifact — so the false RED that blocked
// cycles 1402/1403/1405 (preserved worktree e0638346 re-ran 4/4 GREEN against
// the identical tree; baseline cba017c5 also 4/4 GREEN) left no recoverable
// evidence.
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
//	001 — the new infra ship code exists in the vocabulary, non-aliasing (T1)
//	002 — a genuine test failure stays CodeRepoContractGate, runs ONCE (T1)
//	003 — a transient first failure + green retry ships, runs EXACTLY twice (T1)
//	004 — persistent ambiguity → the infra code, EXACTLY twice, no retry storm (T1)
//	005 — the four pre-existing gate tests are still GREEN (anti-weakening)
//	006 — the scan log artifact is written on BOTH the green and red paths (T2)
//	007 — the RED ship error message NAMES the failing tests (T2)
//	008 — WIRING PROOF: the production caller (runNative) reaches the gate with
//	      the run workspace, so the log lands in <runs>/cycle-N (T2)
package cycle1409

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	shipPkg    = "./internal/phases/ship"
	shiperrPkg = "./internal/shiperr"
)

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

// TestC1409_001_InfraShipCodeInVocabulary — AC1. The retry-exhausted infra
// classification MUST get its own ship error code with ShipClassPrecondition,
// NOT an alias of CodeRepoContractGate: the router/dashboards have to tell
// "fix your code" from "infra flake, safe to re-dispatch". Mirrors the
// CodeManifestGate / CodeRepoContractGate vocab-pin pattern already in
// go/internal/shiperr.
func TestC1409_001_InfraShipCodeInVocabulary(t *testing.T) {
	requirePassing(t, shiperrPkg, "TestCodeRepoContractInfra_Vocab")
}

// TestC1409_002_RealFailureStaysContractRedNoRetry — AC2 and the crux
// anti-regression of the WHOLE change: a genuinely red guard suite must still
// fail the ship closed with CodeRepoContractGate, and must NOT be retried (a
// retry on a real RED both doubles ship wall-time and risks a flaky-but-real
// suite passing by luck, masking the violation).
func TestC1409_002_RealFailureStaysContractRedNoRetry(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_RealTestFailureIsContractRedWithoutRetry")
}

// TestC1409_003_TransientRetriesOnceThenShips — AC3, the defect this cycle
// exists to kill. An unclassifiable nonzero exit on the first attempt followed
// by a clean pack on the retry MUST let the ship proceed (nil error) with the
// pack invoked EXACTLY twice. This is what would have unblocked cycles
// 1402/1403/1405.
func TestC1409_003_TransientRetriesOnceThenShips(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_TransientFailureRetriesOnceThenShips")
}

// TestC1409_004_PersistentAmbiguityIsInfraClassed — AC4, the NEGATIVE case
// (adversarial axis: this is the branch a no-op implementation cannot satisfy).
// Ambiguity on BOTH attempts must fail the ship with the new infra code — not
// silently proceed, and not be retried a third time (no retry storm).
func TestC1409_004_PersistentAmbiguityIsInfraClassed(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_PersistentAmbiguityIsInfraClassedExactlyTwoRuns")
}

// TestC1409_005_ExistingGateContractPreserved — AC5, anti-weakening. The four
// pre-existing gate behaviours (off skips, enforce+green passes, enforce+RED
// uses the DEDICATED code, unknown stage fails toward enforce) must survive the
// rework unmodified. Deleting or loosening one of these to make the new
// classification easier is a regression, not a fix.
func TestC1409_005_ExistingGateContractPreserved(t *testing.T) {
	requirePassing(t, shipPkg,
		"TestRepoContractGate_OffSkips",
		"TestRepoContractGate_EnforceGreenPasses",
		"TestRepoContractGate_EnforceRedFailsWithDedicatedCode",
		"TestRepoContractGate_UnknownStageFailsTowardEnforce",
	)
}

// TestC1409_006_ScanLogPersistedOnGreenAndRed — AC6. The scanner output must be
// teed to <runs>/cycle-N/ship-repocontract-scan.log UNCONDITIONALLY. Green-only
// or red-only persistence is insufficient: diagnosing a false RED requires the
// green baseline too, and a red run whose log is only written on failure is
// exactly what was missing on cycle-1403.
func TestC1409_006_ScanLogPersistedOnGreenAndRed(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_ScanLogPersistedOnGreenAndRedRuns")
}

// TestC1409_007_RedErrorNamesFailingTests — AC7. The failing test names parsed
// from the `go test -json` events must appear in the CodeRepoContractGate
// message so ship-error.json carries them directly, instead of the generic
// "scanner pack RED (exit status 1)" string that made cycle-1402/1403
// undiagnosable.
func TestC1409_007_RedErrorNamesFailingTests(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_RedErrorMessageNamesFailingTests")
}

// TestC1409_008_ProductionCallerThreadsWorkspace — AC8, the WIRING PROOF. A
// log-writing seam whose only caller is a test is dead code: the log path is
// derived from the run workspace, so the PRODUCTION caller
// (ship.Phase.runNative, go/internal/phases/ship/ship.go:147) must reach the
// gate carrying req.Workspace. The unit test must drive runNative itself — not
// runRepoContractGate directly — and assert the workspace arrived at the seam.
// This is the cycle-1064 anti-trap applied to the new parameter.
func TestC1409_008_ProductionCallerThreadsWorkspace(t *testing.T) {
	requirePassing(t, shipPkg, "TestRunNative_RepoContractGateReceivesRunWorkspace")
}
