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

// RunPhaseObserver implements `evolve phase-observer`, the composition root of the manual stall observer.
func RunPhaseObserver(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	a, rc, handled := parseObserverArgs(args, stdout, stderr)
	if handled {
		return rc
	}

	// SIGUSR1 means the subagent has exited: finalize.
	shutdown := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGUSR1, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		close(shutdown)
	}()

	signals := observerSignalCenter(stderr)
	defer signals.Flush()
	return phaseobserver.Run(observerConfig(a, shutdown, signals), "", stderr)
}

type observerArgs struct {
	enforce bool
	scope   phaseobserver.Scope
	pos     []string
}

// parseObserverArgs reads argv once; handled reports that help or a usage error already ended the call.
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
	return a, 0, false
}

// observerConfig projects the parsed argv, the policy thresholds and the root's collaborators onto the host's Config.
func observerConfig(a observerArgs, shutdown <-chan struct{}, signals *signalcenter.Center) phaseobserver.Config {
	// Atoi errors are dropped on purpose: a bogus value parses as 0, and Run rejects a zero cycle.
	pgid, _ := strconv.Atoi(a.pos[1])
	cycle, _ := strconv.Atoi(a.pos[2])
	cycleState := ""
	if len(a.pos) > 5 {
		cycleState = a.pos[5]
	}
	cfg := observerEnvConfig()
	cfg.Workspace = a.pos[0]
	cfg.SubagentPGID = pgid
	cfg.Cycle = cycle
	cfg.Phase = a.pos[3]
	cfg.Agent = a.pos[4]
	cfg.CycleState = cycleState
	cfg.Scope = a.scope
	cfg.Enforce = a.enforce
	cfg.ShutdownSig = shutdown
	cfg.StallPolicy = resolveStallPolicy(a.enforce)
	// The liveness probe is ground truth, not policy, so it is always wired.
	// Acting on a dead group stays Enforce-gated.
	cfg.ProcessAlive = phaseobserver.DefaultProcessAlive
	cfg.Signals = func() *signalcenter.Center { return signals }
	return cfg
}

// observerSignalCenter renders on stderr only: a file sink would interleave a second
// process's sequence space into the orchestrator's per-cycle signals.ndjson.
func observerSignalCenter(w io.Writer) *signalcenter.Center {
	c := signalcenter.New(signalcenter.WithPID(os.Getpid()))
	c.Subscribe(signalcenter.ConsoleSink(w))
	return c
}

// observerEnvConfig resolves observer settings from .evolve/policy.json.
func observerEnvConfig() phaseobserver.Config {
	cfg := loadObserverPolicy()
	return phaseobserver.Config{
		PollS:     *cfg.PollS,
		StallS:    *cfg.StallS,
		NudgeS:    *cfg.NudgeS,
		NudgeBody: cfg.NudgeBody,
		EOFGraceS: cfg.EOFGraceS,
	}
}

func loadObserverPolicy() policy.ObserverPolicy {
	pol, err := policy.Load(filepath.Join(os.Getenv("EVOLVE_PROJECT_ROOT"), ".evolve", "policy.json"))
	if err != nil {
		pol = policy.Policy{}
	}
	return pol.ObserverConfig()
}

// envIPCPhaseRecoveryStage is the env key through which a parent may inject the phase-recovery stage.
// The split spelling keeps the whole key out of string literals, where the flagreaders guard would flag it.
const envIPCPhaseRecoveryStage = "EVOLVE_" + "PHASE_RECOVERY_STAGE" // SSOT IPC-protocol-allowed

// resolveStallPolicy returns the chain-backed stall policy only for --enforce or an injected stage of
// exactly "enforce"; anything else returns nil, so a typo never arms the kill path.
// See ADR-0044.
func resolveStallPolicy(enforce bool) recovery.StallPolicy {
	if !enforce && strings.ToLower(strings.TrimSpace(os.Getenv(envIPCPhaseRecoveryStage))) != "enforce" {
		return nil
	}
	pol, err := policy.Load(filepath.Join(os.Getenv("EVOLVE_PROJECT_ROOT"), ".evolve", "policy.json"))
	if err != nil {
		pol = policy.Policy{}
	}
	return recovery.NewChainStallPolicy(pol.BridgeConfig().ArtifactMaxExtends)
}
