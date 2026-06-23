package main

import (
	"io"
	"os"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/looppreflight"
)

// TestMain disables the workspace-pollution guard for every test in this
// package. Many cmd/evolve tests pre-seed workspace files in temp dirs to
// simulate phase state (M4/M5 dispatch validators, ledger writers,
// cycle-state tests, etc.). The orchestrator's workspace-guard (added in
// v12.2.2 for bug #4) would archive those pre-seeded files away, breaking
// the tests. Production keeps the guard on by default (disableWorkspaceGuardForTest=false);
// tests opt out via the package-level seam (cycle-10: replaced the retired
// EVOLVE_DISABLE_WORKSPACE_GUARD env signal with a DI bool).
//
// Individual tests that specifically EXERCISE the guard should set
// disableWorkspaceGuardForTest=false and restore it with t.Cleanup.
//
// It also installs a pass-through preflight seam so loop-mechanics tests
// (budget, circuit breaker, checkpoint, quota) can drive runLoop with a
// faked orchestrator in a temp project that has no real environment. The
// gate has its own coverage in cmd_loop_preflight_test.go, which overrides
// the seam per-test via the runLoopPreflightFn package var.
func TestMain(m *testing.M) {
	disableWorkspaceGuardForTest = true
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result {
		return looppreflight.Result{}
	}
	os.Exit(m.Run())
}
