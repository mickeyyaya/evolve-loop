//go:build acs

// Package cycle1014 materialises the cycle-1014 acceptance criteria for the sole
// fleet-scoped item of this lane, tripwire-regression-lock (inbox
// telemetry-coverage-tripwire-nonclaude-success, weight 0.93). fleet_scope pins
// this lane to that one id, so per R9.3 no predicate binds to any other lane's
// work. Triage committed exactly one top_n task: add an explicitly-AC-named
// positive+negative report-level regression that locks in the already-landed
// telemetry-coverage tripwire (engine.go recordTokenUsage computes it; cycle-1013
// surfaced it in `evolve tokens report`). This cycle adds no production code —
// it consolidates the AC1/AC2/AC3 contract into a single named regression so a
// future hostile edit to the render path (the exact cycle-1007 render-order
// defect) is caught by a test whose name states the contract.
//
// Predicate strategy — every predicate EXERCISES the system under test, never a
// source-grep of production code (the cycle-85 degenerate-predicate ban). Each
// predicate runs the Builder's in-package behavioral RED tests (package main,
// which reaches the unexported runTokensReport/renderTokensReport entry points)
// as a SUBPROCESS and requires an explicit "--- PASS:" marker per named test. A
// bare exit-0 is rejected, so a renamed/skipped/deleted/never-written test cannot
// green the gate (this is what makes RED real today: neither Builder test exists
// yet, so no PASS marker is emitted). The `-run` pattern is anchored to only this
// task's two tests, so the package's unrelated pre-existing red test
// (TestComposedApicoverGate_WarningOnlyMissesNewUnnamedExport) cannot block — and
// cannot mask — this task's own scoped run.
//
// Adversarial axes (adversarial-testing SKILL §6): POSITIVE/SEMANTIC — 001 requires
// the fire case (non-claude cli, exit 0, >60s, source=none) to surface a TRIPWIRE
// line that names the CLI, agent, AND cycle together (AC1+AC2). NEGATIVE — 002
// requires a matrix of three distinct false-positive vectors (a claude-baseline
// launch, a sub-threshold-duration launch, and a quota-abort exit-85 launch) to
// stay silent (AC3), rejecting an always-on impl. The two named tests are distinct
// behaviors, not one restated.
package cycle1014

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const tokensPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

// runTokensTest runs `go test -run <pattern> <tokensPkg>` as a subprocess
// (verbose, no cache) and fails unless every named test reports an explicit PASS.
// RED today: the Builder has not yet added TestTokensReport_TripwireFiresOnNonClaudeSuccess
// or TestTokensReport_TripwireSilentOnClaudeShortAndAbort, so `-run` matches no
// test, no "--- PASS:" marker is printed, and the wantPass assertion fails. A bare
// exit-0 ("no tests to run") is therefore rejected — the gate requires the test to
// actually execute and pass.
func runTokensTest(t *testing.T, pattern string, wantPass ...string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", pattern, tokensPkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, tokensPkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("%s did not report PASS (renamed, skipped, or not run):\n%s", name, stdout)
		}
	}
}

// TestC1014_001_tripwire_fires_and_names_cli_agent_cycle — AC1+AC2. A single
// non-claude launch that exited 0, ran past the 60s success threshold, and
// resolved to source=none must surface a TRIPWIRE line, and that line must name
// the offending CLI, agent, and cycle together (not just one of them). Delegates
// to the Builder's positive in-package test.
func TestC1014_001_tripwire_fires_and_names_cli_agent_cycle(t *testing.T) {
	runTokensTest(t,
		"^TestTokensReport_TripwireFiresOnNonClaudeSuccess$",
		"TestTokensReport_TripwireFiresOnNonClaudeSuccess")
}

// TestC1014_002_tripwire_silent_on_claude_short_and_abort — AC3 NEGATIVE. Three
// distinct false-positive vectors in one cycle — a claude-baseline launch (out of
// scope), a non-claude success under the duration threshold, and a non-claude
// quota-abort (exit 85) — must all stay silent: no TRIPWIRE line. Rejects an
// always-on implementation. Delegates to the Builder's negative in-package test.
func TestC1014_002_tripwire_silent_on_claude_short_and_abort(t *testing.T) {
	runTokensTest(t,
		"^TestTokensReport_TripwireSilentOnClaudeShortAndAbort$",
		"TestTokensReport_TripwireSilentOnClaudeShortAndAbort")
}
