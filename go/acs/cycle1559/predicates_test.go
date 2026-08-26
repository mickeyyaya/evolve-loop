//go:build acs

// Package cycle1559 continues the red-first-deliverable-reds-main lane
// (live inbox record; same inbox item as cycle-1555's `ship-added-test-*`
// tasks). Cycle-1555 authored the crux detection (T1 AC1/AC2) and the
// production-caller wiring proof plus the skip convention (T2 AC1/AC2); they
// are RED in the tree today (the green implementation never landed) and stay
// pinned here as pre-existing coverage — this cycle does not re-author them.
//
// The one remaining materialized gap in this lane's fleet-scoped task
// (`ship-added-red-test-guard`, triage top_n) is AC3: a `require-tmux`-style,
// environment-exclusive added test must never be silently claimed green (a
// lone-file package excluded by its own `//go:build` constraint would
// otherwise either false-RED as a build failure or vanish with no record) —
// the gate must instead leave an explicit, durable backstop record naming
// the file and its exclusion reason. AC4 (a transient scanner failure stays
// distinct from a genuine RED with the existing one-retry behavior) already
// has structural coverage: the added-test candidates flow through the SAME
// `runRepoContractGate` retry/classification path the pre-existing
// `TestRepoContractGate_TransientFailureRetriesOnceThenShips` /
// `TestRepoContractGate_PersistentAmbiguityIsInfraClassedExactlyTwoRuns`
// pin — no scope-specific retry logic to test separately.
//
// Predicate strategy — behavioral, never source-grep (cycle-85 ban): each
// predicate DRIVES the system by running the named Go unit test in ONE
// named package with `-run` narrowing and requiring its `--- PASS:` line
// (per the flaky-predicate-shape rules — no `./...` sweeps, no wall-clock
// bounds, cmd.Dir always set explicitly). The PASS-line assertion is the
// anti-vacuous guard: a non-matching `-run` pattern exits 0 with "no tests
// to run", so exit-code-only checking would pass on an EMPTY repo.
//
// Predicate map:
//
//	001 — pre-existing: added failing test blocks ship (T1 AC1)
//	002 — pre-existing: selection bounded to newly-added test files (T1 AC2)
//	003 — pre-existing: the fixed four-package gate behaviours are unweakened
//	004 — pre-existing WIRING PROOF: Phase.runNative stops before git/ship (T2 AC1)
//	005 — pre-existing: a t.Skip-ped newly added test does not block (T2 AC2)
//	006 — NEW this cycle: an env-exclusive (`requires_tmux`) added test gets
//	      an explicit backstop record, never a silent false green (T1 AC3)
package cycle1559

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
// load-bearing: exit 0 alone is satisfied by "no tests to run" (a missing
// test would otherwise read as success), and a PASS line alone cannot appear
// if the package fails to build.
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

// TestC1559_001_AddedFailingTestBlocksShip — T1 AC1 (pre-existing, cycle-1555).
func TestC1559_001_AddedFailingTestBlocksShip(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_NewlyAddedFailingTestBlocksShip")
}

// TestC1559_002_SelectionBoundedToNewlyAddedTestFiles — T1 AC2 (pre-existing).
func TestC1559_002_SelectionBoundedToNewlyAddedTestFiles(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_AddedTestSelectionIgnoresModifiedAndNonTestFiles")
}

// TestC1559_003_ExistingGateContractPreserved — anti-weakening (pre-existing).
func TestC1559_003_ExistingGateContractPreserved(t *testing.T) {
	requirePassing(t, shipPkg,
		"TestRepoContractGate_OffSkips",
		"TestRepoContractGate_EnforceGreenPasses",
		"TestRepoContractGate_EnforceRedFailsWithDedicatedCode",
		"TestRepoContractGate_UnknownStageFailsTowardEnforce",
	)
}

// TestC1559_004_ProductionCallerStopsBeforeShip — T2 AC1, WIRING PROOF
// (pre-existing).
func TestC1559_004_ProductionCallerStopsBeforeShip(t *testing.T) {
	requirePassing(t, shipPkg, "TestPhaseRunNative_NewlyAddedFailingTestPreventsRun")
}

// TestC1559_005_SkippedReproducerDoesNotBlock — T2 AC2, negative case
// (pre-existing).
func TestC1559_005_SkippedReproducerDoesNotBlock(t *testing.T) {
	requirePassing(t, shipPkg, "TestRunNative_AddedSkippedTestDoesNotBlockShip")
}

// TestC1559_006_EnvExclusiveCandidateGetsExplicitBackstop — T1 AC3, the
// cycle-1559 addition. A `requires_tmux`-tagged added test must not block
// the ship (it is honestly unrunnable here) AND must not be silently
// dropped — the scan log must explicitly name it and its exclusion reason.
func TestC1559_006_EnvExclusiveCandidateGetsExplicitBackstop(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_AddedEnvExclusiveTestBackstopRecorded")
}
