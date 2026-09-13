package cyclehealth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A phase that recorded verdict FAIL with its own error-severity diagnostics
// (cycles 1634/1636: triage's protected-surface admission rejection) is
// FAILED_EXPLAINED — it already was, by the verdict — but the detail must name
// the phase's reason so the batch report says WHY, not merely THAT. Warnings
// are not reasons; without an error-severity diagnostic the detail keeps its
// verdict-only wording.
func TestClassifyOutcome_FailVerdictDetailNamesThePhaseOwnReason(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	timing := `[{"phase":"scout","verdict":"PASS","duration_ms":1,"cost_usd":0,"attempt_count":1},
	{"phase":"triage","verdict":"FAIL","duration_ms":1,"cost_usd":0,"attempt_count":1,
	 "diagnostics":[{"severity":"warning","message":"metrics file absent"},
	                {"severity":"error","message":"top_n card \"x\" names protected surface \"go/internal/phases/ship/gitops.go\""}]}]`
	if err := os.WriteFile(filepath.Join(ws, "phase-timing.json"), []byte(timing), 0o644); err != nil {
		t.Fatal(err)
	}
	got, detail := ClassifyOutcome(ws)
	if got != OutcomeFailedExplained {
		t.Fatalf("outcome = %s, want %s (detail %q)", got, OutcomeFailedExplained, detail)
	}
	if !strings.Contains(detail, "triage") || !strings.Contains(detail, "names protected surface") {
		t.Errorf("detail must name the phase and its own error diagnostic: %q", detail)
	}
	if strings.Contains(detail, "metrics file absent") {
		t.Errorf("warnings are not reasons: %q", detail)
	}

	warnOnly := t.TempDir()
	if err := os.WriteFile(filepath.Join(warnOnly, "phase-timing.json"), []byte(`[{"phase":"triage","verdict":"FAIL","duration_ms":1,"cost_usd":0,"attempt_count":1,"diagnostics":[{"severity":"warning","message":"metrics file absent"}]}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, detail = ClassifyOutcome(warnOnly)
	if got != OutcomeFailedExplained || !strings.Contains(detail, "recorded verdict FAIL") || strings.Contains(detail, "metrics") {
		t.Errorf("warning-only FAIL keeps the verdict-only detail, got %s %q", got, detail)
	}
}
