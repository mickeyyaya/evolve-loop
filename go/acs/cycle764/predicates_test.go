//go:build acs

// Package cycle764 materializes the cycle-764 acceptance criteria for the sole
// task committed by triage-report.md "## top_n":
// ship-manual-deletes-running-binary (inbox
// 2026-07-13T08-45-00Z-ship-manual-deletes-running-binary.json, weight 0.94).
//
// Defect: `evolve ship --class manual` routes through shipDirect →
// discardBinaryChurn on the SUCCESS path; cmd_ship.go never sets
// Options.ShipBinaryPath, so the discard falls back to os.Executable() — the
// running, UNTRACKED (gitignored) go/bin/evolve — and os.Remove()s it (live
// incidents 2026-07-12; cycle-243 precedent: kernel hooks silently degrade).
// Rollback shells `ship --class manual` (rollback.defaultRevertAndShip), so
// the same shipDirect guard covers that path transitively.
//
// AC map (1:1), from the inbox item's acceptance list:
//
//	AC1 discard never removes the running executable (skip+WARN) → C764_001
//	AC2 manual-ship success leaves untracked go/bin/evolve       → C764_002
//	AC3 rollback shellout path covered (same shipDirect entry)   → C764_002 (see doc)
//	AC4 ship+rollback suites -race PASS (no regression)          → C764_003
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit contract in internal/phases/ship, which EXERCISES discardBinaryChurn /
// shipDirect against real Options — behavioral via subprocess, no source-grep
// predicates (cycle-85 rule). The `-v` + "--- PASS:" guard rejects a rename /
// no-tests-matched silent green.
package cycle764

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	shipPkg     = "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	rollbackPkg = "github.com/mickeyyaya/evolve-loop/go/internal/rollback"
)

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

// AC1 — discardBinaryChurn with ShipBinaryPath="" (the exact CLI wiring) must
// skip the currently-executing binary instead of os.Remove()ing it, and WARN.
func TestC764_001_DiscardNeverRemovesRunningExecutable(t *testing.T) {
	runGoTest(t, shipPkg, "TestDiscardBinaryChurn_NeverRemovesRunningExecutable")
}

// AC2 + AC3 — a successful manual-class shipDirect (the entrypoint the
// rollback shellout `evolve ship --class manual` re-enters; rollback has no
// independent deletion logic) leaves the untracked running go/bin/evolve on
// disk, while STILL discarding untracked go/evolve churn (guard is narrow —
// an over-broad revert of the churn discard fails this predicate).
func TestC764_002_ManualShipLeavesRunningBinaryDiscardsChurn(t *testing.T) {
	runGoTest(t, shipPkg, "TestManualShipSuccess_LeavesUntrackedGoBinEvolvePresent")
}

// AC4 (regression) — the full ship + rollback suites (audit-binding, release
// staging discriminators, rollback pipeline) still pass with the guard in
// place, under -race.
func TestC764_003_ShipAndRollbackSuitesNoRegression(t *testing.T) {
	for _, pkg := range []string{shipPkg, rollbackPkg} {
		stdout, stderr, code, err := acsassert.SubprocessOutput(
			"go", "test", "-race", "-count=1", pkg)
		if code != 0 || err != nil {
			t.Fatalf("full suite failed for %s (exit %d, err=%v)\nstdout:\n%s\nstderr:\n%s",
				pkg, code, err, stdout, stderr)
		}
	}
}
