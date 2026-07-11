//go:build acs

// Package cycle688 materializes the cycle-688 acceptance criteria for the sole
// committed top_n task observer-sink-close-race (triage-report.md ## top_n;
// the scout's chronicle-s2-digest-writer pick was DEFERRED by triage as
// out-of-fleet-scope for this lane — R9.3: predicates bind ONLY to committed
// work, so no chronicle predicates appear here).
//
// AC map (1:1):
//
//	AC1 timeout arm never closes the sink (use-after-close race closed) → C688_001
//	AC2 <-done arm still closes exactly once (regression twin)          → C688_002
//	AC3 nil-closer contract preserved (Start's `if sinkCloser != nil`)  → C688_003
//	AC4 package green under -race + go vet clean                        → C688_004/005
//
// NOTE (pre-existing GREEN): the fix and its unit contract already landed on
// main via the cycle-669 commit 1d0e23ac (closeSinkAfterWait closes only on
// the <-done arm; WARN text documents the deliberate fd leak). These
// predicates therefore pin the behavior against regression rather than gate a
// fresh implementation — documented in test-report.md per the RED
// verification rules ("unexpected pass → pre-existing GREEN").
//
// Each predicate shells `go test -race -run '^<name>$' -v` over the unit-test
// contract, which EXERCISES the SUT (closeSinkAfterWait with real channels,
// timeouts, and a counting io.Closer) — behavioral via subprocess, no
// source-grep predicates (cycle-85 rule). The `-v` + "--- PASS:" guard
// rejects a rename/no-tests-matched silent green.
package cycle688

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const observerPkg = "github.com/mickeyyaya/evolve-loop/go/internal/adapters/observer"

// runGoTest executes the named unit test under -race and requires an explicit
// verbose PASS marker so the predicate fails on: compile failure, test
// failure, a race report, OR the test not existing (rename gaming).
func runGoTest(t *testing.T, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", observerPkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			observerPkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test reported no PASS for %s (renamed or not run?)\nstdout:\n%s", name, stdout)
	}
}

// AC1 — the negative test: a wedged watcher (done never fires) must NOT have
// its sink closed on the timeout arm.
func TestC688_001_TimeoutArmNeverClosesSink(t *testing.T) {
	runGoTest(t, "TestCoreAdapter_NoSinkCloseRaceOnTimeout")
}

// AC2 — the regression twin: the normal <-done path still closes exactly once
// (rejects a degenerate "never close" implementation).
func TestC688_002_DoneArmClosesExactlyOnce(t *testing.T) {
	runGoTest(t, "TestCoreAdapter_SinkClosedOnNormalDone")
}

// AC3 — nil-closer safety (Start's caller-supplied-writer contract).
func TestC688_003_NilCloserSafe(t *testing.T) {
	runGoTest(t, "TestCoreAdapter_CloseSinkAfterWait_NilCloserSafe")
}

// AC4a — the whole observer package is green under the race detector.
func TestC688_004_ObserverPackageRaceClean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", observerPkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			observerPkg, code, err, stdout, stderr)
	}
}

// AC4b — go vet clean on the observer package.
func TestC688_005_ObserverPackageVetClean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", observerPkg)
	if code != 0 || err != nil {
		t.Fatalf("go vet %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			observerPkg, code, err, stdout, stderr)
	}
}
