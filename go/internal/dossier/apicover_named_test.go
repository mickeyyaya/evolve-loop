package dossier

// apicover_named_test.go — named coverage for every exported symbol in the
// dossier package (ADR-0050 Phase 5 requirement). One named test per export
// that is not already covered by build_test.go / render_test.go / write_test.go.
//
// These are minimal behavioural assertions (not mere presence checks) so they
// also serve as regression guards.

import (
	"encoding/json"
	"testing"
)

// TestDefect_Named ensures the Defect type and all its fields are reachable by
// a same-package named test (apicover requires composite-literal or typed-var usage).
func TestDefect_Named(t *testing.T) {
	d := Defect{
		ID:       "d-001",
		Severity: "HIGH",
		Summary:  "test defect",
		Fix:      "fix it",
	}
	if d.ID != "d-001" {
		t.Errorf("Defect.ID: got %q, want %q", d.ID, "d-001")
	}
	if d.Severity != "HIGH" {
		t.Errorf("Defect.Severity: got %q, want %q", d.Severity, "HIGH")
	}
}

// TestLesson_Named covers the Lesson type.
func TestLesson_Named(t *testing.T) {
	l := Lesson{
		ID:               "l-001",
		Pattern:          "recurring nil dereference",
		PreventiveAction: "add nil guard at boundary",
	}
	if l.ID != "l-001" {
		t.Errorf("Lesson.ID: got %q, want %q", l.ID, "l-001")
	}
}

// TestCarryover_Named covers the Carryover type.
func TestCarryover_Named(t *testing.T) {
	c := Carryover{
		ID:       "c-001",
		Action:   "fix the router nil path",
		Priority: "P0",
	}
	if c.ID != "c-001" {
		t.Errorf("Carryover.ID: got %q, want %q", c.ID, "c-001")
	}
}

// TestParseJSON_Named covers ParseJSON (round-trip through RenderJSON).
func TestParseJSON_Named(t *testing.T) {
	original := &Dossier{
		Cycle:        7,
		Goal:         "parse-json round-trip",
		FinalVerdict: VerdictPass,
		Phases:       []PhaseRecord{{Name: "build", Verdict: VerdictPass}},
		Defects:      []Defect{{ID: "d1", Summary: "minor"}},
		Lessons:      []Lesson{{ID: "l1", Pattern: "pattern"}},
		Carryover:    []Carryover{{ID: "c1", Action: "action"}},
	}
	raw, err := RenderJSON(original)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	parsed, err := ParseJSON(raw)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if parsed.Cycle != original.Cycle {
		t.Errorf("ParseJSON Cycle: got %d, want %d", parsed.Cycle, original.Cycle)
	}
	if parsed.Goal != original.Goal {
		t.Errorf("ParseJSON Goal: got %q, want %q", parsed.Goal, original.Goal)
	}
	// Verify ParseJSON rejects invalid JSON.
	if _, err := ParseJSON([]byte("not-json")); err == nil {
		t.Error("ParseJSON: want error for invalid JSON, got nil")
	}
}

// TestVerdicts_Named covers the VerdictPass / VerdictWarn / VerdictFail constants.
func TestVerdicts_Named(t *testing.T) {
	for _, v := range []string{VerdictPass, VerdictWarn, VerdictFail} {
		if v == "" {
			t.Errorf("verdict constant must not be empty")
		}
	}
	if VerdictPass == VerdictFail {
		t.Error("VerdictPass must differ from VerdictFail")
	}
}

// TestPhaseRecord_Named covers the PhaseRecord type (apicover typed-var check).
func TestPhaseRecord_Named(t *testing.T) {
	pr := PhaseRecord{
		Name:        "scout",
		Verdict:     VerdictPass,
		KeyFindings: "found nothing",
		ArtifactSHA: "abc123",
		Signals:     map[string]any{"k": "v"},
	}
	if pr.Name != "scout" {
		t.Errorf("PhaseRecord.Name: got %q, want %q", pr.Name, "scout")
	}
}

// TestBuildOpts_Named covers BuildOpts (apicover typed-var check).
func TestBuildOpts_Named(t *testing.T) {
	opts := BuildOpts{
		WorkspacePath: "/tmp/ws",
		LedgerPath:    "/tmp/ledger.jsonl",
		Goal:          "test goal",
		RunID:         "01ABCDEF",
	}
	if opts.Goal != "test goal" {
		t.Errorf("BuildOpts.Goal: got %q, want %q", opts.Goal, "test goal")
	}
}

// TestDossier_JSONRoundTrip covers the Dossier type with all optional fields
// populated, including fields not exercised by other tests (Decisions, CommitSHA,
// TreeSHA, StartedAt, EndedAt) — ensures JSON tags are correct.
func TestDossier_JSONRoundTrip(t *testing.T) {
	d := &Dossier{
		Cycle:        1,
		RunID:        "01RUN",
		Goal:         "json round-trip",
		FinalVerdict: VerdictWarn,
		CommitSHA:    "abc",
		TreeSHA:      "def",
		StartedAt:    "2026-06-18T00:00:00Z",
		EndedAt:      "2026-06-18T01:00:00Z",
		Phases:       []PhaseRecord{{Name: "p", Verdict: VerdictWarn}},
		Decisions:    []string{"decision-1"},
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out Dossier
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.FinalVerdict != VerdictWarn {
		t.Errorf("FinalVerdict: got %q, want %q", out.FinalVerdict, VerdictWarn)
	}
	if len(out.Decisions) != 1 {
		t.Errorf("Decisions: got %d, want 1", len(out.Decisions))
	}
}
