package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type brakeOrch struct {
	noSignals
	evolveDir string
	engageAt  int
	calls     int
}

func (o *brakeOrch) RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error) {
	o.calls++
	if o.calls == o.engageAt {
		if err := os.WriteFile(paths.LoopStopPath(o.evolveDir), nil, 0o644); err != nil {
			return core.CycleResult{}, err
		}
	}
	return core.CycleResult{Cycle: 5 + o.calls, FinalVerdict: core.VerdictPASS}, nil
}

func (o *brakeOrch) RunCycleFromPhase(ctx context.Context, req core.CycleRequest, _ *core.ResumePoint) (core.CycleResult, error) {
	return o.RunCycle(ctx, req)
}

func runBrakeLoop(t *testing.T, projectRoot string, orch *brakeOrch, extra ...string) (rc int, stdout, stderr string) {
	t.Helper()
	defer installStubDeps(t, &fixtures.FakeStorage{}, newFakeLedger())()
	prevOrch := loopOrchOverride
	loopOrchOverride = orch
	defer func() { loopOrchOverride = prevOrch }()
	var out, errb bytes.Buffer
	args := append([]string{"--project-root", projectRoot, "--evolve-dir", orch.evolveDir, "--goal-text", "brake goal", "--cycles", "3"}, extra...)
	rc = runLoop(args, nil, &out, &errb)
	return rc, out.String(), errb.String()
}

func TestRunLoop_ABrakeEngagedMidRunEndsTheBatchAfterTheRunningIteration(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5)
	gcCalls, _ := installGCHookSpy(t, evolveDir)
	orch := &brakeOrch{evolveDir: evolveDir, engageAt: 1}

	rc, stdout, stderr := runBrakeLoop(t, projectRoot, orch)

	if orch.calls != 1 {
		t.Fatalf("the brake engaged during iteration 1 must stop the run before iteration 2; iterations run = %d\n%s", orch.calls, stderr)
	}
	if rc != 0 || !strings.Contains(stdout, `"stop_reason": "loop_operator_brake"`) {
		t.Errorf("a clean stop with the brake's reason: rc=%d stdout=%s", rc, stdout)
	}
	if !strings.Contains(stderr, "[loop] operator brake "+paths.LoopStopPath(evolveDir)) || !strings.Contains(stderr, "evolve loop-stop --release") {
		t.Errorf("one [loop] line names the brake file and its release: %s", stderr)
	}
	if last := (*gcCalls)[len(*gcCalls)-1]; last.workspace != filepath.Join(gcManifestDir(evolveDir), "batch-end") {
		t.Errorf("the batch closeout (finalize, batch-end sweep, emit) still runs: %+v", *gcCalls)
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "cycle-state.json")); !os.IsNotExist(err) {
		t.Errorf("finalize cleared the completed cycle's marker: %v", err)
	}
}

func TestRunLoop_ABrakePresentAtLaunchRunsZeroIterations(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5)
	if err := os.WriteFile(paths.LoopStopPath(evolveDir), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	orch := &brakeOrch{evolveDir: evolveDir}

	rc, stdout, stderr := runBrakeLoop(t, projectRoot, orch)

	if orch.calls != 0 || rc != 0 || !strings.Contains(stdout, `"stop_reason": "loop_operator_brake"`) {
		t.Fatalf("a brake at launch runs no iteration: calls=%d rc=%d stdout=%s\n%s", orch.calls, rc, stdout, stderr)
	}
}

func TestRunLoopChain_TheBrakeStopsTheRunningBatchAtItsWaveBoundaryAndTheChainAtItsBatchBoundary(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5)
	u13WriteJSON(t, filepath.Join(evolveDir, "inbox", "pending.json"), map[string]any{"id": "pending"})
	orch := &brakeOrch{evolveDir: evolveDir, engageAt: 1}

	_, stdout, stderr := runBrakeLoop(t, projectRoot, orch, "--until-inbox-empty")

	if orch.calls != 1 {
		t.Fatalf("the chained batch stops at its next wave boundary: iterations run = %d\n%s", orch.calls, stderr)
	}
	if !strings.Contains(stdout, `"stop_reason": "loop_operator_brake"`) || !strings.Contains(stdout, `"chain_stop_reason": "chain_operator_brake"`) || strings.Contains(stderr, "batch 2/") {
		t.Errorf("the batch ends on the brake, then the chain stops at its batch boundary without a second batch:\n%s\n%s", stdout, stderr)
	}
}

func TestLoopStop_EngagesAndReleasesTheBrake(t *testing.T) {
	root := t.TempDir()
	evolveDir := paths.EvolveDirOf(root)
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if rc := runLoopStop([]string{"--project-root", root}, nil, &out, &errb); rc != 0 || !loopchain.BrakeEngaged(evolveDir) {
		t.Fatalf("engage: rc=%d engaged=%v %s", rc, loopchain.BrakeEngaged(evolveDir), errb.String())
	}
	raw, err := os.ReadFile(paths.LoopStopPath(evolveDir))
	if err != nil {
		t.Fatal(err)
	}
	if _, perr := time.Parse(time.RFC3339, strings.TrimSpace(string(raw))); perr != nil || !strings.HasSuffix(string(raw), "\n") {
		t.Errorf("the brake file holds one timestamp line: %q %v", raw, perr)
	}
	if !strings.Contains(out.String(), paths.LoopStopPath(evolveDir)) || !strings.Contains(out.String(), "finishes its current wave") || !strings.Contains(out.String(), "evolve loop-stop --release") {
		t.Errorf("engage says what happens next: %s", out.String())
	}
	out.Reset()
	if rc := runLoopStop([]string{"--project-root", root, "--release"}, nil, &out, &errb); rc != 0 || loopchain.BrakeEngaged(evolveDir) || !strings.Contains(out.String(), "released") {
		t.Fatalf("release: rc=%d engaged=%v %s %s", rc, loopchain.BrakeEngaged(evolveDir), out.String(), errb.String())
	}
	out.Reset()
	if rc := runLoopStop([]string{"--project-root", root, "--release"}, nil, &out, &errb); rc != 0 || !strings.Contains(out.String(), "no brake") {
		t.Errorf("releasing an absent brake is a no-op: rc=%d %s", rc, out.String())
	}
}

func TestLoopStop_ResolvesTheRootLikeSyncMainAndRefusesStrayArguments(t *testing.T) {
	root := t.TempDir()
	evolveDir := paths.EvolveDirOf(root)
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var out, errb bytes.Buffer
	if rc := runLoopStop([]string{"release"}, nil, &out, &errb); rc == 0 || loopchain.BrakeEngaged(evolveDir) {
		t.Fatalf("a positional argument is refused and engages nothing: rc=%d %s", rc, errb.String())
	}
	if rc := runLoopStop(nil, nil, &out, &errb); rc != 0 || !loopchain.BrakeEngaged(evolveDir) {
		t.Fatalf("$EVOLVE_PROJECT_ROOT names the root: rc=%d %s", rc, errb.String())
	}
	errb.Reset()
	if rc := runLoopStop([]string{"--project-root", t.TempDir()}, nil, &out, &errb); rc != 1 || !strings.Contains(errb.String(), "evolve loop-stop:") {
		t.Errorf("a root without .evolve fails loudly: rc=%d %s", rc, errb.String())
	}
	if c := lookupCommand("loop-stop"); c == nil {
		t.Error("loop-stop is in the command table")
	}
}
