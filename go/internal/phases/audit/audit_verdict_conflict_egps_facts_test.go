package audit

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// audit_verdict_conflict_egps_facts_test.go — the cycle-1130 increment for the
// inbox item `verdict-coherence-auditor-vs-egps`.
//
// The sibling suites (audit_verdict_conflict_test.go, ..._gates_test.go,
// ..._narrative_test.go) already pin that a `verdict-conflict:` record EXISTS,
// carries the narrative verdict verbatim, is distinguishable per gate reason,
// and stays silent on every coherent case. What none of them pin is the fact
// the scout report's verifiableBy actually asks for: that ONE Classify call
// hands the operator BOTH halves of the forensic pair —
//
//	(a) the auditor's own declared verdict, and
//	(b) the gate's red identity facts (red_count and the normalized red_ids),
//
// both at Severity=="error", so both ride errorSeverityMessages →
// AuditFailReasons → <phase>-fail-reason.json → the dossier's SubstantiveError.
//
// Scope note (the one ambiguity in the AC, resolved deliberately): the AC reads
// "the returned []core.Diagnostic contains a message matching both PASS and the
// red_count/red_ids facts". Read strictly that demands a SINGLE message holding
// both; the shipped implementation instead splits them across two
// error-severity diagnostics returned from the same call ("Gate detail is in
// the error diagnostics beside this one"). These tests pin the SLICE-level
// reading, because the operator-visible outcome the item was filed for — a
// dossier that shows the disagreement next to the evidence — is satisfied
// either way, and both diagnostics travel the same error-severity chain as one
// unit. A future refactor that keeps the conflict record but drops the gate
// facts (or demotes either to warning) breaks the pair and fails here, which is
// the regression this file exists to catch.

// errorMessages returns every error-severity diagnostic message. Severity is
// load-bearing, not cosmetic: errorSeverityMessages (core/system_failure.go)
// keys off exactly this, so a warning-severity fact is a DROPPED fact.
func errorMessages(diags []core.Diagnostic) []string {
	var out []string
	for _, d := range diags {
		if d.Severity == "error" {
			out = append(out, d.Message)
		}
	}
	return out
}

// containsAny reports whether any string in msgs contains sub.
func containsAny(msgs []string, sub string) bool {
	for _, m := range msgs {
		if strings.Contains(m, sub) {
			return true
		}
	}
	return false
}

// TestVerdictConflict_EGPSRed_NarrativeAndGateFactsArriveTogether — AC-1. A
// narrative PASS over a red predicate suite must leave the operator holding the
// pair: what the auditor said, and which predicate the gate actually tripped
// on. Either half alone is what cycles 1107/1116/1117 already had, and it was
// not enough to tell a genuine defect from a poisoned predicate.
func TestVerdictConflict_EGPSRed_NarrativeAndGateFactsArriveTogether(t *testing.T) {
	verdict, diags := classifyWith(t, narrativeReport("PASS"), func(ws string) {
		writeACSVerdictReds(t, ws, "cycle1130/TestC1130_007_ProbeIsolation")
	})
	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict=%q, want FAIL — this is a visibility fix; the gate must still win", verdict)
	}
	errs := errorMessages(diags)
	// The identity token is the CYCLE-NORMALIZED tail, not the raw ac_id:
	// egpsRedIDCycleTokens (audit.go) strips the cycle group and index on
	// purpose, so three occurrences of the same predicate across cycles collide
	// into one failure fingerprint instead of minting a fresh one per retry
	// (bc2e3236, the batch-12 breaker false-trip). Asserting the raw
	// "cycle1130/TestC1130_007_ProbeIsolation" here would pin the exact behavior
	// that commit removed on purpose; what the AC actually needs is that the
	// operator can tell WHICH predicate tripped, and the normalized tail carries
	// that.
	for _, want := range []string{
		conflictMarker,   // the record itself
		"PASS",           // (a) the auditor's own verdict
		"red_count=1",    // (b) the gate's count
		"ProbeIsolation", // (b) the tripped predicate's normalized identity
	} {
		if !containsAny(errs, want) {
			t.Errorf("no error-severity diagnostic carries %q — the operator gets half the forensic "+
				"pair, which is the exact 1107/1116/1117 shape this item was filed for.\nerror diags: %q",
				want, errs)
		}
	}
}

// TestVerdictConflict_ShipEligible_NarrativeAndGateFactsArriveTogether — the
// same pair on the red_count==0 / ship_eligible=false branch, where there are
// no red_ids to name and the gate's reason IS the ship_eligible field.
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

// TestVerdictConflict_GateFactsSurviveWithoutAConflict — the negative axis, and
// the anti-no-op guard for the two tests above. On the COHERENT case (auditor
// also said FAIL) there is no disagreement to record, but the gate's evidence
// must still reach the dossier untouched. An implementation that only ever
// emits facts alongside a conflict record — or that swallows the plain gate
// message once the conflict path exists — greens both tests above and fails
// here.
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

// TestVerdictConflict_CleanPassEmitsNeitherHalf — the untouched happy path: a
// narrative PASS over a green suite keeps PASS and emits no error diagnostics
// at all. Pins that nothing above leaks onto the path that ships.
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
