//go:build acs

// Package cycle1015 materialises the cycle-1015 acceptance criteria for the sole
// fleet-scoped item of this lane, surface-tripwire-in-tokens-report (inbox item
// telemetry-coverage-tripwire-nonclaude-success, weight 0.93). fleet_scope pins
// this lane to that one id, so per R9.3 no predicate binds to any other lane's
// work.
//
// Where cycle-1005 landed the ENGINE side (recordTokenUsage escalates a
// non-claude exit-0 >60s source=none launch to a "tripwire":true record in
// llm-calls.ndjson), cycle-1015 is the REPORT side: `evolve tokens report` must
// SURFACE that record to the operator. The load-bearing crux is the cycle-1007
// render-order regression — renderTripwires runs UNCONDITIONALLY before the
// empty-phases early return, so a cycle with tripwire hits but no phase-timing
// rows still prints the WARN instead of silently early-returning.
//
// Predicate strategy — every predicate EXERCISES the system under test, never a
// source-grep of production code (the cycle-85 degenerate-predicate ban):
//
//   - 001–003 run the in-package behavioral tests (package main, go/cmd/evolve,
//     which drives runTokensReport end-to-end over a real temp .evolve/runs tree)
//     as a SUBPROCESS and require an explicit "--- PASS:" marker for each named
//     test. A bare exit-0 is rejected, so a renamed/skipped/deleted test cannot
//     green the gate.
//
// Adversarial axes (SKILL §6):
//   - SEMANTIC — 001 (surfaces + names CLI/agent/cycle in text AND json), 002
//     (the render-order crux: still surfaces when Phases is empty), 003 (F1
//     control-byte sanitisation) are distinct behaviors, not one restated.
//   - NEGATIVE — 004 requires the zero-tripwire cycle (claude baseline + a
//     quota-abort exit-85 short non-claude launch) to stay SILENT: an
//     always-on/no-op render that unconditionally prints TRIPWIRE fails it. This
//     is the strongest anti-no-op signal (a report that always warns "passes"
//     the surfacing tests but fails here).
//
// The in-package tests (cmd_tokens_test.go) are the RED contract for this
// behavioral surface; the subprocess makes them the cycle's audit-gating
// predicate so the surfacing contract survives beyond this cycle.
package cycle1015

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// tokensPkg is the CLI package whose in-package tests drive runTokensReport
// (the render path under test). Using the full module path keeps the subprocess
// cwd-independent within the module.
const tokensPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

// runTokensTest runs `go test -run <pattern> <tokensPkg>` as a subprocess
// (verbose, no cache) and fails unless every named test reports an explicit PASS.
// A bare exit-0 is not enough: each wanted test must show "--- PASS: <name>", so
// a renamed, skipped, or deleted test cannot silently green the gate.
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

// TestC1015_001_report_surfaces_tripwire_naming_cli_agent_cycle — AC1+AC2. A
// cycle's "tripwire":true llm-calls.ndjson record surfaces in BOTH the plain-text
// report and --json, and the surfaced line names the offending CLI, agent, and
// cycle. A sibling non-tripwire (claude) record in the same file must NOT be
// surfaced (no false positive).
func TestC1015_001_report_surfaces_tripwire_naming_cli_agent_cycle(t *testing.T) {
	runTokensTest(t,
		"^TestRunTokensReport_SurfacesTripwireInTextAndJSON$",
		"TestRunTokensReport_SurfacesTripwireInTextAndJSON")
}

// TestC1015_002_report_surfaces_tripwire_even_when_phases_empty — the cycle-1007
// render-order crux. A discovered cycle (phase-timing.json present) with ZERO
// phase entries — the exact shape that made renderTokensReport early-return
// before printing — must STILL surface the tripwire WARN naming its CLI. A
// regression that moves renderTripwires back below the empty-phases return fails
// this.
func TestC1015_002_report_surfaces_tripwire_even_when_phases_empty(t *testing.T) {
	runTokensTest(t,
		"^TestRunTokensReport_SurfacesTripwireEvenWhenPhasesEmpty$",
		"TestRunTokensReport_SurfacesTripwireEvenWhenPhasesEmpty")
}

// TestC1015_003_report_sanitizes_tripwire_control_bytes — F1 EDGE. A compromised
// non-claude driver embeds ANSI escape bytes in its record's CLI/agent/phase
// fields to rewrite or hide the tripwire line meant to expose it. The plain-text
// render must strip control bytes: no raw ESC (0x1b) reaches the TTY and the
// tripwire still surfaces.
func TestC1015_003_report_sanitizes_tripwire_control_bytes(t *testing.T) {
	runTokensTest(t,
		"^TestRunTokensReport_SanitizesTripwireControlBytes$",
		"TestRunTokensReport_SanitizesTripwireControlBytes")
}

// TestC1015_004_report_stays_quiet_when_zero_tripwires — AC3 NEGATIVE, the
// anti-no-op signal. A cycle whose launches all carry tripwire:false (a claude
// baseline plus a quota-abort exit-85 short non-claude launch — the current
// false-positive pattern) must emit NO TRIPWIRE line. An always-on render that
// unconditionally prints the section passes 001–003 but fails here.
func TestC1015_004_report_stays_quiet_when_zero_tripwires(t *testing.T) {
	runTokensTest(t,
		"^TestRunTokensReport_ZeroTripwireStaysQuiet$",
		"TestRunTokensReport_ZeroTripwireStaysQuiet")
}
