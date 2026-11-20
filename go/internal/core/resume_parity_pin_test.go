package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/outcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type scriptedBuildRunner struct {
	calls int
	fn    func(call int, req core.PhaseRequest) (core.PhaseResponse, error)
}

func (r *scriptedBuildRunner) Name() string { return string(core.PhaseBuild) }
func (r *scriptedBuildRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.calls++
	return r.fn(r.calls, req)
}

type rejectOnceReviewer struct{ calls int }

func (r *rejectOnceReviewer) Review(_ context.Context, _ core.ReviewInput) core.ReviewResult {
	r.calls++
	if r.calls == 1 {
		return core.ReviewResult{Approve: false, Retry: true, Reason: "exercise deliverable correction error"}
	}
	return core.ReviewResult{Approve: true}
}

func assertResumeAbortRecorded(t *testing.T, ws string, res core.CycleResult, phase core.Phase) {
	t.Helper()
	recordedPhase := false
	for _, p := range res.PhasesRun {
		if p == phase {
			recordedPhase = true
			break
		}
	}
	if !recordedPhase {
		t.Errorf("CycleResult.PhasesRun missing %s: %+v", phase, res.PhasesRun)
	}
	sidecarRaw, err := os.ReadFile(outcome.UsageSidecarPath(ws, string(phase)))
	if err != nil {
		t.Fatalf("read usage sidecar: %v", err)
	}
	var sc outcome.UsageSidecar
	if err := json.Unmarshal(sidecarRaw, &sc); err != nil {
		t.Fatalf("decode usage sidecar: %v", err)
	}
	if sc.AbortReason == "" {
		t.Errorf("usage sidecar for %s has empty abort_reason: %+v", phase, sc)
	}
	raw, err := os.ReadFile(filepath.Join(ws, "phase-timing.json"))
	if err != nil {
		t.Fatalf("read phase-timing.json: %v", err)
	}
	var entries []map[string]any
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("decode phase-timing.json: %v", err)
	}
	for _, entry := range entries {
		if entry["phase"] == string(phase) {
			if reason, _ := entry["abort_reason"].(string); reason != "" {
				return
			}
		}
	}
	t.Errorf("phase-timing.json missing non-empty abort_reason for %s: %s", phase, raw)
}

func newResumeExitWorkspace(t *testing.T, cycle int) (string, string) {
	t.Helper()
	projectRoot := t.TempDir()
	ws := filepath.Join(projectRoot, ".evolve", "runs", "cycle-exit")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	return projectRoot, ws
}

func TestRunCycleFromPhase_DeliverableReviewError_RecordsPhaseOutcome(t *testing.T) {
	t.Parallel()

	projectRoot, ws := newResumeExitWorkspace(t, 642)
	st := &recStorage{
		state: core.State{LastCycleNumber: 642},
		cs:    core.CycleState{CycleID: 642, WorkspacePath: ws},
	}
	runners := newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseBuild: &scriptedBuildRunner{
			fn: func(call int, req core.PhaseRequest) (core.PhaseResponse, error) {
				if call == 1 {
					return core.PhaseResponse{Phase: string(core.PhaseBuild), Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
				}
				return core.PhaseResponse{Phase: string(core.PhaseBuild)}, errors.New("correction dispatch crashed")
			},
		},
	})
	retries := policy.Policy{}.RetryConfig()
	retries.ContractCorrectionRetries = 1
	orch := core.NewOrchestrator(st, &fakeLedger{}, runners,
		core.WithReviewer(&rejectOnceReviewer{}), core.WithRetryConfig(retries))

	res, err := orch.RunCycleFromPhase(context.Background(),
		core.CycleRequest{ProjectRoot: projectRoot, GoalHash: "test-goal"},
		&core.ResumePoint{Phase: string(core.PhaseBuild), CycleID: 642})
	if err == nil {
		t.Fatal("deliverable review dispatch error on resume must return an error")
	}
	assertResumeAbortRecorded(t, ws, res, core.PhaseBuild)
}

func TestRunCycleFromPhase_PostBuildExplanationRefreshError_RecordsPhaseOutcome(t *testing.T) {
	t.Parallel()

	repo := gittest.Fixture(t)
	repo.Write(".gitignore", ".evolve/\n")
	repo.Git("add", ".gitignore")
	repo.Git("commit", "-q", "-m", "base")
	base := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	projectRoot := repo.Dir
	ws := filepath.Join(projectRoot, ".evolve", "runs", "cycle-643")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	binding := explanationdocs.CycleBinding{
		ProjectRoot:     projectRoot,
		Workspace:       ws,
		Cycle:           643,
		RunID:           "run-643",
		ContractVersion: explanationdocs.CurrentContractVersion,
	}
	if err := explanationdocs.Activate(binding); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	binding.Worktree = projectRoot
	binding.BaseSHA = base
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatalf("SealBuild: %v", err)
	}
	st := &recStorage{
		state: core.State{LastCycleNumber: 643},
		cs: core.CycleState{
			CycleID:                         643,
			RunID:                           "run-643",
			WorkspacePath:                   ws,
			ActiveWorktree:                  projectRoot,
			WorktreeBaseSHA:                 base,
			CompletedPhases:                 []string{string(core.PhaseBuild)},
			ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
		},
	}
	orch := core.NewOrchestrator(st, &fakeLedger{}, newRunners(nil))

	res, err := orch.RunCycleFromPhase(context.Background(),
		core.CycleRequest{ProjectRoot: projectRoot, GoalHash: "test-goal"},
		&core.ResumePoint{Phase: string(core.PhaseTDD), CycleID: 643})
	if err == nil {
		t.Fatal("post-Build explanation refresh error on resume must return an error")
	}
	assertResumeAbortRecorded(t, ws, res, core.PhaseTDD)
}
