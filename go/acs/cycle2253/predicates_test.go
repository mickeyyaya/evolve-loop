//go:build acs

package cycle2253

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const (
	weeklyLimitEscalation = "[auto-respond] escalation report written (pattern=exhausted reason=escalate)\n"
	unknownPromptStderr   = "[auto-respond] escalation report written (pattern=unknown_prompt reason=escalate)\n"
	pausedCycle           = 7
	laneItem              = "router-quota-item"
)

type stubCLIBridge struct{ stderr string }

func (b stubCLIBridge) Launch(context.Context, core.BridgeRequest) (core.BridgeResponse, error) {
	out := launchoutcome.Classify(launchoutcome.ExitUnknownPrompt, nil, b.stderr)
	return core.BridgeResponse{ExitCode: out.ExitCode, Stderr: b.stderr, CauseCode: out.CauseCode}, out.Err
}

func (stubCLIBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

type existingWorktree struct{ path string }

func (w existingWorktree) Create(string, int) (string, error) { return w.path, nil }
func (w existingWorktree) Cleanup(string, string) error       { return nil }

type journey struct {
	root    string
	err     error
	signals []signalcenter.Event
}

func routingConfig() config.RoutingConfig {
	return config.RoutingConfig{
		Stage:         config.StageAdvisory,
		Mode:          config.ModeDynamicLLM,
		Mandatory:     []string{"scout", "build", "audit", "ship"},
		Conditional:   map[string]config.CondRule{"tdd": {Field: "cycle_size", Op: "!=", Value: "trivial"}},
		MaxInsertions: 4,
		PhaseEnable:   map[string]config.Enable{},
	}
}

func committedRepo(t *testing.T) string {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Git("commit", "-q", "--allow-empty", "-m", "base")
	return repo.Dir
}

func runLaneCycle(t *testing.T, planner router.Planner) journey {
	t.Helper()
	root := committedRepo(t)
	evolveDir := filepath.Join(root, ".evolve")
	ws := core.RunWorkspacePath(root, pausedCycle)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, core.LaneScopeFile), []byte(`{"todo_ids":["`+laneItem+`"],"goal_hash":"c2253"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ipcenv.CycleStateFileKey, filepath.Join(ws, core.CycleStateFile))
	st := storage.New(evolveDir)
	if err := st.WriteState(context.Background(), core.State{LastCycleNumber: pausedCycle - 1}); err != nil {
		t.Fatal(err)
	}
	center := signalcenter.New()
	o := core.NewOrchestrator(st, &fixtures.FakeLedger{}, fixtures.BuildRunners(nil),
		core.WithRouting(routingConfig(), router.StaticPreset{}),
		core.WithPlanner(planner),
		core.WithSignalCenter(center),
		core.WithWorktreeProvisioner(existingWorktree{path: committedRepo(t)}))
	_, err := o.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: root, GoalHash: "c2253", DisableWorkspaceGuard: true})
	center.Flush()
	os.Remove(runlease.PathIn(ws))
	return journey{root: root, err: err, signals: center.Recent()}
}

func weeklyLimitRouter() router.Planner {
	return core.NewPhaseAdvisor(stubCLIBridge{stderr: weeklyLimitEscalation}, core.WithProposerCLI("claude-tmux"))
}

func TestC2253_001_RouterDispatchAtWeeklyLimitEndsTheCycleQuotaPausedNotSkippedUnknown(t *testing.T) {
	j := runLaneCycle(t, weeklyLimitRouter())

	if !errors.Is(j.err, core.ErrAllFamiliesExhausted) {
		t.Fatalf("RED: RunCycle err = %v, want the quota pause (ErrAllFamiliesExhausted): a router launch that exits 85 at the weekly limit must pause the cycle, not degrade to the static spine and seal SKIPPED_UNKNOWN", j.err)
	}
}

func TestC2253_002_CycleResumeAllPicksUpTheCycleThatPausedAtTheRouter(t *testing.T) {
	j := runLaneCycle(t, weeklyLimitRouter())

	paused, err := core.PausedLaneScopes(j.root, filepath.Join(j.root, ".evolve"), core.ResumeOptions{Log: os.Stderr})
	if err != nil {
		t.Fatalf("PausedLaneScopes: %v", err)
	}
	if len(paused) != 1 || paused[0].Cycle != pausedCycle || strings.Join(paused[0].Scope, ",") != laneItem {
		t.Fatalf("RED: the resume-all listing = %+v, want cycle %d with scope [%s] (run err: %v)", paused, pausedCycle, laneItem, j.err)
	}
	if paused[0].AutoMaxAttempts == 0 {
		t.Errorf("RED: the paused cycle carries no auto-resume budget: %+v", paused[0])
	}
}

func TestC2253_003_TheQuotaPauseSignalNamesTheQuotaAndTheExitSignalIsNotUnknownPrompt(t *testing.T) {
	out := launchoutcome.Classify(launchoutcome.ExitUnknownPrompt, nil, weeklyLimitEscalation)
	if out.Signal != launchoutcome.CodeExitQuotaWall || out.Signal == launchoutcome.CodeExitUnknownPrompt {
		t.Errorf("RED: exit 85 at the weekly limit projects onto %s, want %s", out.Signal, launchoutcome.CodeExitQuotaWall)
	}

	j := runLaneCycle(t, weeklyLimitRouter())

	var paused *signalcenter.Event
	for i := range j.signals {
		if j.signals[i].Code == core.CodeQuotaPaused {
			paused = &j.signals[i]
		}
	}
	if paused == nil {
		t.Fatalf("RED: no %s signal after a router launch at the weekly limit (run err: %v)", core.CodeQuotaPaused, j.err)
	}
	if got := paused.Fields["stop_reason"]; got != "quota-pause" {
		t.Errorf("RED: stop_reason = %q, want quota-pause", got)
	}
	if !strings.Contains(paused.Reason, "is on quota") {
		t.Errorf("RED: the pause reason does not name the quota: %q", paused.Reason)
	}
}

func TestC2253_004_ANonQuotaExit85AtTheRouterStillDegradesToTheStaticSpine(t *testing.T) {
	notAWall := core.NewPhaseAdvisor(stubCLIBridge{stderr: unknownPromptStderr}, core.WithProposerCLI("claude-tmux"))

	j := runLaneCycle(t, notAWall)

	if j.err != nil {
		t.Fatalf("RunCycle err = %v, want nil: an exit 85 that is not a usage-limit wall must not pause the cycle", j.err)
	}
	paused, err := core.PausedLaneScopes(j.root, filepath.Join(j.root, ".evolve"), core.ResumeOptions{})
	if err != nil || len(paused) != 0 {
		t.Errorf("PausedLaneScopes = %+v, %v, want none for a non-quota router failure", paused, err)
	}
	for _, e := range j.signals {
		if e.Code == core.CodeQuotaPaused {
			t.Errorf("a non-quota exit 85 emitted %s", core.CodeQuotaPaused)
		}
	}
}
