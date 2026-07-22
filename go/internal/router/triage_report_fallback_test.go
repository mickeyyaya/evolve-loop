package router

import "testing"

// triage_report_fallback_test.go — ADR-0076 slice A (A1): handoff-triage.json
// has been extinct since ~cycle 215, so without a report fallback the triage
// size signal is dead and every size-conditioned budget silently multiplies by
// 1.0. triageFromReportFallback mirrors scoutFromReportFallback but is richer:
// it extracts `cycle_size_estimate: <size>` from triage-report.md so
// RoutingSignals.CycleSize() carries a real value on the live artifact path.

func TestDigest_TriageFromReportFallback_ExtractsSize(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "triage-report.md", "<!-- challenge-token: abc -->\n"+
		"<!-- ANCHOR:triage_decision -->\n# Triage Decision — Cycle 999\n\n"+
		"cycle_size_estimate: medium\nphase_skip: []\n\n## top_n\n- item-a: x\n")
	sig, err := Digest(ws, []string{"triage"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if !sig.Triage.Present {
		t.Errorf("triage report present → Present must be true")
	}
	if sig.Triage.CycleSize != "medium" {
		t.Errorf("Triage.CycleSize = %q, want medium", sig.Triage.CycleSize)
	}
	if sig.CycleSize() != "medium" {
		t.Errorf("CycleSize() = %q, want medium (projection over fallback)", sig.CycleSize())
	}
}

func TestDigest_TriageFromReportFallback_CleanAbsence(t *testing.T) {
	ws := t.TempDir()
	sig, err := Digest(ws, []string{"triage"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if sig.Triage.Present {
		t.Errorf("no report, no handoff → clean absence (Present:false)")
	}
	if len(sig.DigestDegraded) != 0 {
		t.Errorf("clean absence must not degrade: %v", sig.DigestDegraded)
	}
}

func TestDigest_TriageFromReportFallback_ReportWithoutSizeLine(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "triage-report.md", "# Triage Decision — Cycle 7\n\n## top_n\n- item-a: x\n")
	sig, err := Digest(ws, []string{"triage"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if !sig.Triage.Present {
		t.Errorf("non-empty report → phase delivered → Present:true")
	}
	if sig.Triage.CycleSize != "" {
		t.Errorf("no size line → CycleSize empty, got %q", sig.Triage.CycleSize)
	}
}

func TestDigest_TriageHandoffWinsOverReport(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "handoff-triage.json", `{"cycle_size_estimate": "large"}`)
	writeFile(t, ws, "triage-report.md", "cycle_size_estimate: small\n")
	sig, err := Digest(ws, []string{"triage"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if sig.Triage.CycleSize != "large" {
		t.Errorf("handoff must win over report fallback: got %q, want large", sig.Triage.CycleSize)
	}
}

func TestDigest_TriageFallback_CompletedGating(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "triage-report.md", "cycle_size_estimate: small\n")
	sig, _ := Digest(ws, []string{"scout"})
	if sig.Triage.Present {
		t.Errorf("triage not in completed → Present must be false even if report exists")
	}
}
