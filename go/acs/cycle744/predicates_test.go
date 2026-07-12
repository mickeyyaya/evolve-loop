//go:build acs

// Package cycle744 materializes the cycle-744 acceptance criteria for the sole
// committed top_n task fleet-config-hot-reload-wave-boundary (triage-report.md
// ## top_n; scout's two selections were deferred by triage, so per R9.3 no
// predicates bind to them and no deferred-floor predicates exist).
//
// DUPLICATE-TASK NOTICE (recorded in test-report.md): the committed carryover
// item duplicates work ALREADY SHIPPED in cycle 739 — the seam
// reloadFleetConfigAtWaveBoundary (go/cmd/evolve/cmd_loop_wave.go), its
// batch-loop wiring (cmd_loop.go: reload before budgetAwareWaveConfig every
// iteration), the unit-test contract (cmd_loop_wave_reload_test.go), and the
// cycle-739 ACS suite all exist in this worktree's base. These predicates are
// therefore expected pre-existing GREEN: they re-pin the committed AC set for
// THIS cycle's audit gate (ACS suites are cycle-scoped) rather than encode new
// RED work for Builder.
//
// AC map (1:1), derived from the top_n task text ("reload fleet policy config
// at each wave boundary … so a mid-batch policy.json commit takes effect at
// the next wave without killing in-flight lanes"):
//
//	AC1 min_lanes committed mid-batch takes effect next wave      → C744_001
//	AC2 count committed mid-batch takes effect next wave
//	    (widen AND narrow-to-sequential edge)                     → C744_002
//	AC3 unchanged policy ⇒ identical config + zero reload noise   → C744_003
//	AC4 malformed policy at boundary holds width + WARNs (neg.)   → C744_004
//	AC5 batch loop invokes the reload seam before quota/budget
//	    sizing at every iteration                                 → manual+checklist (auditor)
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit-test contract in cmd/evolve, which EXERCISES the SUT (the wave-boundary
// reload seam against real temp policy.json documents) — behavioral via
// subprocess, no source-grep predicates (cycle-85 rule). The `-v` +
// "--- PASS:" guard rejects a rename/no-tests-matched silent green.
package cycle744

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const evolveCmdPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

// runGoTest executes the named unit test under -race and requires an explicit
// verbose PASS marker so the predicate fails on: compile failure, test
// failure, a race report, OR the test not existing (rename gaming).
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

// AC1 — the incident twin: min_lanes committed between waves is resolved at
// the next wave boundary, logged, and the new floor holds width under a
// quota bench.
func TestC744_001_ReloadsMinLanesAtWaveBoundary(t *testing.T) {
	runGoTest(t, evolveCmdPkg, "TestFleetDispatch_ReloadsMinLanesAtWaveBoundary")
}

// AC2 — count changes (widen 3->5, narrow ->1) take effect at the next wave
// boundary without a supervisor bounce; narrowing exits the wave path
// (edge: sequential gate) without killing in-flight lanes.
func TestC744_002_ReloadsCountAtWaveBoundary(t *testing.T) {
	runGoTest(t, evolveCmdPkg, "TestFleetDispatch_ReloadsCountAtWaveBoundary")
}

// AC3 — the regression pin: unchanged policy.json resolves a deep-equal
// config and emits ZERO reload log noise on the steady-state path.
func TestC744_003_UnchangedPolicyByteIdenticalDispatch(t *testing.T) {
	runGoTest(t, evolveCmdPkg, "TestFleetDispatch_UnchangedPolicyByteIdenticalDispatch")
}

// AC4 — the negative: a malformed policy.json at a wave boundary holds the
// previous width (never a silent Count=1 collapse) and WARNs.
func TestC744_004_MalformedPolicyHoldsWidth(t *testing.T) {
	runGoTest(t, evolveCmdPkg, "TestFleetDispatch_MalformedPolicyAtWaveBoundaryHoldsWidth")
}
