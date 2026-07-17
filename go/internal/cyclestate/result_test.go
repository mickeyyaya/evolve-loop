package cyclestate

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSystemFailureSignal_Wire pins the ADR-0072 system-failure signal JSON
// shape (serialized into the escalation dossier) and names the type (apicover).
func TestSystemFailureSignal_Wire(t *testing.T) {
	b, err := json.Marshal(SystemFailureSignal{Category: "verdict-incoherence", Level: "system", Evidence: "e", Halt: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"category"`, `"level"`, `"evidence"`, `"halt"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("SystemFailureSignal JSON missing %s: %s", want, b)
		}
	}
	// CycleResult carries the signal by pointer.
	r := CycleResult{SystemFailure: &SystemFailureSignal{Category: "verdict-incoherence", Halt: true}}
	if r.SystemFailure == nil || !r.SystemFailure.Halt {
		t.Error("CycleResult.SystemFailure not wired")
	}
}

// TestTokenUsage_Wire pins the snake_case JSON wire shape (cost telemetry is
// serialized into ledger/phase artifacts).
func TestTokenUsage_Wire(t *testing.T) {
	b, err := json.Marshal(TokenUsage{Input: 1, Output: 2, CacheRead: 3, CacheWrite: 4})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"input"`, `"output"`, `"cache_read"`, `"cache_write"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("TokenUsage JSON missing %s; got %s", want, b)
		}
	}
}

// TestDiagnostic_Wire pins the diagnostic wire shape.
func TestDiagnostic_Wire(t *testing.T) {
	b, err := json.Marshal(Diagnostic{Severity: "error", Message: "boom"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"severity"`, `"message"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("Diagnostic JSON missing %s; got %s", want, b)
		}
	}
}

// TestSkippedPhase_Wire pins the SkippedPhase wire shape — the degraded-phase
// record (cycle-802 retro-bridge-timeout-width10 guard) that preserves a
// non-floor phase's non-PASS verdict in the cycle dossier instead of letting it
// clobber a floor-derived FinalVerdict. Its snake-case JSON tags are the dossier
// contract, so a drift here silently drops the degrade audit trail.
func TestSkippedPhase_Wire(t *testing.T) {
	b, err := json.Marshal(SkippedPhase{Phase: "retro", Reason: "FAIL"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"phase"`, `"reason"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("SkippedPhase JSON missing %s; got %s", want, b)
		}
	}
}

// TestCycleResult constructs the cycle-summary value and checks PhasesRun uses
// the Phase type (so it composes with the rest of the leaf vocabulary).
func TestCycleResult(t *testing.T) {
	r := CycleResult{
		Cycle:         7,
		FinalVerdict:  VerdictPASS,
		PhasesRun:     []Phase{PhaseScout, PhaseBuild, PhaseShip},
		RetroDecision: "",
	}
	if r.Cycle != 7 || r.FinalVerdict != "PASS" {
		t.Errorf("CycleResult fields wrong: %+v", r)
	}
	if len(r.PhasesRun) != 3 || r.PhasesRun[0] != PhaseScout {
		t.Errorf("PhasesRun wrong: %+v", r.PhasesRun)
	}
}
