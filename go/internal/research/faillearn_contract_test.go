package research

// SCHEMA-PARITY GUARD (do not "clean up" the faillearn import): this
// white-box test is the producer/consumer contract between
// faillearn.RenderLessonYAML (deterministic fallback lesson writer) and
// parseLessonFile (the only corpus reader). It exists so the two
// packages can stay decoupled — faillearn a leaf, research the reader —
// without the twin-schema silent-divergence risk. If either side
// changes shape, this test fails first.

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn"
)

// If this count drifts, a field was added to research.lessonYAML that
// faillearn.RenderLessonYAML may not be writing (or vice versa): update
// faillearn's twin schema struct AND the assertions below together.
const expectedLessonSchemaFields = 9 // 8 scalar fields + failureContext

func TestLessonSchema_FieldCountPinsFaillearnParity(t *testing.T) {
	if n := reflect.TypeOf(lessonYAML{}).NumField(); n != expectedLessonSchemaFields {
		t.Errorf("research.lessonYAML has %d fields, contract pins %d — sync faillearn's schema struct and this test", n, expectedLessonSchemaFields)
	}
}

func TestParseLessonFile_RoundTripsFaillearnRender(t *testing.T) {
	// Fixture intentionally mirrors faillearn_test.go:fixtureEvent —
	// keep the two in sync (cross-package test helpers can't be shared).
	ev := faillearn.FailureEvent{
		Cycle:          243,
		FailedPhase:    "retrospective",
		Scope:          faillearn.ScopePhase,
		Classification: "cycle-mid-execution-fail",
		Verdict:        "FAIL",
		Summary:        "retro bridge exited 81 before writing retrospective-report.md",
		Defects:        []string{"bridge timeout exit=81"},
		EvidencePaths:  []string{".evolve/runs/cycle-243/orchestrator-report.md"},
		GitHead:        "28aa4c3",
		Now:            time.Date(2026, 6, 7, 8, 30, 0, 0, time.UTC),
	}
	id, body := faillearn.RenderLessonYAML(ev)

	path := filepath.Join(t.TempDir(), id+".yaml")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write rendered lesson: %v", err)
	}

	lessons, err := parseLessonFile(path)
	if err != nil {
		t.Fatalf("parseLessonFile rejects faillearn output: %v\n%s", err, body)
	}
	if len(lessons) != 1 {
		t.Fatalf("want 1 lesson, got %d", len(lessons))
	}

	l := lessons[0]
	// All 11 corpus fields must survive the round trip.
	if l.ID != id {
		t.Errorf("ID = %q, want %q", l.ID, id)
	}
	if l.Pattern == "" {
		t.Error("Pattern empty after round trip")
	}
	if l.Description == "" {
		t.Error("Description empty after round trip")
	}
	if l.Confidence <= 0 || l.Confidence > 1 {
		t.Errorf("Confidence = %v, want (0,1] as parsed float", l.Confidence)
	}
	if l.Source == "" {
		t.Error("Source empty after round trip")
	}
	if l.Type == "" {
		t.Error("Type empty after round trip")
	}
	if l.Category == "" {
		t.Error("Category empty after round trip")
	}
	if l.PreventiveAction == "" {
		t.Error("PreventiveAction empty after round trip")
	}
	if l.FailedStep != "retrospective" {
		t.Errorf("failureContext.failedStep = %q, want %q", l.FailedStep, "retrospective")
	}
	if l.ErrorCategory != "cycle-mid-execution-fail" {
		t.Errorf("failureContext.errorCategory = %q, want %q", l.ErrorCategory, "cycle-mid-execution-fail")
	}
	if l.AuditVerdict != "FAIL" {
		t.Errorf("failureContext.auditVerdict = %q, want %q", l.AuditVerdict, "FAIL")
	}
}
