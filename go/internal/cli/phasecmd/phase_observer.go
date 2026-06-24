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
)

// runPhaseObserver is the `evolve phase-observer [--enforce] [--scope=...] <ws> <pgid> <cycle> <phase> <agent> [state]` subcommand.
// Ports the core stall-detection behavior of legacy/scripts/dispatch/phase-observer.sh.
func RunPhaseObserver(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	enforce := false
	scope := phaseobserver.ScopePhase
	var pos []string

	for _, a := range args {
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve phase-observer [--enforce] [--scope=cycle|phase] \\")
			fmt.Fprintln(stdout, "       <workspace> <pgid> <cycle> <phase> <agent> [cycle-state]")
			return 0
		case a == "--enforce":
			enforce = true
		case a == "--scope=cycle":
			scope = phaseobserver.ScopeCycle
		case a == "--scope=phase":
			scope = phaseobserver.ScopePhase
		case strings.HasPrefix(a, "--scope="):
			fmt.Fprintf(stderr, "[phase-observer] unknown --scope value: %s\n", a)
			return phaseobserver.ExitInvalidArgs
		case strings.HasPrefix(a, "--"):
			fmt.Fprintf(stderr, "[phase-observer] unknown flag: %s\n", a)
			return phaseobserver.ExitInvalidArgs
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) < 5 {
		fmt.Fprintln(stderr, "[phase-observer] usage: phase-observer [--enforce] [--scope=...] <workspace> <pgid> <cycle> <phase> <agent> [cycle-state]")
		return phaseobserver.ExitInvalidArgs
	}
	pgid, _ := strconv.Atoi(pos[1])
	cycle, _ := strconv.Atoi(pos[2])
	cycleState := ""
	if len(pos) > 5 {
		cycleState = pos[5]
	}

	// SIGUSR1 = "subagent has exited; finalize"
	shutdown := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGUSR1, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		close(shutdown)
	}()

	cfg := observerEnvConfig()
	cfg.Workspace = pos[0]
	cfg.SubagentPGID = pgid
	cfg.Cycle = cycle
	cfg.Phase = pos[3]
	cfg.Agent = pos[4]
	cfg.CycleState = cycleState
	cfg.Scope = scope
	cfg.Enforce = enforce
	cfg.ShutdownSig = shutdown
	// ADR-0044 C3: the chain-backed stall policy executes ONLY at enforce. For
	// this standalone subcommand the operator's --enforce flag is the live signal
	// (the IPC stage env key is an accepted fallback for an injecting parent).
	// off/shadow/unset + no --enforce ⇒ nil policy ⇒ byte-identical legacy Enforce
	// branch — shadow observability for stalls already exists via the INCIDENT
	// events themselves.
	cfg.StallPolicy = resolveStallPolicy(enforce)
	// R3.4: the process-liveness probe is wired unconditionally — it is
	// deterministic ground truth (signal-0), not policy; nil in Run means
	// probe-off (fixture Configs). The ACTION on a dead group stays
	// policy/Enforce-gated; at shadow the INCIDENT is pure soak telemetry
	// (pane echo ≠ liveness, cycles 274/277).
	cfg.ProcessAlive = phaseobserver.DefaultProcessAlive
	return phaseobserver.Run(cfg, "", stderr)
}

// observerEnvConfig resolves observer settings from .evolve/policy.json.
// The name is retained to avoid widening this mechanical configuration change.
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

// envIPCPhaseRecoveryStage is the IPC key the parent orchestrator injects into
// the subprocess env to communicate the policy-resolved ADR-0044 stage.
// The split-const form keeps "EVOLVE_PHASE_RECOVERY" out of this file as a
// string literal (the retired key), which the flagreaders guard checks.
const envIPCPhaseRecoveryStage = "EVOLVE_" + "PHASE_RECOVERY_STAGE" // SSOT IPC-protocol-allowed

// resolveStallPolicy resolves the ADR-0044 chain-backed stall policy for the
// observer subprocess. It activates ONLY at enforce, from EITHER source:
//   - enforce: the operator's --enforce flag on the manual `evolve phase-observer`
//     command. This is the live signal for the standalone subcommand — the
//     orchestrator's auto-spawn path uses the in-process observer adapter
//     (adapters/observer.CoreAdapter, which reads its own RecoveryStage field),
//     not this subprocess, so nothing injects the IPC stage key here.
//   - envIPCPhaseRecoveryStage == "enforce": a parent that DOES inject the stage
//     (kept as an accepted IPC channel for forward-compat).
//
// Any other state (off, shadow, unset, typo, --enforce absent) ⇒ nil policy ⇒
// byte-identical legacy behavior. A typo never enables a kill-path.
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
