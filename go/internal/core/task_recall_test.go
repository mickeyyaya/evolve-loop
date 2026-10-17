package core

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestAdvisorPlanInput_RecallsGoalWithoutHistory(t *testing.T) {
	t.Parallel()
	corpus := t.TempDir()
	if err := os.WriteFile(filepath.Join(corpus, "lesson.yaml"), []byte("- id: inst-goal\n  pattern: checkpoint\n  preventiveAction: Preserve checkpoint identity.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithKB(research.NewFileKB([]string{corpus})))
	in := o.advisorPlanInput(context.Background(), "start", router.RoutingSignals{}, CycleRequest{Context: map[string]string{"goal": "checkpoint"}}, State{}, CycleState{}, 1, nil, nil)
	if len(in.Lessons) != 1 || !strings.Contains(in.Lessons[0], "inst-goal") {
		t.Fatalf("advisor goal recall without history = %v", in.Lessons)
	}
}

func TestTaskRecall_ReplacesStaleMemoryWithoutMutatingCaller(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{name: "no relevant lessons"},
		{name: "lookup failure is visible", err: context.DeadlineExceeded, want: "Memory unavailable: context deadline exceeded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := &Orchestrator{kb: &fakeKB{err: tc.err}}
			base := map[string]string{"goal": "checkpoint", CtxKeyRecallMemory: "unrelated previous task"}
			got := o.seedTaskRecall(context.Background(), base, PhaseBuild, CycleState{}, t.TempDir())
			if got[CtxKeyRecallMemory] != tc.want || got["goal"] != "checkpoint" {
				t.Fatalf("recall context = %v; want recall %q and original goal", got, tc.want)
			}
			if base[CtxKeyRecallMemory] != "unrelated previous task" {
				t.Fatalf("caller context was mutated: %v", base)
			}
		})
	}
}

func TestTaskRecall_RecordsInjectedLessonIDsForTheDossier(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		lessons []research.Lesson
		want    []string
	}{
		{name: "empty store records nothing"},
		{name: "ids in lookup order", lessons: []research.Lesson{{ID: "L-a", Pattern: "p"}, {ID: "L-b", Pattern: "q"}}, want: []string{"L-a", "L-b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := t.TempDir()
			o := &Orchestrator{kb: &fakeKB{lessons: tc.lessons}}
			o.seedTaskRecall(context.Background(), map[string]string{"goal": "x"}, PhaseBuild, CycleState{WorkspacePath: ws}, t.TempDir())
			d, err := dossier.Build(3, dossier.BuildOpts{WorkspacePath: ws, Goal: "g", FinalVerdict: dossier.VerdictPass})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(d.RecalledLessons, tc.want) {
				t.Fatalf("dossier RecalledLessons = %v, want %v", d.RecalledLessons, tc.want)
			}
		})
	}
}

func TestTaskRecall_UnwritableRecordWarnsAndStillInjects(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	var warn strings.Builder
	o := &Orchestrator{kb: &fakeKB{lessons: []research.Lesson{{ID: "L-a", Pattern: "p"}}}, stderr: &warn}
	got := o.seedTaskRecall(context.Background(), map[string]string{"goal": "x"}, PhaseBuild, CycleState{WorkspacePath: blocker}, t.TempDir())
	if !strings.Contains(got[CtxKeyRecallMemory], "L-a") {
		t.Fatalf("recall not injected: %v", got)
	}
	if !strings.Contains(warn.String(), "recalled-lessons record") {
		t.Fatalf("no warning on stderr: %q", warn.String())
	}
}

func TestTaskRecall_QueryIncludesSelectedItemTitle(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeWSFile(t, ws, LaneScopeFile, `{"todo_ids":["a"]}`)
	item := filepath.Join(t.TempDir(), "a.json")
	if err := os.WriteFile(item, []byte(`{"id":"a","title":"zebra crossing","acceptance":["quokka"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	kb := &fakeKB{}
	o := &Orchestrator{kb: kb}
	o.seedTaskRecall(context.Background(), map[string]string{"goal": "g", "fleet_scope_paths": "a=" + item}, PhaseBuild, CycleState{WorkspacePath: ws}, t.TempDir())
	if got := strings.Join(kb.gotQuery.Keywords, " "); !strings.Contains(got, "zebra") || !strings.Contains(got, "quokka") {
		t.Fatalf("query keywords = %q, want the item title and acceptance", got)
	}
}
