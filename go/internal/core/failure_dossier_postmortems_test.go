package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func touchFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func readDossierArtifact(t *testing.T, dir string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, failureDossierFile))
	if err != nil {
		t.Fatalf("%s must be written: %v", failureDossierFile, err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("%s is not valid JSON: %v", failureDossierFile, err)
	}
	return got
}

func TestFailureDossierPostmortems_NamesEachPostmortemFileSorted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeVerdicts(t, dir, "FAIL", "FAIL")
	touchFiles(t, dir,
		"build-attempt-2-postmortem.json",
		"audit-attempt-1-postmortem.json",
		"build-attempt-1-postmortem.json",
	)

	d := buildFailureDossier(CycleState{CycleID: 2249, WorkspacePath: dir}, VerdictFAIL, policy.DefaultSystemFailurePolicy())

	want := []string{"audit-attempt-1-postmortem.json", "build-attempt-1-postmortem.json", "build-attempt-2-postmortem.json"}
	if !reflect.DeepEqual(d.Postmortems, want) {
		t.Fatalf("Postmortems = %v, want %v", d.Postmortems, want)
	}
	if err := writeFailureDossier(dir, d); err != nil {
		t.Fatal(err)
	}
	got, _ := readDossierArtifact(t, dir)["postmortems"].([]any)
	if len(got) != len(want) {
		t.Fatalf("artifact postmortems = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("artifact postmortems[%d] = %v, want %s", i, got[i], w)
		}
	}
}

func TestFailureDossierPostmortems_NoPostmortemOmitsTheField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeVerdicts(t, dir, "FAIL", "FAIL")

	d := buildFailureDossier(CycleState{CycleID: 2249, WorkspacePath: dir}, VerdictFAIL, policy.DefaultSystemFailurePolicy())
	if len(d.Postmortems) != 0 {
		t.Fatalf("Postmortems = %v, want none", d.Postmortems)
	}
	if err := writeFailureDossier(dir, d); err != nil {
		t.Fatal(err)
	}
	if _, present := readDossierArtifact(t, dir)["postmortems"]; present {
		t.Error("artifact carries a postmortems key with no postmortem file in the workspace")
	}
}

func TestFailureDossierPostmortems_IgnoresMalformedNamesDirectoriesAndLinks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeVerdicts(t, dir, "FAIL", "FAIL")
	touchFiles(t, dir,
		"scout-attempt-1-postmortem.json",
		"postmortem.json",
		"build-postmortem.json",
		"build-attempt-x-postmortem.json",
		"build-attempt-1-postmortem.json.tmp",
		"build-attempt-1-postmortem.md",
		"-attempt-1-postmortem.json",
		"bad phase-attempt-1-postmortem.json",
		"target.json",
	)
	if err := os.Mkdir(filepath.Join(dir, "tdd-attempt-1-postmortem.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "target.json"), filepath.Join(dir, "audit-attempt-1-postmortem.json")); err != nil {
		t.Fatal(err)
	}

	d := buildFailureDossier(CycleState{CycleID: 2249, WorkspacePath: dir}, VerdictFAIL, policy.DefaultSystemFailurePolicy())

	want := []string{"scout-attempt-1-postmortem.json"}
	if !reflect.DeepEqual(d.Postmortems, want) {
		t.Errorf("Postmortems = %v, want only %v: a malformed name, a directory, a symlink and a temp file are not postmortem records", d.Postmortems, want)
	}
}

func TestFailureDossierPostmortems_AuditFailDecisionWritesThePathsToTheArtifact(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeVerdicts(t, dir, "FAIL", "FAIL")
	touchFiles(t, dir, "build-attempt-1-postmortem.json", "build-attempt-2-postmortem.json")
	o := floorOrchestrator(fixedNextStrategy{next: "end"})

	o.decideAfterAuditFail(CycleState{CycleID: 2249, RunID: "run-2249", WorkspacePath: dir})

	got, _ := readDossierArtifact(t, dir)["postmortems"].([]any)
	if len(got) != 2 || got[0] != "build-attempt-1-postmortem.json" || got[1] != "build-attempt-2-postmortem.json" {
		t.Errorf("failure-dossier.json postmortems = %v after decideAfterAuditFail, want both build postmortem files", got)
	}
}

func TestFailureDossierPostmortems_RetroFloorDecisionWritesThePathsToTheArtifact(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeVerdicts(t, dir, "FAIL", "FAIL")
	touchFiles(t, dir, "audit-attempt-1-postmortem.json")
	o := floorOrchestrator(fixedNextStrategy{next: "end"})

	o.applyFailureDecisionFloor(CycleState{CycleID: 2249, RunID: "run-2249", WorkspacePath: dir}, VerdictFAIL)

	got, _ := readDossierArtifact(t, dir)["postmortems"].([]any)
	if len(got) != 1 || got[0] != "audit-attempt-1-postmortem.json" {
		t.Errorf("failure-dossier.json postmortems = %v after applyFailureDecisionFloor, want the audit postmortem file", got)
	}
}
