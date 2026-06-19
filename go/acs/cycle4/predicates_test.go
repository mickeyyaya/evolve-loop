//go:build acs

// Package cycle4 materializes the cycle-4 acceptance criteria for:
//
//   - fleet-soak-invariants: Slice 5 of the concurrency-arch-slices campaign.
//     cmd/evolve/cmd_fleet_soak.go implements `evolve fleet soak --count N`,
//     an in-process soak harness (no LLM, no tmux) that proves the four
//     concurrency invariants from the sibling-worktree architecture under -race.
package cycle4

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// --- Task: fleet-soak-invariants ---

// TestC4_001_FleetSoakFileExistsAndTracked asserts that
// go/cmd/evolve/cmd_fleet_soak.go was created in the worktree and is
// git-tracked. A gitignored file is silently dropped at ship (cycle-93 lesson).
func TestC4_001_FleetSoakFileExistsAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "cmd", "evolve", "cmd_fleet_soak.go")
	path := filepath.Join(root, rel)
	if !acsassert.FileExists(t, path) {
		t.Fatalf("RED: %s missing on disk — Builder must create cmd_fleet_soak.go", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s not git-tracked — may be gitignored and dropped at ship", rel)
	}
}

// TestC4_002_FleetSoakBuilds asserts that `go build ./cmd/evolve/` exits 0.
// This confirms runFleetSoak is defined and the fleet soak dispatch is wired
// (cmd_fleet.go must delegate 'soak' args to runFleetSoak).
func TestC4_002_FleetSoakBuilds(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "build",
		"-C", goDir,
		"./cmd/evolve/...",
	)
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("RED: go build ./cmd/evolve/ failed:\n%s", combined)
	}
}

// TestC4_003_AllFourInvariantsTestPasses runs TestFleetSoak_AllFourInvariants
// under -race -tags integration and asserts: (a) the test executed (anti-no-op
// guard), (b) exit 0, (c) no DATA RACE.
//
// This single predicate covers AC2 (test exists and passes), AC3 (distinct
// branches), AC4 (reaped sessions), AC5 (no cross-run reap), and AC6 (no torn
// config) — all four invariants are verified within that one test.
func TestC4_003_AllFourInvariantsTestPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-v", "-count=1",
		"-tags", "integration",
		"-run", "TestFleetSoak_AllFourInvariants",
		"./cmd/evolve/...",
	)
	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "TestFleetSoak_AllFourInvariants") {
		t.Fatalf("RED: TestFleetSoak_AllFourInvariants did not execute — function missing or build failed:\n%s", combined)
	}
	if strings.Contains(combined, "DATA RACE") {
		t.Errorf("RED: DATA RACE detected in TestFleetSoak_AllFourInvariants:\n%s", combined)
	}
	if code != 0 {
		t.Fatalf("RED: TestFleetSoak_AllFourInvariants failed (exit %d):\n%s", code, combined)
	}
}

// TestC4_004_RejectsZeroCountTestPasses runs TestFleetSoakArgs_RejectsZeroCount
// under -race -tags integration and asserts it passes. This is the AC7
// predicate: --count 0 must be rejected with exit 1.
func TestC4_004_RejectsZeroCountTestPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-v", "-count=1",
		"-tags", "integration",
		"-run", "TestFleetSoakArgs_RejectsZeroCount",
		"./cmd/evolve/...",
	)
	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "TestFleetSoakArgs_RejectsZeroCount") {
		t.Fatalf("RED: TestFleetSoakArgs_RejectsZeroCount did not execute:\n%s", combined)
	}
	if code != 0 {
		t.Fatalf("RED: TestFleetSoakArgs_RejectsZeroCount failed (exit %d):\n%s", code, combined)
	}
}

// TestC4_005_SoakReportUsedInCmdFleetSoak verifies that cmd_fleet_soak.go
// imports or references the soakreport package (AC8: the verdict table must
// be rendered via soakreport.RenderTable to stdout).
//
// acs-predicate: config-check — source-wiring assertion is inherently a
// file-presence check; the behavioral side is covered by TestC4_003 (the
// integration test asserts stdout contains 4 PASS rows).
func TestC4_005_SoakReportUsedInCmdFleetSoak(t *testing.T) {
	root := acsassert.RepoRoot(t)
	soakPath := filepath.Join(root, "go", "cmd", "evolve", "cmd_fleet_soak.go")
	// acs-predicate: config-check
	if !acsassert.FileContains(t, soakPath, "soakreport") {
		t.Errorf("RED: cmd_fleet_soak.go does not reference soakreport — verdict table must use soakreport")
	}
}

// TestC4_006_ExistingRegressionTestsPass runs the three existing tests that AC9
// requires be unbroken: TestSimulatePhases_CoversCanonicalOrder (fleet simulate
// harness), TestCycleRunArgs_Simulate (fleet arg threading), and TestRunFleet_*
// (fleet flag validation). Runs without -tags integration to exclude the new
// soak test file, matching the pre-Slice-5 test surface.
func TestC4_006_ExistingRegressionTestsPass(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-v", "-count=1",
		"-run", "TestSimulatePhases_CoversCanonicalOrder|TestCycleRunArgs_Simulate|TestRunFleet_",
		"./cmd/evolve/...",
	)
	combined := stdout + "\n" + stderr
	if strings.Contains(combined, "DATA RACE") {
		t.Errorf("RED: DATA RACE in regression tests:\n%s", combined)
	}
	if code != 0 {
		t.Fatalf("RED: regression tests failed (exit %d):\n%s", code, combined)
	}
}
