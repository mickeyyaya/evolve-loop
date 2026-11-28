package incidentkb_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentkb"
)

func TestAPICover_NamesEveryExportedSymbol(t *testing.T) {
	t.Parallel()
	if _, err := time.Parse("2006-01-02", incidentkb.LegacyCutoff); incidentkb.Schema != "incident-record/1" || incidentkb.RelDir == "" || err != nil {
		t.Fatalf("unexpected constants: %q %q %q (%v)", incidentkb.Schema, incidentkb.RelDir, incidentkb.LegacyCutoff, err)
	}
	if incidentkb.ErrMissingMarker == nil || incidentkb.ErrOverlappingMarkers == nil || incidentkb.ErrCoverageRow == nil {
		t.Fatal("the marker and coverage errors must be set")
	}
	line7 := &incidentkb.CoverageRowError{Line: 7, Reason: incidentkb.ErrMissingMarker}
	line8 := &incidentkb.CoverageRowError{Line: 8, Reason: incidentkb.ErrMissingMarker}
	if !errors.Is(line7, incidentkb.ErrCoverageRow) || line7.Unwrap() != incidentkb.ErrCoverageRow || line7.Error() == line8.Error() {
		t.Fatalf("CoverageRowError must unwrap to ErrCoverageRow and name its line: %v", line7)
	}
	if incidentkb.CoverageIndexRel != incidentkb.RelDir+"/REGRESSION-COVERAGE-INDEX.md" {
		t.Fatalf("CoverageIndexRel = %q", incidentkb.CoverageIndexRel)
	}
	coverage := []byte("<!-- GENERATED:coverage-summary BEGIN -->\n<!-- GENERATED:coverage-summary END -->\n## Coverage map\n| Incident | a | b | Coverage | c |\n|---|---|---|---|---|\n| [n](r.md) | m | f | ✅ | t |\n")
	if counts, err := incidentkb.CoverageCountsOf(coverage); err != nil || counts != (incidentkb.CoverageCounts{Incidents: 1, Rows: 1, Covered: 1}) {
		t.Fatalf("CoverageCountsOf = %+v, %v", counts, err)
	}
	if out, _, err := incidentkb.RenderCoverageSummary(coverage); err != nil || !strings.Contains(string(out), "| Incidents mapped | 1 |") {
		t.Fatalf("RenderCoverageSummary = %q, %v", out, err)
	}
	if len(incidentkb.RequiredSections) != 10 {
		t.Fatalf("RequiredSections = %d, want 10", len(incidentkb.RequiredSections))
	}
	if !incidentkb.ValidID("2026-11-19-sample") || incidentkb.ValidID("bad") {
		t.Errorf("ValidID mismatch")
	}
	if !incidentkb.IsSkipped("REGRESSION-COVERAGE-INDEX.md") || incidentkb.IsSkipped("2026-11-19-sample.md") {
		t.Errorf("IsSkipped mismatch")
	}
	for name, want := range map[string]bool{"2026-11-19-sample.md": true, "2026-11-20-session-log.md": false, "REGRESSION-COVERAGE-INDEX.md": false, "2026-11-19-sample.txt": false} {
		if incidentkb.IsRecordFile(name) != want {
			t.Errorf("IsRecordFile(%q) = %v, want %v", name, !want, want)
		}
	}
	dayBefore := dayShiftedFromCutoff(t, -1) + "-old.md"
	if !incidentkb.IsLegacy(dayBefore, []byte("# old\n")) || incidentkb.IsLegacy(incidentkb.LegacyCutoff+"-new.md", []byte("# new\n")) {
		t.Errorf("IsLegacy mismatch")
	}

	var (
		_ incidentkb.Action
		_ incidentkb.Filter
		_ incidentkb.Metric
		_ incidentkb.Baseline
		_ incidentkb.CheckResult
		_ incidentkb.Effectiveness
		_ incidentkb.Record
		_ incidentkb.ListEntry
		_ incidentkb.CheckReport
	)

	doc := sampleRecordDoc()
	rec, err := incidentkb.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if rec.ID != "2026-11-19-sample" {
		t.Errorf("rec.ID = %q", rec.ID)
	}
	rendered, err := incidentkb.Render([]byte(doc))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if again, err := incidentkb.RenderFor(rec.ID, []byte(doc)); err != nil || string(again) != string(rendered) {
		t.Errorf("RenderFor = %v", err)
	}
	if probs := incidentkb.CheckBytes(rec.ID, rendered); len(probs) != 0 {
		t.Errorf("CheckBytes = %v", probs)
	}

	root := t.TempDir()
	if err := os.MkdirAll(incidentkb.Dir(root), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(incidentkb.RecordPath(root, rec.ID), rendered, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(incidentkb.Dir(root), "2026-11-18-old.md"), []byte("# old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	probs, legacy, err := incidentkb.Check(root, rec.ID)
	if err != nil || legacy || len(probs) != 0 {
		t.Errorf("Check = (%v, %v, %v)", probs, legacy, err)
	}
	report, err := incidentkb.CheckAll(root)
	if err != nil || len(report.Records) != 1 || len(report.Legacy) != 1 {
		t.Errorf("CheckAll = %+v, %v", report, err)
	}
	rows, err := incidentkb.List(root, "fixed", "SHIP_INTEGRATION_EJECTED")
	if err != nil || len(rows) != 1 || !strings.Contains(rows[0].ID, rec.ID) {
		t.Errorf("List = %+v, %v", rows, err)
	}
}
