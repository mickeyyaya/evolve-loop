package audit

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func errorMessages(diags []core.Diagnostic) []string {
	var out []string
	for _, d := range diags {
		if d.Severity == "error" {
			out = append(out, d.Message)
		}
	}
	return out
}

func containsAny(msgs []string, sub string) bool {
	for _, m := range msgs {
		if strings.Contains(m, sub) {
			return true
		}
	}
	return false
}

func TestVerdictConflict_EGPSRed_NarrativeAndGateFactsArriveTogether(t *testing.T) {
	verdict, diags := classifyWith(t, narrativeReport("PASS"), func(ws string) {
		writeACSVerdictReds(t, ws, "cycle1130/TestC1130_007_ProbeIsolation")
	})
	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict=%q, want FAIL — this is a visibility fix; the gate must still win", verdict)
	}
	errs := errorMessages(diags)
	for _, want := range []string{
		conflictMarker,
		"PASS",
		"red_count=1",
		"ProbeIsolation",
	} {
		if !containsAny(errs, want) {
			t.Errorf("no error-severity diagnostic carries %q — the operator gets half the forensic "+
				"pair, which is the exact 1107/1116/1117 shape this item was filed for.\nerror diags: %q",
				want, errs)
		}
	}
}

func TestVerdictConflict_ShipEligible_NarrativeAndGateFactsArriveTogether(t *testing.T) {
	no := false
	verdict, diags := classifyWith(t, narrativeReport("WARN"), func(ws string) {
		writeACSVerdictShip(t, ws, 0, &no)
	})
	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict=%q, want FAIL", verdict)
	}
	errs := errorMessages(diags)
	for _, want := range []string{conflictMarker, "WARN", "ship_eligible=false"} {
		if !containsAny(errs, want) {
			t.Errorf("no error-severity diagnostic carries %q\nerror diags: %q", want, errs)
		}
	}
}

func TestVerdictConflict_GateFactsSurviveWithoutAConflict(t *testing.T) {
	verdict, diags := classifyWith(t, narrativeReport("FAIL"), func(ws string) {
		writeACSVerdictReds(t, ws, "cycle1130/TestC1130_009_Coherent")
	})
	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict=%q, want FAIL", verdict)
	}
	errs := errorMessages(diags)
	if !containsAny(errs, "red_count=1") || !containsAny(errs, "Coherent") {
		t.Errorf("the gate's own evidence vanished on the coherent path — the conflict record must "+
			"be ADDITIVE, never a replacement for the gate diagnostic\nerror diags: %q", errs)
	}
	if containsAny(errs, conflictMarker) {
		t.Errorf("a conflict record was emitted where the auditor and the gate AGREED (both FAIL) — "+
			"fabricated conflicts dilute the signal\nerror diags: %q", errs)
	}
}

func TestVerdictConflict_CleanPassEmitsNeitherHalf(t *testing.T) {
	yes := true
	verdict, diags := classifyWith(t, narrativeReport("PASS"), func(ws string) {
		writeACSVerdictShip(t, ws, 0, &yes)
	})
	if verdict != core.VerdictPASS {
		t.Fatalf("verdict=%q, want PASS — a green gate must not disturb a narrative PASS", verdict)
	}
	if errs := errorMessages(diags); len(errs) != 0 {
		t.Errorf("clean PASS emitted %d error diagnostic(s): %q", len(errs), errs)
	}
}
