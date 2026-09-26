package dossier

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

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
	if _, err := ParseJSON([]byte("not-json")); err == nil {
		t.Error("ParseJSON: want error for invalid JSON, got nil")
	}
}

func TestFailureRecord_Named(t *testing.T) {
	fr := FailureRecord{
		Fingerprint: "audit|gate-block|ab12cd34ef56",
		PreClass:    "gate-block",
		Reasons:     []string{"EGPS floor blocked ship: red_count=1"},
	}
	raw, err := json.Marshal(&Dossier{
		Cycle:        1,
		Goal:         "failure-record round-trip",
		FinalVerdict: VerdictFail,
		Phases:       []PhaseRecord{{Name: "audit", Verdict: VerdictFail}},
		Defects:      []Defect{{ID: "d1", Summary: "s"}},
		Carryover:    []Carryover{{ID: "c1", Action: "a"}},
		Failure:      &fr,
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out Dossier
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Failure == nil || out.Failure.Fingerprint != fr.Fingerprint ||
		out.Failure.PreClass != fr.PreClass || len(out.Failure.Reasons) != 1 {
		t.Errorf("FailureRecord round-trip: got %+v, want %+v", out.Failure, fr)
	}
}

func TestBuildOpts_SystemFailureNamed(t *testing.T) {
	sig := &cyclestate.SystemFailureSignal{Category: "landing-lost", Level: "system", Evidence: "ship ran but did not land"}
	d, err := Build(1, BuildOpts{WorkspacePath: t.TempDir(), Goal: "preserve system failure", SystemFailure: sig})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.SystemFailure == nil || *d.SystemFailure != *sig {
		t.Errorf("Dossier.SystemFailure = %+v, want %+v", d.SystemFailure, sig)
	}
}

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

func TestSchemaVersionAndSkipEvidence_Named(t *testing.T) {
	d := &Dossier{
		SchemaVersion: CurrentSchemaVersion,
		SkippedPhases: []cyclestate.SkippedPhase{{Phase: "retro", Reason: "abnormal exit"}},
	}
	if CurrentSchemaVersion < 2 {
		t.Fatalf("CurrentSchemaVersion = %d, want >= 2", CurrentSchemaVersion)
	}
	if got := PhaseSkipEvidence(t.TempDir(), d, "retro"); got != SkipEvidenceTrusted {
		t.Errorf("versioned retro skip = %q, want %q", got, SkipEvidenceTrusted)
	}
	d.SchemaVersion = 0
	if got := PhaseSkipEvidence(t.TempDir(), d, "retro"); got != SkipEvidenceUnverified {
		t.Errorf("legacy retro skip without a receipt = %q, want %q", got, SkipEvidenceUnverified)
	}
	for _, evidence := range []SkipEvidence{SkipEvidenceNone, SkipEvidenceTrusted, SkipEvidenceContradicted, SkipEvidenceUnverified} {
		if evidence == "" {
			t.Error("SkipEvidence constant must not be empty")
		}
	}
}

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

// Production reaches these accessors only through text/template reflection,
// which apicover's AST scan cannot see.
func TestCommitmentAccessors_NamedAndExercised(t *testing.T) {
	var unrecorded Dossier
	if unrecorded.HasCommitment() || unrecorded.CommitmentLine() != "" {
		t.Errorf("an unrecorded commitment must render nothing; got %q", unrecorded.CommitmentLine())
	}
	empty := []string{}
	stated := Dossier{Tasks: &empty}
	if !stated.HasCommitment() || !strings.Contains(stated.CommitmentLine(), "nothing") {
		t.Errorf("an explicit empty commitment must state itself; got %q", stated.CommitmentLine())
	}
	ids := []string{"alpha", "beta"}
	full := Dossier{Tasks: &ids}
	if line := full.CommitmentLine(); !strings.Contains(line, "alpha") || !strings.Contains(line, "beta") {
		t.Errorf("CommitmentLine = %q, want both ids", line)
	}
	var nilRecv *Dossier
	if nilRecv.HasCommitment() {
		t.Error("a nil receiver must report no commitment, not panic")
	}
}
