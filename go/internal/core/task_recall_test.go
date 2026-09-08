package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
