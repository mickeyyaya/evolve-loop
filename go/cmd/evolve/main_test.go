package main

import (
	"io"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
)

// TestMain sets the workspace-pollution guard's opt-out env var for
// every test in this package. Many cmd/evolve tests pre-seed workspace
// files in temp dirs to simulate phase state (M4/M5 dispatch validators,
// ledger writers, cycle-state tests, etc.). The orchestrator's
// workspace-guard (added in v12.2.2 for bug #4) would archive those
// pre-seeded files away, breaking the tests. Production keeps the guard
// on by default; tests opt out via EVOLVE_DISABLE_WORKSPACE_GUARD=1.
//
// Individual tests that specifically EXERCISE the guard (e.g. a future
// integration test that asserts workspace-archival fires for the
// cycle-108 scenario) should re-enable it with t.Setenv.
//
// It also installs a pass-through preflight seam so loop-mechanics tests
// (budget, circuit breaker, checkpoint, quota) can drive runLoop with a
// faked orchestrator in a temp project that has no real environment. The
// gate has its own coverage in cmd_loop_preflight_test.go, which overrides
// the seam per-test via the runLoopPreflightFn package var.
func TestMain(m *testing.M) {
	if err := os.Setenv("EVOLVE_DISABLE_WORKSPACE_GUARD", "1"); err != nil {
		panic("TestMain setenv: " + err.Error())
	}
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result {
		return looppreflight.Result{}
	}
	os.Exit(m.Run())
}
