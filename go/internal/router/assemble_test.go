package router

import "testing"

func TestAssembleHandoffs_MatchesDigest(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "handoff-build.json", buildHandoff)
	writeFile(t, ws, "handoff-auditor.json", auditHandoff)
	writeFile(t, ws, "handoff-scout.json", scoutHandoff)
	roles := []string{"scout", "build", "audit"}

	sig, err := Digest(ws, roles)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	h, err := AssembleHandoffs(ws, roles)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}

	sc, ok := h.Scout()
	if !ok || sc.CycleSizeEstimate != sig.Scout.CycleSizeEstimate || sc.ItemCount != sig.Scout.ItemCount {
		t.Errorf("scout: (%+v, ok=%v) vs digest %+v", sc, ok, sig.Scout)
	}
	b, ok := h.Build()
	if !ok || b.Verdict != sig.Build.Verdict || b.SeverityMax != sig.Build.SeverityMax.String() ||
		b.FilesTouched != sig.Build.FilesTouched || b.ACSRed != sig.Build.ACSRed {
		t.Errorf("build: (%+v, ok=%v) vs digest %+v", b, ok, sig.Build)
	}
	a, ok := h.Audit()
	if !ok || a.RedCount != sig.Audit.RedCount || a.Confidence != sig.Audit.Confidence {
		t.Errorf("audit: (%+v, ok=%v) vs digest %+v", a, ok, sig.Audit)
	}
	if a.DefectsBySeverity["MEDIUM"] != sig.Audit.DefectsBySeverity[SevMedium] ||
		a.DefectsBySeverity["LOW"] != sig.Audit.DefectsBySeverity[SevLow] {
		t.Errorf("audit defects word-keyed: %+v vs ordinal %+v", a.DefectsBySeverity, sig.Audit.DefectsBySeverity)
	}
}

func TestAssembleHandoffs_AbsentRole_OkFalse(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "handoff-build.json", buildHandoff)
	h, err := AssembleHandoffs(ws, []string{"build"})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if _, ok := h.Scout(); ok {
		t.Errorf("absent scout should be ok=false")
	}
	if _, ok := h.Audit(); ok {
		t.Errorf("absent audit should be ok=false")
	}
	if _, ok := h.Build(); !ok {
		t.Errorf("present build should be ok=true")
	}
}
