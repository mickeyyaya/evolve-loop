//go:build acs

// Package cycle1566 carries the red-first-deliverable-reds-main lane's
// acceptance predicates for the two triage-committed tasks:
// `ship-new-test-red-gate` (the ship-time consumer for a newly added failing
// test) and `ship-new-test-red-production-wiring` (proof that the consumer is
// reached from the real production ship path, not merely from a helper).
//
// The lane's mechanism exists in the tree; cycle-1559's audit reproduced four
// defects in it against the real gate, and they are OPEN in
// `.evolve/runs/cycle-1566/defect-dispositions.json`. The predicates therefore
// split into pre-existing coverage (001-005, GREEN at RED time — pinned so the
// repair cannot weaken what already works) and this cycle's gaps (006-010, RED
// at RED time — one per open defect).
//
// H2 (007) is not hypothetical here: THIS file is a lone `//go:build acs`
// package newly added to this cycle's own shipping diff. Under the current
// gate it reports `[build failed]` and hard-blocks the lane's own honest ship
// with a false CodeRepoContractGate. The predicate that fixes the defect is
// the predicate that un-blocks its own ship.
//
// Predicate strategy — behavioral, never source-grep (cycle-85 ban): every
// predicate DRIVES the system by running the named Go test in ONE named
// package with `-run` narrowing, and requires a `--- PASS:` line for each
// name. Both halves are load-bearing: a non-matching `-run` pattern exits 0
// with "no tests to run", so exit-code-only checking would pass on an EMPTY
// repo, and a PASS line cannot appear if the package fails to build.
//
// Predicate map:
//
//	001 — T1 AC1 pre-existing: a newly added failing test blocks the ship
//	002 — T1 AC2 NEW: an explicit t.Skip reproducer does not block, at the gate
//	003 — T1 AC3 pre-existing: selection bounded to newly ADDED test files
//	004 — T2 AC1 pre-existing WIRING PROOF: runNative stops before git/ship
//	005 — T2 AC2 pre-existing: an honest skip is not blocked on that same path
//	006 — H1 NEW: a tag-guarded FAILING added test is not silently green
//	007 — H2 NEW: a tag-guarded GREEN added package is not a false RED
//	008 — H3 NEW: a string literal is not a build constraint
//	009 — M1 NEW: a failed discovery is recorded, never a silent disable
//	010 — M2 NEW: red messages are attributable to the scan that produced them
//	011 — anti-weakening: the four fixed-pack behaviours stay intact
package cycle1566

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
// tests and requires a `--- PASS:` line for EVERY name.
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

// TestC1566_001_NewlyAddedFailingTestBlocksShip — T1 AC1 (pre-existing).
func TestC1566_001_NewlyAddedFailingTestBlocksShip(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_NewlyAddedFailingTestBlocksShip")
}

// TestC1566_002_NewlyAddedSkippedTestDoesNotBlockShip — T1 AC2. The gate-level
// half of the skip convention; only the runNative half existed before.
func TestC1566_002_NewlyAddedSkippedTestDoesNotBlockShip(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_NewlyAddedSkippedTestDoesNotBlockShip")
}

// TestC1566_003_SelectionBoundedToNewlyAddedTestFiles — T1 AC3 (pre-existing).
func TestC1566_003_SelectionBoundedToNewlyAddedTestFiles(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_AddedTestSelectionIgnoresModifiedAndNonTestFiles")
}

// TestC1566_004_ProductionCallerStopsBeforeShip — T2 AC1, the WIRING PROOF
// (pre-existing): a real staged red test driven through Phase.runNative.
func TestC1566_004_ProductionCallerStopsBeforeShip(t *testing.T) {
	requirePassing(t, shipPkg, "TestPhaseRunNative_NewlyAddedFailingTestPreventsRun")
}

// TestC1566_005_SkippedReproducerDoesNotBlockProductionPath — T2 AC2
// (pre-existing), the negative half of the wiring proof.
func TestC1566_005_SkippedReproducerDoesNotBlockProductionPath(t *testing.T) {
	requirePassing(t, shipPkg, "TestRunNative_AddedSkippedTestDoesNotBlockShip")
}

// TestC1566_006_TaggedFailingAddedTestIsNotSilentlyGreen — H1, incident
// 25040cea's shape: a `//go:build integration` failing test compiled out of an
// untagged backstop run ships as green.
func TestC1566_006_TaggedFailingAddedTestIsNotSilentlyGreen(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_NewlyAddedTaggedFailingTestIsNotSilentlyGreen")
}

// TestC1566_007_TagGuardedGreenPackageIsNotFalseRed — H2, the defect this very
// file's `//go:build acs` header would otherwise trigger against its own ship.
func TestC1566_007_TagGuardedGreenPackageIsNotFalseRed(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_NewlyAddedTagGuardedGreenPackageIsNotFalseRed")
}

// TestC1566_008_LiteralBuildConstraintIsNotAnExclusion — H3, the
// bug-reproduction phase's pinned reproducer: a mention of a constraint in a
// Go string literal must not launder a genuinely failing test into an
// "excluded" record.
func TestC1566_008_LiteralBuildConstraintIsNotAnExclusion(t *testing.T) {
	requirePassing(t, shipPkg, "TestBugReproduction_AddedTestLiteralBuildConstraintIsNotExcluded")
}

// TestC1566_009_DiscoveryFailureIsRecorded — M1: a backstop that can disable
// itself invisibly makes the ship report claim coverage it never had.
func TestC1566_009_DiscoveryFailureIsRecorded(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_AddedTestDiscoveryFailureIsRecorded")
}

// TestC1566_010_RedMessagesAreAttributable — M2: the fixed pack keeps naming
// its four guard suites, and an added-test RED says so.
func TestC1566_010_RedMessagesAreAttributable(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_RedMessagesDistinguishFixedPackFromAddedTests")
}

// TestC1566_011_ExistingGateContractPreserved — anti-weakening. The repair
// touches the shared classification/retry path, so the fixed pack's own
// contract is pinned alongside it.
func TestC1566_011_ExistingGateContractPreserved(t *testing.T) {
	requirePassing(t, shipPkg,
		"TestRepoContractGate_OffSkips",
		"TestRepoContractGate_EnforceGreenPasses",
		"TestRepoContractGate_EnforceRedFailsWithDedicatedCode",
		"TestRepoContractGate_UnknownStageFailsTowardEnforce",
		"TestRepoContractGate_TransientFailureRetriesOnceThenShips",
		"TestRepoContractGate_PersistentAmbiguityIsInfraClassedExactlyTwoRuns",
	)
}
