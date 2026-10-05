package core

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
)

func recordingCheck(name string, calls *[]string, failures ...string) BuildFloorCheck {
	return BuildFloorCheck{Name: name, Run: func(_ context.Context, in ReviewInput) []string {
		*calls = append(*calls, name+"@"+in.Worktree)
		return failures
	}}
}

func TestBuildHandoffFloor_FailuresRunsEveryCheckInOrderAndNamesThem(t *testing.T) {
	var calls []string
	floor := BuildHandoffFloor{
		recordingCheck("first", &calls, "first: red"),
		recordingCheck("second", &calls),
		recordingCheck("third", &calls, "third: red", "third: also red"),
	}
	got := floor.Failures(context.Background(), ReviewInput{Phase: string(PhaseBuild), Worktree: "/wt"})
	want := []string{"first: red", "third: red", "third: also red"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Failures = %q, want every check's failures in check order %q", got, want)
	}
	if wantCalls := []string{"first@/wt", "second@/wt", "third@/wt"}; !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("every check runs once, in order, on the one input; calls = %q", calls)
	}
	if names := floor.Names(); !reflect.DeepEqual(names, []string{"first", "second", "third"}) {
		t.Fatalf("Names = %q", names)
	}
}

func TestBuildHandoffFloor_ReviewRejectsInTheFloorsOwnWords(t *testing.T) {
	var calls []string
	floor := BuildHandoffFloor{recordingCheck("only", &calls, "Explanation Documentation: cited path a.go is not in the Build diff")}
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: "/wt"}
	got := floor.Review(context.Background(), in)
	want := "build handoff floor: 1 deterministic check failure(s) — fix these exactly before handoff:\n  Explanation Documentation: cited path a.go is not in the Build diff"
	if got.Approve || !got.Retry || got.Reason != want {
		t.Fatalf("a rejection is the floor's reason, retried through the correction ladder: got %+v, want reason %q", got, want)
	}
	if res := floor.Review(context.Background(), ReviewInput{Phase: string(PhaseAudit)}); !res.Approve {
		t.Fatalf("the floor judges build handoffs only; audit got %+v", res)
	}
}

func TestWholeBuildHandoffFloor_RunsTheMandatoryExplanationFloorFirst(t *testing.T) {
	var calls []string
	composed := BuildHandoffFloor{recordingCheck("production", &calls)}
	whole := WholeBuildHandoffFloor(composed)
	want := append(MandatoryBuildHandoffFloor().Names(), "production")
	if !reflect.DeepEqual(whole.Names(), want) {
		t.Fatalf("WholeBuildHandoffFloor names = %q, want %q", whole.Names(), want)
	}
	if got := MandatoryBuildHandoffFloor().Names(); !reflect.DeepEqual(got, []string{"explanation-documentation"}) {
		t.Fatalf("MandatoryBuildHandoffFloor names = %q", got)
	}
	if len(composed) != 1 {
		t.Fatal("composing the whole floor must not mutate the composed checks")
	}
}

func TestMandatoryBuildHandoffFloor_LegacyCycleOwesNoExplanation(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.Activate(explanationdocs.CycleBinding{
		ProjectRoot: root, Workspace: workspace, Cycle: 42, RunID: "run-42", ContractVersion: explanationdocs.CurrentContractVersion,
	}); err != nil {
		t.Fatal(err)
	}
	in := ReviewInput{Phase: string(PhaseBuild), Cycle: 42, RunID: "run-42", ProjectRoot: root, Workspace: workspace}
	if got := MandatoryBuildHandoffFloor().Failures(context.Background(), in); len(got) != 0 {
		t.Fatalf("a cycle with no explanation contract owes no explanation, as the mandatory reviewer decides; got %q", got)
	}
	in.ExplanationDocumentationVersion = explanationdocs.CurrentContractVersion
	if got := MandatoryBuildHandoffFloor().Failures(context.Background(), in); len(got) == 0 || !strings.Contains(got[0], "Explanation Documentation") {
		t.Fatalf("an active contract without a sealed build must fail the explanation floor; got %q", got)
	}
}

func TestOrchestrator_BuildHandoffFloorNames_ListsEveryFloorCheckTheReviewerRuns(t *testing.T) {
	var calls []string
	composed := BuildHandoffFloor{recordingCheck("production", &calls), recordingCheck("document-solution", &calls)}
	o := NewOrchestrator(nil, nil, nil, WithReviewer(ChainReviewers(composed, noopReviewer{})))
	if got, want := o.BuildHandoffFloorNames(), WholeBuildHandoffFloor(composed).Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildHandoffFloorNames = %q, want the whole floor %q", got, want)
	}
	extra := BuildHandoffFloor{recordingCheck("cycle-only", &calls)}
	nested := NewOrchestrator(nil, nil, nil, WithReviewer(ChainReviewers(composed, ChainReviewers(noopReviewer{}, extra))))
	if got := nested.BuildHandoffFloorNames(); reflect.DeepEqual(got, WholeBuildHandoffFloor(composed).Names()) || got[len(got)-1] != "cycle-only" {
		t.Fatalf("a floor mounted anywhere in the chain is named, so a probe that lacks it cannot match; got %q", got)
	}
	if got := NewOrchestrator(nil, nil, nil).BuildHandoffFloorNames(); !reflect.DeepEqual(got, MandatoryBuildHandoffFloor().Names()) {
		t.Fatalf("an orchestrator with no optional reviewer still runs the mandatory floor; got %q", got)
	}
}

func TestReviewInputFor_ProjectsTheCycleBindingTheFloorReviews(t *testing.T) {
	cs := CycleState{
		CycleID: 1788, RunID: "01M3SPWPFVP5QQQYYCDBNPN0CT", ExplanationDocumentationVersion: 1,
		WorktreeBaseSHA: strings.Repeat("6", 40), WorkspacePath: "/p/.evolve/runs/cycle-1788", ActiveWorktree: "/p/.evolve/worktrees/cycle-1788",
	}
	got := ReviewInputFor(cs, PhaseBuild, "/p")
	want := ReviewInput{
		Cycle: 1788, RunID: cs.RunID, ExplanationDocumentationVersion: 1, Phase: string(PhaseBuild),
		WorktreeBaseSHA: cs.WorktreeBaseSHA, Workspace: cs.WorkspacePath, Worktree: cs.ActiveWorktree, ProjectRoot: "/p",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReviewInputFor = %+v\nwant %+v", got, want)
	}
}

func writeRunState(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, RunStateFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadRunCycleState_PrefersTheWorkspaceMirrorAndNamesAnUnreadableFile(t *testing.T) {
	t.Setenv("EVOLVE_CYCLE_STATE_FILE", "")
	evolveDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(evolveDir, CycleStateFile), []byte(`{"cycle_id":1,"phase":"build"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()
	writeRunState(t, ws, `{"cycle_id":1788,"phase":"build","run_id":"run-1788"}`)
	cs, err := ReadRunCycleState(ws, evolveDir)
	if err != nil || cs.CycleID != 1788 || cs.RunID != "run-1788" {
		t.Fatalf("the workspace mirror is the run's own state: cs=%+v err=%v", cs, err)
	}
	if cs, err := ReadRunCycleState("", evolveDir); err != nil || cs.CycleID != 1 {
		t.Fatalf("without a workspace the evolve dir's state file is read: cs=%+v err=%v", cs, err)
	}
	if _, err := ReadRunCycleState(t.TempDir(), evolveDir); err == nil || !strings.Contains(err.Error(), RunStateFile+" unreadable") {
		t.Fatalf("a missing mirror is an error naming the file, never a zero state: %v", err)
	}
	bad := t.TempDir()
	writeRunState(t, bad, `{not json`)
	if _, err := ReadRunCycleState(bad, evolveDir); err == nil || !strings.Contains(err.Error(), RunStateFile+" unparseable") {
		t.Fatalf("an unparseable mirror is an error naming the file: %v", err)
	}
}

func TestBuildHandoffProbe_Input_BindsThePersistedCycleOrSaysWhyNot(t *testing.T) {
	wt := t.TempDir()
	ws := t.TempDir()
	writeRunState(t, ws, `{"cycle_id":1788,"phase":"build","run_id":"run-1788","explanation_documentation_version":1,"worktree_base_sha":"abc","workspace_path":"`+ws+`","active_worktree":"`+wt+`"}`)
	probe := BuildHandoffProbe{Workspace: ws, EvolveDir: t.TempDir(), ProjectRoot: "/p", Worktree: wt}
	got, err := probe.Input()
	cs, _ := ReadRunCycleState(ws, "")
	if err != nil || !reflect.DeepEqual(got, ReviewInputFor(cs, PhaseBuild, "/p")) {
		t.Fatalf("a bound probe reviews exactly the input the floor reviews: got %+v err=%v", got, err)
	}
	other := probe
	other.Worktree = t.TempDir()
	got, err = other.Input()
	if err == nil || !strings.Contains(err.Error(), "binds worktree") {
		t.Fatalf("a state that binds another worktree must not be borrowed: err=%v", err)
	}
	if want := (ReviewInput{Phase: string(PhaseBuild), Workspace: ws, Worktree: other.Worktree, ProjectRoot: "/p"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("an unbound probe reviews the tree it was asked about: got %+v want %+v", got, want)
	}
	unbound := BuildHandoffProbe{EvolveDir: filepath.Join(t.TempDir(), ".evolve"), ProjectRoot: "/p", Worktree: wt}
	if got, err := unbound.Input(); err == nil || got.Cycle != 0 || got.Worktree != wt {
		t.Fatalf("no persisted state is reported, and the probe falls back to the unbound input: got %+v err=%v", got, err)
	}
}
