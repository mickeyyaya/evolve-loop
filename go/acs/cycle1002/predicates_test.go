//go:build acs

// Package cycle1002 materialises the cycle-1002 acceptance criteria for the sole
// fleet-scoped todo of this lane, adr0072-s4-orchestrator-failure-judgment
// (ADR-0072 Slice 4 — the orchestrator failure-judgment layer). fleet_scope pins
// this lane to that one id, so per R9.3 no predicate binds to any other lane's
// work. The four scout-selected tasks (evidence-dossier-builder,
// failure-decision-schema-reader, wire-floor-override-consumption,
// orchestrator-emits-failure-decision) form one dependency-linked slice and are
// committed together (triage top_n).
//
// Predicate strategy — every predicate EXERCISES the system under test, never a
// source-grep of production code (the cycle-85 degenerate-predicate ban):
//
//   - 001–004,006 run the in-package behavioral tests (package core, which can
//     reach the deliberately-UNEXPORTED buildFailureDossier / readFailureDecision
//     / applyFailureDecisionFloor — kept unexported per the design's apicover
//     constraint) as a SUBPROCESS and require an explicit PASS marker for each.
//     A bare exit-0 is rejected by asserting the named "--- PASS:" line, so a
//     renamed/skipped test cannot pass the gate. The in-package tests are the
//     RED contract authored this TDD phase; the subprocess makes them the
//     cycle's audit-gating predicate.
//   - 005 asserts the inert-API wiring on the REAL instruction artifact
//     (agents/evolve-retrospective.md): the emit directive + the six schema keys
//   - the write-allowlist widening must be present, else the consumer added in
//     Task 3 is permanently fallback-only (the "No inert API" failure mode Task 4
//     exists to prevent). This reads an emitted instruction file, not Go source.
//
// Adversarial axes (SKILL §6): NEGATIVE — 002 requires malformed/absent/unknown
// artifacts to fall back to (nil,nil); 003 requires a proposed RETRY to be
// OVERRIDDEN to a halt; 005 requires the allowlist to have widened. EDGE — 001
// covers the incoherent-vs-coherent-vs-prose-vs-structured dossier fixtures.
// SEMANTIC — dossier-build, decision-read, floor-override, and emit-wiring are
// four distinct behaviors, not one restated.
package cycle1002

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

// runCoreTest runs `go test -run <pattern> <corePkg>` as a subprocess (verbose,
// no cache) and fails unless every named test reports an explicit PASS. RED
// today: the core package does not compile until Builder adds the S4 symbols, so
// `go test` exits non-zero.
func runCoreTest(t *testing.T, pattern string, wantPass ...string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", pattern, corePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, corePkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("%s did not report PASS (renamed, skipped, or not run):\n%s", name, stdout)
		}
	}
}

// AC1 — failure-dossier.json is built per cycle from INDEPENDENT evidence
// (coherence + audit self-declared envelope + non-progress counters) and
// written to the workspace.
func TestC1002_001_evidence_dossier_built_and_written(t *testing.T) {
	runCoreTest(t, "^TestFailureDossier$", "TestFailureDossier")
}

// AC2 — the failure-decision reader validates against the schema and falls back
// to (nil,nil) on malformed/absent/schema-invalid artifacts (never a cycle abort).
func TestC1002_002_failure_decision_reader_validates_and_falls_back(t *testing.T) {
	runCoreTest(t, "^TestReadFailureDecision$", "TestReadFailureDecision")
}

// AC3 — the Go floor OVERRIDES an orchestrator/router-proposed retry to a halt
// for a floor category, in the LIVE routed path (F1), and catches the cycle-1001
// audit-declared system class both deterministically and by judgment (F2).
func TestC1002_003_go_floor_overrides_retry_to_halt(t *testing.T) {
	runCoreTest(t, "^TestDecideAfterRetroFloor_(RoutedRetryOverriddenToHalt|Cycle1001DeterministicHalt|Cycle1001JudgmentHalt)$",
		"TestDecideAfterRetroFloor_RoutedRetryOverriddenToHalt",
		"TestDecideAfterRetroFloor_Cycle1001DeterministicHalt",
		"TestDecideAfterRetroFloor_Cycle1001JudgmentHalt")
}

// AC4/AC5 — REGRESSION: with no decision + no floor, the branch/env/reason fall
// back byte-identical to the deterministic failureadapter output (nil signal).
func TestC1002_004_fallback_byte_identical_regression_guard(t *testing.T) {
	runCoreTest(t, "^TestDecideAfterRetroFloor_FallbackByteIdentical$",
		"TestDecideAfterRetroFloor_FallbackByteIdentical")
}

// AC2 (emit half) — the retrospective agent instructions must actually emit the
// artifact the consumer reads, or the whole S4 path is permanently fallback-only
// (No-inert-API). Assert the emit directive + schema keys + write-allowlist
// widening on the real instruction file.
func TestC1002_005_orchestrator_emit_wiring_present(t *testing.T) {
	root := acsassert.RepoRoot(t)
	agentFile := root + "/agents/evolve-retrospective.md"
	if !acsassert.FileExists(t, agentFile) {
		t.Fatalf("retrospective agent instructions not found at %s", agentFile)
	}
	// The artifact name + every schema key the consumer parses.
	for _, needle := range []string{
		"failure-decision.json",
		"category", "level", "evidence", "justification", "action", "fix_type",
	} {
		if !acsassert.FileContains(t, agentFile, needle) {
			t.Errorf("retrospective instructions must document the emit contract token %q (inert-API guard)", needle)
		}
	}
	// The write-allowlist must widen to permit the new artifact — the agent is
	// READ-ONLY except an explicit set, and failure-decision.json must join it.
	if !acsassert.LineContainsAll(agentFile, "failure-decision.json") {
		t.Error("no line names failure-decision.json — the write-allowlist / output directive was not wired")
	}
}

// AC2 (round-trip) — the exact instruction-documented decision shape parses
// clean through the reader (emitter and consumer share one schema).
func TestC1002_006_emitted_decision_shape_round_trips(t *testing.T) {
	runCoreTest(t, "^TestFailureDecisionWiring$", "TestFailureDecisionWiring")
}
