package router

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// buildHandoff mirrors the real cycle-55 handoff-build.json shape.
const buildHandoff = `{
  "schema_version": 1, "cycle": 55, "phase": "build", "verdict": "PASS",
  "acs_result": {"green": 30, "red": 2, "total": 32, "this_cycle": 4, "regression": 26},
  "thrusts": [
    {"id":"t1","severity":"HIGH","files_modified":["a.go","b.go"],"files_new":["c.go"]},
    {"id":"t2","severity":"CRITICAL","files_modified":["a.go"],"files_new":[]}
  ]
}`

const auditHandoff = `{
  "cycle": 106, "verdict": "WARN", "confidence": 0.88, "red_count": 0,
  "defects": [ {"id":"d1","severity":"medium"}, {"id":"d2","severity":"low"} ]
}`

const scoutHandoff = `{
  "cycle": 56, "cycle_size_estimate": "medium",
  "item1_a": {}, "item2_b": {}, "item3_c": {}, "items_not_a_block": {}, "tdd_order": []
}`

func TestDigest_AllRolesExtracted(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "handoff-build.json", buildHandoff)
	writeFile(t, ws, "handoff-auditor.json", auditHandoff) // auditor variant
	writeFile(t, ws, "handoff-scout.json", scoutHandoff)
	writeFile(t, ws, "handoff-triage.json", `{"cycle_size_estimate":"medium","phase_skip":["retrospective"]}`)

	sig, err := Digest(ws, []string{"scout", "triage", "build", "audit"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}

	// Build
	if !sig.Build.Present || sig.Build.Verdict != "PASS" {
		t.Errorf("build = %+v", sig.Build)
	}
	if sig.Build.ACSRed != 2 || sig.Build.ACSRegression != 26 {
		t.Errorf("build acs = red %d reg %d, want 2/26", sig.Build.ACSRed, sig.Build.ACSRegression)
	}
	if sig.Build.SeverityMax != SevCritical {
		t.Errorf("build SeverityMax = %v, want CRITICAL", sig.Build.SeverityMax)
	}
	if sig.Build.FilesTouched != 3 { // a.go,b.go,c.go (a.go deduped)
		t.Errorf("build FilesTouched = %d, want 3 (deduped union)", sig.Build.FilesTouched)
	}
	// Audit (auditor filename variant resolved)
	if !sig.Audit.Present || sig.Audit.Confidence != 0.88 {
		t.Errorf("audit = %+v", sig.Audit)
	}
	if sig.Audit.DefectsBySeverity[SevMedium] != 1 || sig.Audit.DefectsBySeverity[SevLow] != 1 {
		t.Errorf("audit defects = %+v", sig.Audit.DefectsBySeverity)
	}
	// Scout: 3 itemN_ blocks, "items_not_a_block" excluded
	if !sig.Scout.Present || sig.Scout.ItemCount != 3 {
		t.Errorf("scout ItemCount = %d, want 3", sig.Scout.ItemCount)
	}
	if sig.Scout.CycleSizeEstimate != "medium" {
		t.Errorf("scout size = %q", sig.Scout.CycleSizeEstimate)
	}
	// Triage + authoritative CycleSize precedence
	if !sig.Triage.Present || sig.Triage.CycleSize != "medium" || len(sig.Triage.PhaseSkip) != 1 {
		t.Errorf("triage = %+v", sig.Triage)
	}
	if sig.CycleSize() != "medium" {
		t.Errorf("CycleSize() = %q, want medium", sig.CycleSize())
	}
}

func TestDigest_BuilderNamingTolerance(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "handoff-builder.json", buildHandoff) // builder variant only
	sig, _ := Digest(ws, []string{"build"})
	if !sig.Build.Present {
		t.Errorf("expected handoff-builder.json to be resolved")
	}
}

func TestDigest_FailOpenOnMissingAndCorrupt(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "handoff-build.json", `{ this is not json `)

	sig, err := Digest(ws, []string{"scout", "build", "audit"})
	if err != nil {
		t.Fatalf("Digest should not error on missing/corrupt: %v", err)
	}
	if sig.Build.Present {
		t.Errorf("corrupt build handoff must yield Present:false (fail-open)")
	}
	if sig.Audit.Present || sig.Scout.Present {
		t.Errorf("missing artifacts must yield Present:false")
	}
}

func TestDigest_CompletedGating(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "handoff-build.json", buildHandoff)
	// build artifact exists on disk, but build NOT in completed → not Present.
	sig, _ := Digest(ws, []string{"scout"})
	if sig.Build.Present {
		t.Errorf("build not in completed → Present must be false even if artifact exists")
	}
}

func TestCycleSize_FallbackToScout(t *testing.T) {
	sig := RoutingSignals{Scout: ScoutSignals{CycleSizeEstimate: "large", Present: true}}
	if sig.CycleSize() != "large" {
		t.Errorf("CycleSize() = %q, want large (scout fallback)", sig.CycleSize())
	}
	if (RoutingSignals{}).CycleSize() != "" {
		t.Errorf("empty signals CycleSize() should be empty string")
	}
}
