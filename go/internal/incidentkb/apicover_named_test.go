package incidentkb_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentkb"
)

func TestAPICover_NamesEveryExportedSymbol(t *testing.T) {
	t.Parallel()
	if incidentkb.Schema != "incident-record/1" || incidentkb.RelDir == "" || incidentkb.LegacyCutoff != "2026-11-19" {
		t.Fatalf("unexpected constants: %q %q %q", incidentkb.Schema, incidentkb.RelDir, incidentkb.LegacyCutoff)
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
	if !incidentkb.IsLegacy("2026-11-18-old.md", []byte("# old\n")) || incidentkb.IsLegacy("2026-11-19-new.md", []byte("# new\n")) {
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
