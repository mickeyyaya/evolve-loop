package main

import (
	"os"
	"testing"
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
// It also disables the pre-batch readiness gate (EVOLVE_SKIP_PREFLIGHT=1)
// for the same reason: the loop-mechanics tests (budget, circuit breaker,
// checkpoint, quota) drive runLoop with a faked orchestrator in a temp
// project that has no profiles dir, so the real readiness probe would halt
// before the mechanics under test run. The gate has its own coverage in
// cmd_loop_preflight_test.go, which re-enables it per-test via t.Setenv.
func TestMain(m *testing.M) {
	if err := os.Setenv("EVOLVE_DISABLE_WORKSPACE_GUARD", "1"); err != nil {
		panic("TestMain setenv: " + err.Error())
	}
	if err := os.Setenv("EVOLVE_SKIP_PREFLIGHT", "1"); err != nil {
		panic("TestMain setenv: " + err.Error())
	}
	os.Exit(m.Run())
}
