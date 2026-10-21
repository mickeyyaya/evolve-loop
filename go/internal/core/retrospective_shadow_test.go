package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recoveryretro"
)

func TestRunCycle_TheCloseoutWritesTheShadowRetrospectiveIntoTheWorkspace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	storage := &fakeStorage{state: State{CarryoverTodos: []CarryoverTodo{{ID: "cycle-5-judgment-plan-review", Action: "the plan widens the lane scope"}}}}
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil),
		WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}), WithDossierDestination(DossierFilesOnly))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "retro-shadow"})

	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	docs, _ := filepath.Glob(filepath.Join(root, ".evolve", "runs", "cycle-*", recoveryretro.DirName, "cycle-5-judgment-plan-review.md"))
	if len(docs) != 1 || res.Cycle == 0 {
		t.Fatalf("the closeout of cycle %d wrote %v, want one shadow retrospective in the workspace", res.Cycle, docs)
	}
	if _, serr := os.Stat(filepath.Join(root, recoveryretro.DocsHome)); serr == nil {
		t.Fatalf("the shadow stage wrote into %s", recoveryretro.DocsHome)
	}
}

func TestWriteShadowRetrospectives_AFailureIsAWarnLineOnly(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	cr := &cycleRun{o: NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithStderr(&stderr)), cycle: 9}

	cr.writeShadowRetrospectives()

	if !strings.Contains(stderr.String(), "WARN cycle 9: the shadow retrospective is not complete") {
		t.Fatalf("stderr = %q, want the WARN line", stderr.String())
	}
}
