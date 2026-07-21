//go:build acs

// Package cycle1013 materialises the cycle-1013 acceptance criteria for the sole
// fleet-scoped item of this lane, surface-tripwire-in-tokens-report (inbox
// telemetry-coverage-tripwire-nonclaude-success, weight 0.93). fleet_scope pins
// this lane to that one id, so per R9.3 no predicate binds to any other lane's
// work. Scout committed one top_n task: read the engine's llm-calls.ndjson
// tripwire records inside `evolve tokens report` and surface them in the
// plain-text and --json output (the engine side, engine.go recordTokenUsage, is
// already done and unchanged; the gap is entirely cmd/evolve/cmd_tokens.go).
//
// Predicate strategy — every predicate EXERCISES the system under test, never a
// source-grep of production code (the cycle-85 degenerate-predicate ban). Each
// predicate runs the in-package behavioral RED tests (package main, which reaches
// the unexported runTokensReport/renderTokensReport entry points) as a SUBPROCESS
// and requires an explicit "--- PASS:" marker per named test. A bare exit-0 is
// rejected, so a renamed/skipped/deleted test cannot green the gate. The `-run`
// pattern is anchored to only this task's four tests, so the package's unrelated
// pre-existing red test (TestComposedApicoverGate_WarningOnlyMissesNewUnnamedExport)
// cannot block — and cannot mask — this task's own scoped run.
//
// Adversarial axes (adversarial-testing SKILL §6): NEGATIVE — 003 requires a
// cycle whose launches are all tripwire:false (claude baseline + quota-abort
// exit-85 short) to stay silent; the input must NOT trip, rejecting an always-on
// impl. EDGE — 004 requires ANSI/control bytes in the record's CLI/agent/phase
// fields to be escaped/stripped from the TTY (F1 injection defense). SEMANTIC —
// 001 (surface in text+json, naming cli/agent/cycle), 002 (survive the empty-
// phases render-order defect), 003 (silence on non-signals), 004 (sanitize) are
// distinct behaviors, not one restated.
package cycle1013

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const tokensPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

// runTokensTest runs `go test -run <pattern> <tokensPkg>` as a subprocess
// (verbose, no cache) and fails unless every named test reports an explicit PASS.
// RED today: cmd_tokens.go never reads llm-calls.ndjson and renderTokensReport
// early-returns before any tripwire text, so the in-package assertions fail and
// `go test` exits non-zero.
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

// TestC1013_001_tripwire_surfaced_in_text_and_json — AC1+AC2. A non-claude
// tripwire record in a cycle's llm-calls.ndjson surfaces in both plain-text and
// --json output, and the surfaced line names the CLI, agent, and cycle; a sibling
// non-tripwire claude record is not surfaced.
func TestC1013_001_tripwire_surfaced_in_text_and_json(t *testing.T) {
	runTokensTest(t,
		"^TestRunTokensReport_SurfacesTripwireInTextAndJSON$",
		"TestRunTokensReport_SurfacesTripwireInTextAndJSON")
}

// TestC1013_002_tripwire_survives_empty_phases — the cycle-1007 render-order
// regression. A cycle with zero phase entries but a real tripwire must still
// surface the tripwire (renderTokensReport must not early-return past it).
func TestC1013_002_tripwire_survives_empty_phases(t *testing.T) {
	runTokensTest(t,
		"^TestRunTokensReport_SurfacesTripwireEvenWhenPhasesEmpty$",
		"TestRunTokensReport_SurfacesTripwireEvenWhenPhasesEmpty")
}

// TestC1013_003_zero_tripwire_stays_quiet — AC3 NEGATIVE. A cycle whose launches
// are all tripwire:false (claude baseline + quota-abort exit-85 short non-claude)
// must emit no TRIPWIRE line — no false positives on the current quota-abort
// pattern.
func TestC1013_003_zero_tripwire_stays_quiet(t *testing.T) {
	runTokensTest(t,
		"^TestRunTokensReport_ZeroTripwireStaysQuiet$",
		"TestRunTokensReport_ZeroTripwireStaysQuiet")
}

// TestC1013_004_tripwire_control_bytes_sanitized — F1 EDGE (carried from the
// cycle-1010 audit). ANSI escape bytes embedded in a tripwire record's
// CLI/agent/phase fields must be escaped/stripped in the plain-text path: no raw
// ESC (0x1b) reaches the TTY, and the tripwire still surfaces.
func TestC1013_004_tripwire_control_bytes_sanitized(t *testing.T) {
	runTokensTest(t,
		"^TestRunTokensReport_SanitizesTripwireControlBytes$",
		"TestRunTokensReport_SanitizesTripwireControlBytes")
}
