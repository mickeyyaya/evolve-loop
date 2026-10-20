package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestRemediation_TheFixedBuildIsBoundAgainAfterTheHostRewritesItsReport(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Write("seed.txt", "seed\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "seed")
	runners := buildRunners(nil)
	runners[PhaseTDD] = &scriptedRunner{name: PhaseTDD, verdicts: []string{VerdictFAIL, VerdictPASS}}
	runners[PhaseBuild] = &scriptedRunner{name: PhaseBuild, verdicts: []string{VerdictPASS}}
	ledger := &fakeLedger{}
	wf := policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"tdd"}}
	o := NewOrchestrator(&fakeStorage{}, ledger, runners, WithWorkflowConfig(wf))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: repo.Dir, GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	builders := 0
	for _, e := range ledger.entries {
		if e.Role == "builder" {
			builders++
		}
	}
	if builders != 2 {
		t.Fatalf("builder bindings = %d, want 2: the remediation fix rewrites build-report.md, so it must be bound again", builders)
	}
}
