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

const judgmentTodoID = "cycle-5-judgment-plan-review"

func enforceRoot(t *testing.T, stage string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	policyJSON := `{"retrospectives":{"stage":"` + stage + `"}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func closeoutWithJudgmentTodo(t *testing.T, root string, errOut *bytes.Buffer) (CycleResult, error) {
	storage := &fakeStorage{state: State{CarryoverTodos: []CarryoverTodo{{ID: judgmentTodoID, Action: "the plan widens the lane scope"}}}}
	t.Helper()
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil),
		WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}), WithDossierDestination(DossierFilesOnly), WithStderr(errOut))
	return o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "retro-enforce"})
}

func TestRunCycle_RetrospectivesStageEnforceWritesThePairIntoThePendingDirNotTheWorkspace(t *testing.T) {
	t.Parallel()
	root := enforceRoot(t, "enforce")

	if _, err := closeoutWithJudgmentTodo(t, root, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	for _, ext := range []string{".md", ".yaml"} {
		if _, err := os.Stat(filepath.Join(recoveryretro.PendingDir(root), judgmentTodoID+ext)); err != nil {
			t.Errorf("enforce left no pending %s: %v", ext, err)
		}
	}
	inWorkspace, _ := filepath.Glob(filepath.Join(root, ".evolve", "runs", "cycle-*", recoveryretro.DirName, "*"))
	if len(inWorkspace) != 0 {
		t.Errorf("enforce also wrote %v into the workspace", inWorkspace)
	}
	if _, err := os.Stat(filepath.Join(root, recoveryretro.DocsHome)); err == nil {
		t.Errorf("the closeout wrote into %s; only the boundary publish may", recoveryretro.DocsHome)
	}
}

func TestRunCycle_RetrospectivesStageShadowLeavesThePendingDirAbsent(t *testing.T) {
	t.Parallel()
	root := enforceRoot(t, "shadow")

	if _, err := closeoutWithJudgmentTodo(t, root, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	inWorkspace, _ := filepath.Glob(filepath.Join(root, ".evolve", "runs", "cycle-*", recoveryretro.DirName, judgmentTodoID+".md"))
	if len(inWorkspace) != 1 {
		t.Errorf("shadow wrote %v into the workspace, want one pair", inWorkspace)
	}
	if _, err := os.Stat(recoveryretro.PendingDir(root)); err == nil {
		t.Errorf("shadow created %s", recoveryretro.PendingDir(root))
	}
}

func TestRunCycle_AnEnforceWriteFailureIsAWarnLineAndTheCloseoutStillSucceeds(t *testing.T) {
	t.Parallel()
	root := enforceRoot(t, "enforce")
	if err := os.WriteFile(recoveryretro.PendingDir(root), []byte("a file where the directory goes"), 0o644); err != nil {
		t.Fatal(err)
	}
	var errOut bytes.Buffer

	res, err := closeoutWithJudgmentTodo(t, root, &errOut)

	if err != nil || res.Cycle == 0 {
		t.Fatalf("RunCycle = %+v, %v; a retrospective write failure must not fail the closeout", res, err)
	}
	warn := errOut.String()
	if !strings.Contains(warn, "WARN") || !strings.Contains(strings.ToLower(warn), "retrospective") {
		t.Fatalf("stderr = %q, want a WARN line about the retrospective", warn)
	}
}
