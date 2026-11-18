package dossier

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func wireOf(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	return out
}

func TestDefect_Named(t *testing.T) {
	w := wireOf(t, Defect{ID: "d-001", Severity: "HIGH", Summary: "test defect", Fix: "fix it"})
	for key, want := range map[string]string{"id": "d-001", "severity": "HIGH", "summary": "test defect", "fix": "fix it"} {
		if w[key] != want {
			t.Errorf("Defect wire %q = %v, want %q", key, w[key], want)
		}
	}
	if _, ok := wireOf(t, Defect{ID: "d"})["fix"]; ok {
		t.Error("Defect.Fix must be omitted when empty")
	}
}

func TestLesson_Named(t *testing.T) {
	w := wireOf(t, Lesson{ID: "l-001", Pattern: "recurring nil dereference", PreventiveAction: "add nil guard at boundary"})
	for key, want := range map[string]string{"id": "l-001", "pattern": "recurring nil dereference", "preventive_action": "add nil guard at boundary"} {
		if w[key] != want {
			t.Errorf("Lesson wire %q = %v, want %q", key, w[key], want)
		}
	}
	if _, ok := wireOf(t, Lesson{ID: "l"})["preventive_action"]; ok {
		t.Error("Lesson.PreventiveAction must be omitted when empty")
	}
}

func TestCarryover_Named(t *testing.T) {
	w := wireOf(t, Carryover{ID: "c-001", Action: "fix the router nil path", Priority: "P0"})
	for key, want := range map[string]string{"id": "c-001", "action": "fix the router nil path", "priority": "P0"} {
		if w[key] != want {
			t.Errorf("Carryover wire %q = %v, want %q", key, w[key], want)
		}
	}
	if _, ok := wireOf(t, Carryover{ID: "c"})["priority"]; ok {
		t.Error("Carryover.Priority must be omitted when empty")
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
	w := wireOf(t, PhaseRecord{Name: "scout", Verdict: VerdictPass, KeyFindings: "found nothing", ArtifactSHA: "abc123", Signals: map[string]any{"k": "v"}})
	for key, want := range map[string]string{"name": "scout", "verdict": VerdictPass, "key_findings": "found nothing", "artifact_sha": "abc123"} {
		if w[key] != want {
			t.Errorf("PhaseRecord wire %q = %v, want %q", key, w[key], want)
		}
	}
	signals, ok := w["signals"].(map[string]any)
	if !ok || signals["k"] != "v" {
		t.Errorf("PhaseRecord signals = %v, want k=v", w["signals"])
	}
}

func TestBuildOpts_Named(t *testing.T) {
	d, err := Build(3, BuildOpts{WorkspacePath: t.TempDir(), Goal: "test goal", RunID: "01ABCDEF", FinalVerdict: VerdictWarn})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.Cycle != 3 || d.Goal != "test goal" || d.RunID != "01ABCDEF" || d.FinalVerdict != VerdictWarn {
		t.Errorf("Build did not carry BuildOpts into the dossier: %+v", d)
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

func TestHome_Named(t *testing.T) {
	if Home != "knowledge-base/cycles" {
		t.Errorf("Home = %q, want knowledge-base/cycles", Home)
	}
}
