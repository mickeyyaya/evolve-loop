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

// TestSkippedPhase_Wire pins the SkippedPhase wire shape — a phase that did NOT
// run, with its skip CAUSE (closeout after an abnormal mid-cycle exit). Its
// snake-case JSON tags are the dossier contract.
func TestSkippedPhase_Wire(t *testing.T) {
	b, err := json.Marshal(SkippedPhase{Phase: "closeout", Reason: "abnormal exit in phase build"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"phase"`, `"reason"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("SkippedPhase JSON missing %s; got %s", want, b)
		}
	}
}

// TestVerdictNotAdopted_Wire pins the ran-but-declined record (cycle-802
// retro-bridge-timeout-width10 guard): a non-floor phase's non-PASS verdict is
// preserved in the cycle dossier instead of clobbering a floor-derived
// FinalVerdict. It is DISTINCT from SkippedPhase — the field names the phase's
// VERDICT, so the dossier can never claim retro was skipped on a cycle where retro
// ran (dossier-retro-skipped-mislabel). The tags are the dossier contract, so a
// drift here silently drops the audit trail.
func TestVerdictNotAdopted_Wire(t *testing.T) {
	b, err := json.Marshal(VerdictNotAdopted{Phase: "retro", Verdict: "FAIL"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"phase"`, `"verdict"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("VerdictNotAdopted JSON missing %s; got %s", want, b)
		}
	}
	if strings.Contains(string(b), `"reason"`) {
		t.Errorf("VerdictNotAdopted must not carry a skip-shaped \"reason\" key: %s", b)
	}
	// CycleResult carries the records; SkippedPhases stays reserved for real skips.
	r := CycleResult{VerdictsNotAdopted: []VerdictNotAdopted{{Phase: "retro", Verdict: "FAIL"}}}
	if len(r.VerdictsNotAdopted) != 1 || r.VerdictsNotAdopted[0].Phase != "retro" {
		t.Errorf("CycleResult.VerdictsNotAdopted not wired: %+v", r)
	}
	if len(r.SkippedPhases) != 0 {
		t.Errorf("a ran-but-declined phase must not land in SkippedPhases: %+v", r.SkippedPhases)
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
