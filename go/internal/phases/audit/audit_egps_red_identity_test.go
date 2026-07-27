package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// audit_egps_red_identity_test.go — regression lock for the 2026-07-27
// batch-12 false breaker trip: the EGPS gate-block diagnostic said only
// "EGPS: red_count=1 (cycle ships only when red_count==0)" with NO predicate
// identity, so three DIFFERENT red predicates (cycles 1107/1115/1116 — three
// distinct whole-suite meta-predicates flaking under width-2 contention)
// produced byte-identical audit-fail reasons → one failure fingerprint
// (audit|gate-block|048c5b1ca3fb) → the identical-fingerprint pipeline
// breaker halted the batch on what were three distinct honest failures.
// Same class as the cycle-1054/1060 verdict-path collision: a constant
// failure message blinds the breaker's identity premise.
//
// Contract: the EGPS red_count diagnostic embeds the red predicates' ac_ids
// (capped — the message must stay one line), so distinct red predicates yield
// distinct fingerprints while a genuine recurrence (same predicate red again)
// still collides exactly.

// writeACSVerdictReds writes acs-verdict.json whose results carry the given
// red ac_ids (mirroring acssuite's real shape: red_count + results[]).
func writeACSVerdictReds(t *testing.T, ws string, redIDs ...string) {
	t.Helper()
	results := make([]map[string]any, 0, len(redIDs)+1)
	results = append(results, map[string]any{"ac_id": "cycleX/TestGreen", "result": "green"})
	for _, id := range redIDs {
		results = append(results, map[string]any{"ac_id": id, "result": "red"})
	}
	v := map[string]any{
		"cycle":     42,
		"red_count": len(redIDs),
		"results":   results,
	}
	b, _ := json.Marshal(v)
	if err := os.WriteFile(filepath.Join(ws, "acs-verdict.json"), b, 0o644); err != nil {
		t.Fatalf("write verdict: %v", err)
	}
}

// egpsRedDiagnostic runs the Classify path against a workspace whose verdict
// carries the given reds and returns the EGPS red_count diagnostic message.
func egpsRedDiagnostic(t *testing.T, redIDs ...string) string {
	t.Helper()
	ws := t.TempDir()
	writeACSVerdictReds(t, ws, redIDs...)
	h := hooks{}
	_, diags, _ := h.Classify(
		"# Audit Report\n\n## Verdict\n**PASS**\n",
		core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})
	for _, d := range diags {
		if strings.Contains(d.Message, "EGPS: red_count=") {
			return d.Message
		}
	}
	t.Fatalf("no EGPS red_count diagnostic; diags=%v", diags)
	return ""
}

// TestEGPSRedDiagnostic_CarriesPredicateIdentity — the crux: the message names
// the red predicate's SEMANTIC identity, so the failure fingerprint differs
// per distinct defect.
func TestEGPSRedDiagnostic_CarriesPredicateIdentity(t *testing.T) {
	m1 := egpsRedDiagnostic(t, "cycle1115/TestC1115_003_BridgeAndRecoveryStayGreen")
	if !strings.Contains(m1, "BridgeAndRecoveryStayGreen") {
		t.Errorf("EGPS diagnostic lacks the red predicate identity (fingerprint stays content-free):\n  %s", m1)
	}
	m2 := egpsRedDiagnostic(t, "cycle1116/TestC1116_005_CoreAndTopngateSuitesStayGreen")
	if m1 == m2 {
		t.Errorf("distinct red predicates produced IDENTICAL diagnostics — the breaker false-trip class:\n  %s", m1)
	}
}

// TestEGPSRedDiagnostic_CycleNumbersNeverEmbedded — the flip side: the same
// SEMANTIC defect red again on a retry cycle carries a NEW cycle-numbered
// ac_id; the messages must still be byte-identical, or the breaker goes blind
// to real cross-cycle recurrences (verdictFailDistinguisher's "never cycle
// numbers" rule).
func TestEGPSRedDiagnostic_CycleNumbersNeverEmbedded(t *testing.T) {
	m1 := egpsRedDiagnostic(t, "cycle1115/TestC1115_003_BridgeAndRecoveryStayGreen")
	m2 := egpsRedDiagnostic(t, "cycle1116/TestC1116_004_BridgeAndRecoveryStayGreen")
	if m1 != m2 {
		t.Errorf("same defect across retry cycles produced different messages (breaker misses the recurrence):\n  %s\n  %s", m1, m2)
	}
	if strings.Contains(m1, "1115") {
		t.Errorf("cycle number leaked into the diagnostic: %s", m1)
	}

	// The TWO-PART live convention (no index group): TestC<cycle>_<Name> —
	// real ids from .evolve/runs/cycle-841 and cycle-1000 acs-verdicts. The
	// first normalizer required C\d+_\d+_ and left "C841_" (a cycle number)
	// embedded — adversarial-review catch.
	a1 := egpsRedDiagnostic(t, "cycle841/TestC841_Amplify_CLIOutput_Memo_ResolvesToClaudeTmux")
	a2 := egpsRedDiagnostic(t, "cycle999/TestC999_Amplify_CLIOutput_Memo_ResolvesToClaudeTmux")
	if a1 != a2 {
		t.Errorf("two-part-convention recurrence produced different messages:\n  %s\n  %s", a1, a2)
	}
	if strings.Contains(a1, "841") {
		t.Errorf("cycle number leaked from two-part id: %s", a1)
	}
	if !strings.Contains(a1, "Amplify_CLIOutput_Memo_ResolvesToClaudeTmux") {
		t.Errorf("two-part id lost its semantic name: %s", a1)
	}
	// NEG-prefixed sibling shape (cycle-416 era) keeps its semantic tail.
	n1 := egpsRedDiagnostic(t, "cycle416/TestC416_NEG_MarkerlessBody_CompactionIsNoOp")
	if !strings.Contains(n1, "NEG_MarkerlessBody_CompactionIsNoOp") || strings.Contains(n1, "416") {
		t.Errorf("NEG-shape id mishandled: %s", n1)
	}
	// A name that merely STARTS with 'C'+letters is not cycle chrome and must
	// survive whole (C\d+ requires digits).
	c1 := egpsRedDiagnostic(t, "cycle12/TestCarryforward_003_Foo")
	if !strings.Contains(c1, "Carryforward_003_Foo") {
		t.Errorf("legitimate C-name mangled: %s", c1)
	}
}

// TestEGPSRedDiagnostic_CapsIDList — a mass-red verdict must not explode the
// one-line diagnostic: at most 5 ids are named, the rest summarized.
func TestEGPSRedDiagnostic_CapsIDList(t *testing.T) {
	ids := make([]string, 8)
	for i := range ids {
		ids[i] = "cycleX/TestRed_" + string(rune('A'+i))
	}
	m := egpsRedDiagnostic(t, ids...)
	if !strings.Contains(m, "red_count=8") {
		t.Errorf("count lost: %s", m)
	}
	named := 0
	for _, id := range ids {
		if strings.Contains(m, id) {
			named++
		}
	}
	if named > 5 {
		t.Errorf("message names %d ids (cap 5): %s", named, m)
	}
	if named == 0 {
		t.Errorf("message names no ids at all: %s", m)
	}
	if strings.Contains(m, "\n") {
		t.Errorf("diagnostic must stay one line: %q", m)
	}
}

// TestEGPSRedDiagnostic_LegacyVerdictWithoutResults — a verdict carrying only
// red_count (no results array — the shape writeACSVerdictShip pins for other
// tests) must still gate with the bare-count message, never crash or skip.
func TestEGPSRedDiagnostic_LegacyVerdictWithoutResults(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdictShip(t, ws, 2, nil)
	h := hooks{}
	_, diags, _ := h.Classify(
		"# Audit Report\n\n## Verdict\n**PASS**\n",
		core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})
	var found bool
	for _, d := range diags {
		if strings.Contains(d.Message, "EGPS: red_count=2") {
			found = true
		}
	}
	if !found {
		t.Errorf("legacy no-results verdict lost the EGPS gate; diags=%v", diags)
	}
}
