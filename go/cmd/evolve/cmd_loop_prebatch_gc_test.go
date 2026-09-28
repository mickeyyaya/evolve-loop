package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func storageAtCycle5() *fixtures.FakeStorage {
	return &fixtures.FakeStorage{State: core.State{LastCycleNumber: 5}}
}

func runOneCycleBatch(t *testing.T, projectRoot, evolveDir string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "pre-batch gc goal",
		"--cycles", "1",
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d, want 0 (clean max_cycles completion); stderr=%s", rc, stderr.String())
	}
}

func TestC1735_002_PreBatchGCHookTargetsBatchOwnedDir(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5)

	calls, cycleRan := installGCHookSpy(t, evolveDir)
	defer installStubDeps(t, storageAtCycle5(), newFakeLedger())()

	prevOrch := loopOrchOverride
	loopOrchOverride = &batchEndOrch{ran: cycleRan}
	defer func() { loopOrchOverride = prevOrch }()

	runOneCycleBatch(t, projectRoot, evolveDir)
	if len(*calls) == 0 {
		t.Fatal("gcHookFn was never invoked")
	}
	first := (*calls)[0]
	if first.cycleHadRun {
		t.Fatalf("first gcHookFn call already saw a completed cycle (calls=%+v) — expected the pre-batch call first", *calls)
	}

	if next := cycleWorkspace(projectRoot, 6); first.workspace == next {
		t.Errorf("pre-batch gcHookFn workspace = %q is the next cycle's own run dir; archivePollutedWorkspace would archive it", first.workspace)
	}
	want := filepath.Join(gcManifestDir(evolveDir), "pre-batch")
	if first.workspace != want {
		t.Errorf("pre-batch gcHookFn workspace = %q, want %q", first.workspace, want)
	}
}

func TestC1735_003_NextCycleWorkspaceHasNoGCManifestsAfterPreBatchGC(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5)

	cycleRan := new(bool)
	prevOrch := loopOrchOverride
	loopOrchOverride = &batchEndOrch{ran: cycleRan}
	defer func() { loopOrchOverride = prevOrch }()
	defer installStubDeps(t, storageAtCycle5(), newFakeLedger())()

	runOneCycleBatch(t, projectRoot, evolveDir)

	nextCycleWorkspace := cycleWorkspace(projectRoot, 6)
	entries, err := os.ReadDir(nextCycleWorkspace)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReadDir(%q): %v", nextCycleWorkspace, err)
	}
	for _, e := range entries {
		if e.Name() == "gc-shadow-manifest.json" || e.Name() == "workspace-gc-manifest.json" {
			t.Errorf("next cycle workspace %q contains %q — archivePollutedWorkspace would archive it", nextCycleWorkspace, e.Name())
		}
	}

	preBatchManifest := filepath.Join(gcManifestDir(evolveDir), "pre-batch", "gc-shadow-manifest.json")
	if _, err := os.Stat(preBatchManifest); err != nil {
		t.Errorf("the pre-batch GC published no manifest at %q: %v", preBatchManifest, err)
	}
}

func TestC1735_006_PreBatchGCManifestSurvivesBatchEndGC(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5)

	type publication struct {
		dir      string
		manifest []byte
		readErr  error
	}
	var published []publication
	prevHook := gcHookFn
	gcHookFn = func(cfg loopConfig, workspace string, stderr io.Writer) {
		runGCHook(cfg, workspace, stderr)
		raw, err := os.ReadFile(filepath.Join(workspace, "gc-shadow-manifest.json"))
		published = append(published, publication{dir: workspace, manifest: raw, readErr: err})
	}
	t.Cleanup(func() { gcHookFn = prevHook })

	cycleRan := new(bool)
	prevOrch := loopOrchOverride
	loopOrchOverride = &batchEndOrch{ran: cycleRan}
	defer func() { loopOrchOverride = prevOrch }()
	defer installStubDeps(t, storageAtCycle5(), newFakeLedger())()

	runOneCycleBatch(t, projectRoot, evolveDir)

	if len(published) < 2 {
		t.Fatalf("gcHookFn ran %d time(s), want a pre-batch and a batch-end sweep", len(published))
	}
	preBatch, batchEnd := published[0], published[len(published)-1]
	if preBatch.readErr != nil {
		t.Fatalf("the pre-batch sweep's gc-shadow-manifest.json in %q is unreadable: %v", preBatch.dir, preBatch.readErr)
	}
	if len(preBatch.manifest) == 0 {
		t.Fatalf("the pre-batch sweep published no gc-shadow-manifest.json in %q", preBatch.dir)
	}
	if preBatch.dir == batchEnd.dir {
		t.Errorf("pre-batch and batch-end sweeps share %q, so the batch-end manifest overwrites the pre-batch evidence of record", preBatch.dir)
	}
	after, err := os.ReadFile(filepath.Join(preBatch.dir, "gc-shadow-manifest.json"))
	if err != nil {
		t.Fatalf("the pre-batch manifest did not survive the batch: %v", err)
	}
	if !bytes.Equal(after, preBatch.manifest) {
		t.Errorf("the pre-batch manifest in %q was rewritten by the batch-end sweep", preBatch.dir)
	}
}
