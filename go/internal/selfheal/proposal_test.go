package selfheal

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func lowDiagnosis(id string) phasecontract.Diagnosis {
	return phasecontract.Diagnosis{Schema: phasecontract.DiagnosisSchema, IncidentID: id, Hypothesis: "the audit pane waits on a prompt",
		Evidence: []string{"i1"}, Action: phasecontract.DiagnosisAction{Kind: phasecontract.ActionOpenRepairItem}, ExpectedEffect: "no stall", Confidence: phasecontract.ConfidenceLow}
}

func TestProposal_RecordsTheIncidentTheDiagnosisAndTheProposedAction(t *testing.T) {
	t.Parallel()
	inc := liveIncident(t, stallIncident)
	p := NewProposal(inc, "shadow", "agent-selfheal-orchestrator-x", t0).WithDiagnosis(lowDiagnosis(inc.ID))

	if p.Schema != ProposalSchema || p.IncidentID != inc.ID || p.Class != ClassStall || p.Tier != TierPipeline || p.Stage != "shadow" || p.RunLabel != "agent-selfheal-orchestrator-x" {
		t.Fatalf("header %+v", p)
	}
	if p.State != ProposalProposed || p.Diagnosis == nil || p.Action == nil || p.Action.Kind != phasecontract.ActionHaltAndEscalate {
		t.Fatalf("a low diagnosis is recorded with the halt-and-escalate action, got %+v", p)
	}
	refused := NewProposal(inc, "shadow", "", t0).Refused(errors.New("schema"))
	failed := NewProposal(inc, "shadow", "", t0).Failed(errors.New("dispatch"))
	if refused.State != ProposalRefused || refused.Error != "schema" || failed.State != ProposalFailed || failed.Error != "dispatch" || refused.Action != nil {
		t.Fatalf("refused %+v failed %+v", refused, failed)
	}
}

func TestPending_DiagnosesEachOpenIncidentOnceUntilItChangesAndCapsThePass(t *testing.T) {
	t.Parallel()
	incidents := liveRecords(t)
	done := map[string]Proposal{}
	first := Pending(incidents, done, 2)
	if len(first) != 2 || first[0].Severity != "INCIDENT" || first[1].Severity != "INCIDENT" {
		t.Fatalf("the pass takes the INCIDENT records first, got %v", ids(first))
	}
	for _, inc := range first {
		done[inc.ID] = NewProposal(inc, "shadow", "", t0)
	}
	if again := Pending(incidents, done, 8); len(again) != 6 || slices.ContainsFunc(again, func(i Incident) bool { _, ok := done[i.ID]; return ok }) {
		t.Fatalf("a diagnosed incident that did not change waits, got %v", ids(again))
	}
	changed := slices.Clone(incidents)
	i := slices.IndexFunc(changed, func(inc Incident) bool { return inc.ID == first[0].ID })
	changed[i].Recurrence++
	if got := Pending(changed, done, 8); !slices.ContainsFunc(got, func(inc Incident) bool { return inc.ID == first[0].ID }) {
		t.Errorf("an incident that recurred after its diagnosis is diagnosed again, got %v", ids(got))
	}
	failed := map[string]Proposal{first[1].ID: NewProposal(first[1], "shadow", "", t0).Failed(errors.New("x"))}
	if got := Pending(incidents, failed, 8); !slices.ContainsFunc(got, func(inc Incident) bool { return inc.ID == first[1].ID }) {
		t.Errorf("a failed run is tried again, got %v", ids(got))
	}
	closed := slices.Clone(incidents)
	for k := range closed {
		closed[k].State = StateClosed
	}
	if got := Pending(closed, map[string]Proposal{}, 8); len(got) != 0 {
		t.Errorf("a closed incident needs no diagnosis, got %v", ids(got))
	}
}

func ids(incidents []Incident) []string {
	var out []string
	for _, inc := range incidents {
		out = append(out, inc.ID)
	}
	return out
}

func TestProposals_RoundTripUnderTheIncidentDirectory(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	dir := ProposalsDir(evolveDir)
	if !strings.HasPrefix(dir, Dir(evolveDir)+string(filepath.Separator)) {
		t.Fatalf("the proposals live under the protected incident directory, got %s", dir)
	}
	if got, err := LoadProposals(dir); err != nil || len(got) != 0 {
		t.Fatalf("no directory is no proposal: %v %v", got, err)
	}
	inc := liveIncident(t, stallIncident)
	want := NewProposal(inc, "shadow", "run-1", t0).WithDiagnosis(lowDiagnosis(inc.ID))
	if err := WriteProposal(dir, want); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadProposals(dir)
	if err != nil || len(got) != 1 || got[inc.ID].Action.Kind != phasecontract.ActionHaltAndEscalate || !got[inc.ID].IncidentLastSeenAt.Equal(inc.LastSeenAt) {
		t.Fatalf("round trip: %v %+v", err, got)
	}
	if incidents, err := LoadIncidents(Dir(evolveDir)); err != nil || len(incidents) != 0 {
		t.Fatalf("the proposals directory is no incident record: %v %v", incidents, err)
	}
}

func TestProposals_Faults(t *testing.T) {
	t.Parallel()
	inc := liveIncident(t, stallIncident)
	t.Run("a torn record", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, inc.ID+".json"), []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadProposals(dir); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("a record that cannot be read", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.Symlink(filepath.Join(dir, "nowhere"), filepath.Join(dir, inc.ID+".json")); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadProposals(dir); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("a directory is no record", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, inc.ID+".json"), 0o755); err != nil {
			t.Fatal(err)
		}
		if got, err := LoadProposals(dir); err != nil || len(got) != 0 {
			t.Fatalf("got %v %v", got, err)
		}
	})
	t.Run("a directory that is a file", func(t *testing.T) {
		t.Parallel()
		file := filepath.Join(t.TempDir(), "f")
		if err := os.WriteFile(file, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadProposals(file); err == nil {
			t.Fatal("want an error")
		}
		if err := WriteProposal(filepath.Join(file, "proposals"), NewProposal(inc, "shadow", "", time.Time{})); err == nil {
			t.Fatal("want a write error")
		}
	})
}
