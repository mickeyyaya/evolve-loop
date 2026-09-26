package router

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// buildHandoff mirrors a real handoff-build.json shape.
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
	if !sig.Audit.Present || sig.Audit.Confidence != 0.88 {
		t.Errorf("audit = %+v", sig.Audit)
	}
	if sig.Audit.DefectsBySeverity[SevMedium] != 1 || sig.Audit.DefectsBySeverity[SevLow] != 1 {
		t.Errorf("audit defects = %+v", sig.Audit.DefectsBySeverity)
	}
	if !sig.Scout.Present || sig.Scout.ItemCount != 3 {
		t.Errorf("scout ItemCount = %d, want 3", sig.Scout.ItemCount)
	}
	if sig.Scout.CycleSizeEstimate != "medium" {
		t.Errorf("scout size = %q", sig.Scout.CycleSizeEstimate)
	}
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

func TestDigest_GenericSignalFold(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "handoff-build.json", `{
	  "phase": "build", "verdict": "PASS",
	  "acs_result": {"green": 1, "red": 0, "total": 1},
	  "signals": { "files_touched": 4, "security.precheck": "clean" }
	}`)
	sig, err := Digest(ws, []string{"build"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	got, ok := sig.GenericValue("build.files_touched")
	if f, isF := got.(float64); !ok || !isF || f != 4 {
		t.Errorf("Generic[build.files_touched] = %v (%T, ok=%v), want float64(4)", got, got, ok)
	}
	got, ok = sig.GenericValue("security.precheck")
	if s, isS := got.(string); !ok || !isS || s != "clean" {
		t.Errorf("Generic[security.precheck] = %v (%T, ok=%v), want \"clean\" (dotted key kept as-is)", got, got, ok)
	}
	if !sig.Build.Present || sig.Build.Verdict != "PASS" {
		t.Errorf("typed Build extraction regressed: %+v", sig.Build)
	}
}

func TestDigest_NoSignalsBlock_GenericNil(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "handoff-scout.json", scoutHandoff)
	sig, _ := Digest(ws, []string{"scout"})
	if sig.Generic != nil {
		t.Errorf("Generic = %v, want nil when no signals block present", sig.Generic)
	}
}

func TestDigest_FailOpenOnTruncatedJSON(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "handoff-build.json", `{"phase":"build"`)

	sig, err := Digest(ws, []string{"scout", "build", "audit"})
	if err != nil {
		t.Fatalf("Digest should not error on truncated JSON: %v", err)
	}
	if sig.Build.Present {
		t.Errorf("truncated build handoff must yield Present:false (fail-open)")
	}
}

// tdd's contract artifact is test-report.md, resolved through the phasecontract registry.
func TestDigest_LiftsFailureSentinelSignals(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", "## Verdict\nFAIL\n"+
		phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL",
			&phasecontract.FailureBlock{Class: "code-audit-fail", Defects: []string{"d1", "d2"}})+"\n")
	writeFile(t, ws, "test-report.md", "## Tests\n"+
		phasecontract.RenderVerdictSentinelWithFailure("tdd", "FAIL",
			&phasecontract.FailureBlock{Class: "code-build-fail", Defects: []string{"t1"}})+"\n")

	sig, err := Digest(ws, []string{"tdd", "audit"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if got, _ := sig.GenericValue("audit.failure_class"); got != "code-audit-fail" {
		t.Errorf("audit.failure_class = %v, want code-audit-fail", got)
	}
	if got, _ := sig.GenericValue("audit.defect_count"); got != float64(2) {
		t.Errorf("audit.defect_count = %v, want 2", got)
	}
	if got, _ := sig.GenericValue("tdd.failure_class"); got != "code-build-fail" {
		t.Errorf("tdd.failure_class = %v, want code-build-fail (contract artifact name test-report.md)", got)
	}

	if !evalCondition(sig, config.Condition{Field: "audit.defect_count", Op: "gte", Value: 1}) {
		t.Error("condition audit.defect_count gte 1 must match the lifted signal")
	}
}

func TestDigest_NoFailureSignalsOnPass(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", "## Verdict\nPASS\n"+
		phasecontract.RenderVerdictSentinel("audit", "PASS")+"\n")
	sig, err := Digest(ws, []string{"audit"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if _, ok := sig.GenericValue("audit.failure_class"); ok {
		t.Error("PASS artifact must not surface failure_class")
	}
	if _, ok := sig.GenericValue("audit.defect_count"); ok {
		t.Error("PASS artifact must not surface defect_count")
	}
}
