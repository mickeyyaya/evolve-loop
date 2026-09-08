//go:build integration

package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/research"
)

func TestDispatch_TaskRecallWithoutFailureHistory(t *testing.T) {
	for _, resume := range []bool{false, true} {
		t.Run(map[bool]string{false: "fresh", true: "resume"}[resume], func(t *testing.T) {
			root, corpus := initGitWorktree(t), t.TempDir()
			if err := os.WriteFile(filepath.Join(corpus, "lessons.yaml"), []byte(`- id: inst-relevant
  pattern: checkpoint-restoration
  preventiveAction: Preserve checkpoint identity before replay.
  confidence: 0.9
- id: inst-unrelated
  pattern: banana-yellow
  preventiveAction: Peel bananas.
  confidence: 0.9
`), 0600); err != nil {
				t.Fatal(err)
			}
			runners := buildRunners(nil)
			st := &fakeStorage{}
			if resume {
				st.cycleState = CycleState{CycleID: 1, Phase: string(PhaseBuild), WorkspacePath: RunWorkspacePath(root, 1)}
			}
			o := NewOrchestrator(st, &fakeLedger{}, runners, WithKB(research.NewFileKB([]string{corpus})))
			req := CycleRequest{ProjectRoot: root, DisableWorkspaceGuard: true, Context: map[string]string{"goal": "checkpoint restoration"}}
			var err error
			if resume {
				_, err = o.RunCycleFromPhase(context.Background(), req, &ResumePoint{Phase: string(PhaseBuild), CycleID: 1})
			} else {
				_, err = o.RunCycle(context.Background(), req)
			}
			if err != nil {
				t.Fatal(err)
			}
			phases := []Phase{PhaseBuild, PhaseAudit}
			if !resume {
				phases = append(phases, PhaseScout, PhaseTDD)
			}
			for _, p := range phases {
				fr := runners[p].(*fakeRunner)
				if len(fr.requests) == 0 {
					t.Fatalf("%s not dispatched", p)
				}
				got := fr.requests[0].Context["recalled_lessons"]
				if !strings.Contains(got, "inst-relevant") || strings.Contains(got, "inst-unrelated") {
					t.Errorf("%s recall = %q; want only relevant durable lesson with no failure history", p, got)
				}
			}
		})
	}
}
