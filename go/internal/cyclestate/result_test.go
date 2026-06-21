package cyclestate

import (
	"encoding/json"
	"strings"
	"testing"
)

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
