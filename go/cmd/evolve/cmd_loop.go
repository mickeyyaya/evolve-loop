package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	_ "github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

var validStrategies = map[string]struct{}{
	"balanced":     {},
	"innovate":     {},
	"harden":       {},
	"repair":       {},
	"ultrathink":   {},
	"autoresearch": {},
}

type loopConfig struct {
	ProjectRoot       string            `json:"project_root"`
	EvolveDir         string            `json:"evolve_dir"`
	GoalHash          string            `json:"goal_hash"`
	GoalText          string            `json:"goal_text,omitempty"`
	GoalSource        string            `json:"goal_source,omitempty"`
	Strategy          string            `json:"strategy"`
	MaxCycles         int               `json:"max_cycles"`
	MaxCyclesExplicit bool              `json:"max_cycles_explicit,omitempty"`
	Resume            bool              `json:"resume,omitempty"`
	AutoResume        bool              `json:"auto_resume,omitempty"`
	Reset             bool              `json:"reset,omitempty"`
	Fingerprint       string            `json:"fingerprint,omitempty"`
	ConsensusAudit    bool              `json:"consensus_audit,omitempty"`
	DryRun            bool              `json:"dry_run,omitempty"`
	ForceFresh        bool              `json:"force_fresh,omitempty"`
	SkipPreflight     bool              `json:"skip_preflight,omitempty"`
	SkipPreflightBoot bool              `json:"skip_preflight_boot,omitempty"`
	BypassPolicy      bool              `json:"bypass_policy,omitempty"`
	ChainMode         bool              `json:"chain_mode,omitempty"`
	PerAgentCLI       map[string]string `json:"per_agent_cli,omitempty"`
	PerAgentModel     map[string]string `json:"per_agent_model,omitempty"`
	PreflightOnly     bool              `json:"preflight_only,omitempty"`
	Detach            bool              `json:"detach,omitempty"`
	Relaunch          bool              `json:"relaunch,omitempty"`
	LogPath           string            `json:"log_path,omitempty"`
	DetachArgv        []string          `json:"-"`
	JSON              bool              `json:"-"`
	ResumeWaves       int               `json:"-"`
	HandedOff         bool              `json:"-"`
	announce          loopAnnouncer
	retire            func()
}

func emitSignalStop(stdout, stderr io.Writer, lr *loopResult, cycle int) {
	signalStop(stdout, stderr, lr, fmt.Sprintf("at cycle %d — checkpointed", cycle))
}

func signalStop(stdout, stderr io.Writer, lr *loopResult, where string) {
	fmt.Fprintf(stderr, "[loop] received interrupt (SIGINT/SIGTERM) %s; resume with: evolve loop --resume\n", where)
	lr.StopReason = "signal"
	lr.emit(stdout)
}

func runLoop(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if statusArgs, ok := loopStatusArgs(args); ok {
		return runLoopStatus(statusArgs, stdout, stderr)
	}
	cfg, rc := parseLoopArgs(args, stderr)
	if rc != 0 {
		return rc
	}
	return loopLaunchFn(cfg, stdin, stdout, stderr)
}

var loopLaunchFn = launchParsedLoop

func launchParsedLoop(cfg loopConfig, stdin io.Reader, stdout, stderr io.Writer) int {
	if cfg.PreflightOnly || cfg.Detach {
		return runLoopFrontDoor(cfg, stdout, stderr)
	}
	cfg.ResumeWaves, cfg.HandedOff = takeReexecHandoff(cfg.EvolveDir, stderr)
	if rc := refuseInertGoalItems(cfg, stderr); rc != 0 {
		return rc
	}
	recordLaunchGoal(cfg, stderr)
	chainCfg := loadChainConfig(cfg.EvolveDir)
	cfg.ChainMode = cfg.ChainMode || chainCfg.Enabled

	if cfg.ChainMode && cfg.Resume {
		fmt.Fprintln(stderr, "[chain] --resume runs a single batch; chaining is not applied to a resumed cycle — relaunch with --until-inbox-empty once it completes")
	}
	if cfg.ChainMode && !cfg.DryRun && !cfg.Resume {
		return runLoopChain(cfg, chainCfg, stdin, stdout, stderr)
	}
	return runLoopBatch(cfg, stdin, stdout, stderr)
}

func runLoopFrontDoor(cfg loopConfig, stdout, stderr io.Writer) int {
	if rc := refuseInertGoalItems(cfg, stderr); rc != 0 {
		return rc
	}
	if cfg.PreflightOnly {
		return runLoopPreflightOnly(cfg, stdout, stderr)
	}
	return runLoopDetached(cfg, stdout, stderr)
}

func printLoopDryRun(cfg loopConfig, stdout io.Writer) int {
	buf, _ := json.MarshalIndent(map[string]any{
		"dry_run": true,
		"config":  cfg,
	}, "", "  ")
	fmt.Fprintln(stdout, string(buf))
	return 0
}

func runLoopBatch(cfg loopConfig, _ io.Reader, stdout, stderr io.Writer) (exit int) {
	maybePrintSetupNudge(stderr, cfg.EvolveDir)

	if pi, perr := plane.Classify(cfg.ProjectRoot); perr == nil {
		fmt.Fprintln(stderr, plane.BootLine(pi))
	} else {
		fmt.Fprintf(stderr, "[loop] WARN: plane classification failed (%v) — the ADR-0080 primary-checkout tripwire is dark this run\n", perr)
	}

	if cfg.DryRun {
		return printLoopDryRun(cfg, stdout)
	}

	runtime := startLoopBatchRuntime()
	defer runtime.close()
	ctx := runtime.ctx

	gcOrphanSessions("startup", stderr)

	deps := wireOrchestratorDepsFn(cfg.ProjectRoot, cfg.EvolveDir, stderr, routingRun{bypass: cfg.BypassPolicy, env: carriedEnv(os.Environ()), role: roleLoop, overrides: cfg.agentOverrides()})
	defer deps.closeSignals()
	if deps.RoutingErr != nil {
		fmt.Fprintf(stderr, "evolve loop: %v\n", deps.RoutingErr)
		return exitRoutingRefused
	}
	cfg = cfg.retiring(deps.closeSignals)
	finish := cfg.announcer()(deps.Signals, stderr)
	defer finish(&exit)
	var orch loopCycleRunner = deps.Orchestrator
	if loopOrchOverride != nil {
		orch = loopOrchOverride
	}
	wc := loadWorkflowConfig(cfg.EvolveDir)
	cycleclassify.SetHangClassifier(loadClassifyConfig(cfg.EvolveDir).HangClassifier)

	maintainBatchState(cfg, wc.AutoPrune, stderr)

	cycleEnv := carriedEnv(os.Environ())
	cycleCtx := buildCycleContext(cfg)

	lr := loopResult{StopReason: "max_cycles", classifyRoot: cfg.ProjectRoot}
	if last, lerr := readLastCycleNumber(ctx, deps.Storage); lerr == nil {
		lr.batchFirstCycle = last + 1
	}

	if cfg.Resume {
		return runResumeBatch(ctx, cfg, orch, deps.Ledger, deps.Signals, cycleEnv, cycleCtx, &lr, stdout, stderr)
	}

	exitCode, halt := prepareFreshBatch(ctx, cfg, deps, &lr, stdout, stderr)
	if halt {
		return exitCode
	}

	return (&loopBatchCoordinator{
		ctx:          ctx,
		cfg:          cfg,
		deps:         deps,
		orch:         orch,
		cycleEnv:     cycleEnv,
		cycleContext: cycleCtx,
		result:       &lr,
		workflow:     wc,
		stdout:       stdout,
		stderr:       stderr,
	}).run()
}

func isTaskLevelFailure(c cycleclassify.Classification) bool {
	return cycleoutcome.IsTaskLevelFailure(c)
}

func readCarryoverCount(statePath string) (count int, ok bool) {
	b, err := os.ReadFile(statePath)
	if err != nil {
		return 0, false
	}
	var s struct {
		CarryoverTodos []json.RawMessage `json:"carryoverTodos"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return 0, false
	}
	return len(s.CarryoverTodos), true
}

func finalizeCompletedCycle(cfg loopConfig, stderr io.Writer) {
	cleared, err := core.ClearCompletedCycleMarker(cfg.EvolveDir, core.FinalizeOptions{Now: time.Now, PidAlive: pidAlive})
	if err != nil {
		fmt.Fprintf(stderr, "[loop] finalize: %v\n", err)
		return
	}
	if cleared {
		fmt.Fprintf(stderr, "[loop] cleared completed cycle-state.json marker (clean exit) — no `evolve cycle reset` needed before the next launch\n")
	}
}
