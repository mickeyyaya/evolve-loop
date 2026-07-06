//go:build acs

// Package cycle566 materialises the cycle-566 acceptance criteria for the
// fleet-lane-committed top_n task `per-phase-effort-routing` (inbox weight 0.88;
// see triage-report.md and .evolve/inbox/2026-07-05T15-10-00Z-per-phase-effort-
// routing.json).
//
// Committed scope (triage top_n): the PLUMBING slice only — add an abstract
// `effort` (low|medium|high) dimension to LaunchIntent, realize it per-manifest to
// each CLI's native mechanism (claude effort flag, codex reasoning_effort;
// agy/ollama noop), and pin per-phase defaults in config. Retry-escalation and
// telemetry/soak validation are EXPLICITLY deferred to a follow-up cycle.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549/553/555/557/561/
// 563/565 precedent). Each predicate shells `go test`/`go vet` over the RED unit
// tests authored this cycle in go/internal/bridge (effort_routing_test.go) and
// go/internal/profiles (effort_defaults_test.go). RED now — the bridge tests do
// not compile until Builder adds LaunchIntent.Effort + realizeScalar("effort",…) +
// the manifest params.effort entries, and the profiles matrix asserts values the
// shipped config does not yet carry. GREEN once effort is wired and the per-phase
// defaults are aligned.
package cycle566

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	bridgePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	profilesPkg = "github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// runGoTest shells `go test -run '^<pattern>$' -count=1 <pkg>` and returns whether
// it exited cleanly plus the combined output for diagnostics. -count=1 defeats the
// test cache so the predicate always exercises current source. A compile failure
// in the target package (e.g. undefined LaunchIntent.Effort) surfaces as a
// non-zero exit — the intended RED signal before Builder implements.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	// code < 0 is a genuine launch failure (binary missing / killed by signal),
	// not a test verdict; SubprocessOutput returns a non-nil err for ANY non-zero
	// exit, so a plain compile/assertion failure (code 1/2 — the RED signal) must
	// flow through as ok=false, NOT be misread as "failed to launch".
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC566_001_EffortManifestParityPerCLI — AC-A: every embedded tmux manifest
// maps the abstract effort onto its CLI's native mechanism (claude/codex
// translate through an effective channel) or cleanly no-ops (agy/ollama). Drives
// TestEffortRealize_Matrix in go/internal/bridge.
func TestC566_001_EffortManifestParityPerCLI(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestEffortRealize_Matrix")
	if !ok {
		t.Errorf("effort not realized per-manifest (claude/codex must translate, agy/ollama must noop):\n%s", out)
	}
}

// TestC566_002_EffortDefaultMatrix — AC-B: the per-phase effort defaults
// (scout/triage=low, tdd/audit/adversarial=medium, builder=medium) are pinned in
// the shipped .evolve/profiles config, read via the loader. Drives
// TestEffortDefaults_Matrix in go/internal/profiles.
func TestC566_002_EffortDefaultMatrix(t *testing.T) {
	ok, out := runGoTest(t, profilesPkg, "TestEffortDefaults_Matrix")
	if !ok {
		t.Errorf("per-phase effort default matrix not pinned in config (scout/triage=low, tdd/audit/adversarial=medium, builder=medium):\n%s", out)
	}
}

// TestC566_003_EffortAbsentByteIdentical — AC-C regression: with effort unset the
// realization is byte-identical to a pre-effort manifest — the dimension is purely
// additive. Drives TestEffortRealize_AbsentByteIdentical in go/internal/bridge.
func TestC566_003_EffortAbsentByteIdentical(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestEffortRealize_AbsentByteIdentical")
	if !ok {
		t.Errorf("unset effort is not byte-identical to a pre-effort manifest (additive-only regression guard):\n%s", out)
	}
}

// TestC566_004_TouchedPackagesVetClean — AC-D: the packages this feature touches
// (bridge, profiles) type-check and vet cleanly once effort is wired. RED now —
// the new bridge test references the not-yet-defined LaunchIntent.Effort, so vet
// fails to type-check the package. GREEN once the field + realizer wiring land.
func TestC566_004_TouchedPackagesVetClean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", bridgePkg, profilesPkg)
	if code < 0 {
		t.Fatalf("go vet failed to launch: code=%d err=%v\nstderr:\n%s", code, err, stderr)
	}
	if code != 0 {
		t.Errorf("go vet not clean on touched packages (bridge, profiles):\n%s\n%s", stdout, stderr)
	}
}
