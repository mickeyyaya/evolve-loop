package phasecmd

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseobserver"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func RunPhaseObserver(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	a, rc, handled := parseObserverArgs(args, stdout, stderr)
	if handled {
		return rc
	}

	shutdown := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGUSR1, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		close(shutdown)
	}()

	signals := observerSignalCenter(stderr)
	defer signals.Flush()
	pol := observerPolicy(a, signals)
	return phaseobserver.Run(observerConfig(a, pol, shutdown, signals), "", stderr)
}

const CodePolicyLoadFailed signalcenter.Code = "OBSERVER_POLICY_LOAD_FAILED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleObserver, CodePolicyLoadFailed, ".evolve/policy.json exists but could not be read or decoded when `evolve phase-observer` or `evolve phase-watchdog` resolved its settings (absence is silent); the poll, stall, nudge and watchdog thresholds and the --enforce stall chain's artifact extends fall back to the compiled defaults; fields.step=policy, path")
}

func observerPolicy(a observerArgs, signals *signalcenter.Center) policy.Policy {
	pol, path, err := loadPolicy()
	if err != nil {
		signals.Emit(signalcenter.Event{
			Cycle: a.cycle, Phase: a.pos[3], Module: signalcenter.ModuleObserver, Origin: "RunPhaseObserver",
			Kind: signalcenter.KindObserverWarning, Severity: signalcenter.SeverityWarn, Code: CodePolicyLoadFailed,
			Reason: "policy load failed (using compiled observer defaults): " + err.Error(),
			Fields: map[string]string{"step": "policy", "path": path},
		})
	}
	return pol
}

type observerArgs struct {
	enforce bool
	scope   phaseobserver.Scope
	pos     []string
	pgid    int
	cycle   int
}

func parseObserverArgs(args []string, stdout, stderr io.Writer) (a observerArgs, rc int, handled bool) {
	a.scope = phaseobserver.ScopePhase
	for _, arg := range args {
		switch {
		case arg == "--help" || arg == "-h":
			fmt.Fprintln(stdout, "Usage: evolve phase-observer [--enforce] [--scope=cycle|phase] \\")
			fmt.Fprintln(stdout, "       <workspace> <pgid> <cycle> <phase> <agent> [cycle-state]")
			return a, 0, true
		case arg == "--enforce":
			a.enforce = true
		case arg == "--scope=cycle":
			a.scope = phaseobserver.ScopeCycle
		case arg == "--scope=phase":
			a.scope = phaseobserver.ScopePhase
		case strings.HasPrefix(arg, "--scope="):
			fmt.Fprintf(stderr, "[phase-observer] unknown --scope value: %s\n", arg)
			return a, phaseobserver.ExitInvalidArgs, true
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(stderr, "[phase-observer] unknown flag: %s\n", arg)
			return a, phaseobserver.ExitInvalidArgs, true
		default:
			a.pos = append(a.pos, arg)
		}
	}
	if len(a.pos) < 5 {
		fmt.Fprintln(stderr, "[phase-observer] usage: phase-observer [--enforce] [--scope=...] <workspace> <pgid> <cycle> <phase> <agent> [cycle-state]")
		return a, phaseobserver.ExitInvalidArgs, true
	}
	pgid, err := strconv.Atoi(a.pos[1])
	if err != nil || pgid <= 0 {
		fmt.Fprintf(stderr, "[phase-observer] pgid must be a positive integer, got: %s\n", a.pos[1])
		return a, phaseobserver.ExitInvalidArgs, true
	}
	a.pgid = pgid
	a.cycle, _ = strconv.Atoi(a.pos[2])
	return a, 0, false
}

func observerConfig(a observerArgs, pol policy.Policy, shutdown <-chan struct{}, signals *signalcenter.Center) phaseobserver.Config {
	cycleState := ""
	if len(a.pos) > 5 {
		cycleState = a.pos[5]
	}
	cfg := observerEnvConfig(pol)
	cfg.Workspace = a.pos[0]
	cfg.SubagentPGID = a.pgid
	cfg.Cycle = a.cycle
	cfg.Phase = a.pos[3]
	cfg.Agent = a.pos[4]
	cfg.CycleState = cycleState
	cfg.Scope = a.scope
	cfg.Enforce = a.enforce
	cfg.ShutdownSig = shutdown
	cfg.StallPolicy = resolveStallPolicy(a.enforce, pol)
	cfg.ProcessAlive = phaseobserver.DefaultProcessAlive
	cfg.Signals = func() *signalcenter.Center { return signals }
	return cfg
}

func observerSignalCenter(w io.Writer) *signalcenter.Center {
	c := signalcenter.New(signalcenter.WithPID(os.Getpid()))
	c.Subscribe(signalcenter.ConsoleSink(w))
	return c
}

func observerEnvConfig(pol policy.Policy) phaseobserver.Config {
	cfg := pol.ObserverConfig()
	return phaseobserver.Config{
		PollS:     *cfg.PollS,
		StallS:    *cfg.StallS,
		NudgeS:    *cfg.NudgeS,
		NudgeBody: cfg.NudgeBody,
		EOFGraceS: cfg.EOFGraceS,
	}
}

func loadPolicy() (policy.Policy, string, error) {
	path := filepath.Join(os.Getenv("EVOLVE_PROJECT_ROOT"), ".evolve", "policy.json")
	pol, err := policy.Load(path)
	return pol, path, err
}

const envIPCPhaseRecoveryStage = "EVOLVE_" + "PHASE_RECOVERY_STAGE" // SSOT IPC-protocol-allowed

func resolveStallPolicy(enforce bool, pol policy.Policy) recovery.StallPolicy {
	if !enforce && strings.ToLower(strings.TrimSpace(os.Getenv(envIPCPhaseRecoveryStage))) != "enforce" {
		return nil
	}
	return recovery.NewChainStallPolicy(pol.BridgeConfig().ArtifactMaxExtends)
}
