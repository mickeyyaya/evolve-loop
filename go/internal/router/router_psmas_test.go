package router

// Cycle-252 task `psmas-phase-skip-wire-go-router` — TDD contract (RED first).
//
// The PSMAS gap (scout F1): digest.go extracts Triage.PhaseSkip from the
// triage handoff, but Route() never consumes it — the Go path has always
// been a no-op for PSMAS. These tests pin the wiring contract:
//
//   1. RouteInput gains a PSMASEnabled bool gate (orchestrator wires it
//      from EVOLVE_PSMAS_SKIP=1). Until the field exists this file does
//      not compile — that compile error IS the RED signal for the new API.
//   2. When enabled, Triage.PhaseSkip is unioned into the skip decision
//      ADDITIVELY: it can only skip phases that are genuinely optional
//      this cycle. Mandatory phases and the conditional tdd pin
//      (cycle_size != trivial) always win.
//   3. Triage emits persona vocabulary ("tdd-engineer", per
//      agents/evolve-triage.md §3a); the router order uses canonical
//      names ("tdd"). The wiring must normalize, or real triage output
//      silently never matches.
//   4. Gate off ⇒ byte-identical legacy behavior (PhaseSkip ignored).

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
)

// psmasBase mirrors base() but with the PSMAS gate open and a present
// triage signal — the minimal enabled-path fixture.
func psmasBase(cur string, size string, skip []string) RouteInput {
	in := base(cur)
	in.PSMASEnabled = true
	in.Signals.Triage = TriageSignals{CycleSize: size, PhaseSkip: skip, Present: true}
	return in
}

// TestPSMAS_SkipsTriageRecommendedOptionalPhase: a phase that WOULD run
// (tester's insert_when fires on build.acs_red>0) is skipped when triage
// recommends it and the gate is open. This is the enforcement delta the
// no-op status quo cannot produce: pre-wiring, this input routes to tester.
func TestPSMAS_SkipsTriageRecommendedOptionalPhase(t *testing.T) {
	in := psmasBase("build", "small", []string{"tester"})
	in.Completed = []string{"scout", "tdd", "build"}
	in.Signals.Build = BuildSignals{ACSRed: 2, Present: true}

	d := Route(in, nil)
	if d.NextPhase != "audit" {
		t.Errorf("build(red>0, psmas skip tester) → %q, want audit", d.NextPhase)
	}
	if !containsPhase(d.SkipPhases, "tester") {
		t.Errorf("SkipPhases = %v, want tester recorded (skip must be auditable, not silent)", d.SkipPhases)
	}
}

// TestPSMAS_GateOffIgnoresPhaseSkip: identical input with the gate closed
// routes exactly as legacy — tester runs. This is the anti-no-op pair to
// the test above and pins "EVOLVE_PSMAS_SKIP unset ⇒ byte-identical".
func TestPSMAS_GateOffIgnoresPhaseSkip(t *testing.T) {
	in := psmasBase("build", "small", []string{"tester"})
	in.PSMASEnabled = false
	in.Completed = []string{"scout", "tdd", "build"}
	in.Signals.Build = BuildSignals{ACSRed: 2, Present: true}

	d := Route(in, nil)
	if d.NextPhase != "tester" {
		t.Errorf("gate off → %q, want tester (PhaseSkip must be ignored)", d.NextPhase)
	}
}

// TestPSMAS_GateOnWithoutTriagePresenceDoesNotSkip: the gate alone is not
// enough. PhaseSkip data only counts when a triage handoff is actually
// present, otherwise a stale zero-value/default signal could silently skip
// a phase on cycles that never ran triage.
func TestPSMAS_GateOnWithoutTriagePresenceDoesNotSkip(t *testing.T) {
	in := psmasBase("build", "small", []string{"tester"})
	in.Signals.Triage.Present = false
	in.Completed = []string{"scout", "tdd", "build"}
	in.Signals.Build = BuildSignals{ACSRed: 2, Present: true}

	d := Route(in, nil)
	if d.NextPhase != "tester" {
		t.Errorf("gate on but no triage presence -> %q, want tester (stale PhaseSkip ignored)", d.NextPhase)
	}
	if containsPhase(d.SkipPhases, "tester") {
		t.Errorf("SkipPhases = %v, want no tester skip recorded without triage presence", d.SkipPhases)
	}
}

// TestPSMAS_IgnoresUnknownPhaseNamesButAppliesKnownOnes: triage output is
// advisory, so a malformed phase name must not poison the whole skip set.
// The known optional recommendation still applies and remains auditable.
func TestPSMAS_IgnoresUnknownPhaseNamesButAppliesKnownOnes(t *testing.T) {
	in := psmasBase("build", "small", []string{"not-a-phase", "tester"})
	in.Completed = []string{"scout", "tdd", "build"}
	in.Signals.Build = BuildSignals{ACSRed: 2, Present: true}

	d := Route(in, nil)
	if d.NextPhase != "audit" {
		t.Errorf("unknown + tester skip -> %q, want audit (known optional skip still applies)", d.NextPhase)
	}
	if !containsPhase(d.SkipPhases, "tester") {
		t.Errorf("SkipPhases = %v, want tester recorded", d.SkipPhases)
	}
	if containsPhase(d.SkipPhases, "not-a-phase") {
		t.Errorf("SkipPhases = %v, want unknown phase name ignored", d.SkipPhases)
	}
}

// TestPSMAS_CannotSkipMandatoryPhase: PSMAS is additive-only. A hostile or
// buggy phase_skip[] naming mandatory phases must not weaken the spine.
func TestPSMAS_CannotSkipMandatoryPhase(t *testing.T) {
	in := psmasBase("scout", "trivial", []string{"build", "audit", "ship"})
	in.Completed = []string{"scout"}

	d := Route(in, nil)
	if d.NextPhase != "build" {
		t.Errorf("psmas skip [build,audit,ship] → %q, want build (mandatory-never-skipped)", d.NextPhase)
	}
}

// TestPSMAS_CannotUnpinTDDOnNonTrivial: the conditional-mandatory pin
// (tdd unless cycle_size==trivial) outranks the PSMAS recommendation —
// triage persona promises never to recommend it, but the router must not
// trust the persona (integrity floor: ship ⇒ tdd unless trivial).
func TestPSMAS_CannotUnpinTDDOnNonTrivial(t *testing.T) {
	in := psmasBase("scout", "medium", []string{"tdd-engineer"})
	in.Completed = []string{"scout"}

	d := Route(in, nil)
	if d.NextPhase != "tdd" {
		t.Errorf("psmas skip tdd-engineer on medium → %q, want tdd (conditional pin wins)", d.NextPhase)
	}
}

// TestPSMAS_TriageVocabularyNormalized: triage emits "tdd-engineer"
// (agents/evolve-triage.md §3a mapping for trivial), the router order says
// "tdd". On a trivial cycle the skip is legal and must take effect under
// the persona vocabulary — and be recorded canonically in SkipPhases.
//
// PhaseEnable[tdd]=On mirrors the production default
// (EVOLVE_TEST_PHASE_ENABLED=1 → EnableOn): legacy routing RUNS tdd here
// even on trivial, so PSMAS skip is the only mechanism — this is the
// scenario the whole task exists for. The trivial-only conditional rule
// keeps it floor-safe.
func TestPSMAS_TriageVocabularyNormalized(t *testing.T) {
	in := psmasBase("scout", "trivial", []string{"tdd-engineer", "retrospective"})
	in.Completed = []string{"scout"}
	in.Cfg.PhaseEnable["tdd"] = config.EnableOn

	d := Route(in, nil)
	if d.NextPhase != "build" {
		t.Errorf("trivial + psmas skip tdd-engineer → %q, want build", d.NextPhase)
	}
	if !containsPhase(d.SkipPhases, "tdd") {
		t.Errorf("SkipPhases = %v, want canonical \"tdd\" recorded from triage's \"tdd-engineer\"", d.SkipPhases)
	}
}

func containsPhase(list []string, want string) bool {
	for _, p := range list {
		if p == want {
			return true
		}
	}
	return false
}
