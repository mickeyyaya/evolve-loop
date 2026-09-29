package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunCycle_AdoptionRetiresTheAncestorsOwnPredicatePackage(t *testing.T) {
	root, wt := initContinuationRepo(t, 83)
	ancestorPredicate := "go/acs/cycle83/predicates_test.go"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(wt, ancestorPredicate)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, ancestorPredicate), []byte("//go:build acs\n\npackage cycle83\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := stampedContinuation(t, root, wt, 83)
	if err := os.WriteFile(m.FindingsPath, []byte(`{"phase":"build","summary":"fence predicate"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	seedStampedInboxItem(t, root, 83, "task-a")
	runners := buildRunners(nil)
	buildR := runners[PhaseBuild].(*fakeRunner)
	probe := &worktreeProbeRunner{fakeRunner: buildR, probeFile: "prior_work.go", absentFile: ancestorPredicate}
	archiveProbe := &predicateArchiveProbe{worktreeProbeRunner: probe}
	runners[PhaseBuild] = archiveProbe
	runners[PhaseTriage] = &claimingTriageRunner{fakeRunner: runners[PhaseTriage].(*fakeRunner), root: root, taskID: "task-a"}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithContinuationResolver(productionResolver(t)))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	if buildR.calls == 0 {
		t.Fatal("build must have dispatched")
	}
	if !probe.sawFile {
		t.Error("the ancestor's prior work must stay in the adopted tree")
	}
	if !archiveProbe.sawArchived {
		t.Error("the ancestor's predicate package must be archived beside its explanation records (one retention rule, design A2), not dropped")
	}
	if probe.sawAbsentFile {
		t.Errorf("%s is the ancestor cycle's own predicate package: it must leave the adopted tree before Build, or the build floor runs a contract this cycle does not own (cycle 1764 failed on 1761's fence, diffed against 1761's base)", ancestorPredicate)
	}
}

type predicateArchiveProbe struct {
	*worktreeProbeRunner
	sawArchived bool
}

func (r *predicateArchiveProbe) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	if req.Worktree != "" {
		matches, _ := filepath.Glob(filepath.Join(req.Worktree, "docs", "private", "research", "archived-*", "superseded-predicate-packages", "cycle83", "predicates_test.go"))
		r.sawArchived = len(matches) == 1
	}
	return r.worktreeProbeRunner.Run(ctx, req)
}
