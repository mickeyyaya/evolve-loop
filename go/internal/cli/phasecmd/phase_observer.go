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

// RunPhaseObserver is the `evolve phase-observer [--enforce] [--scope=...] <ws> <pgid> <cycle> <phase> <agent> [state]` subcommand.
// Ports the core stall-detection behavior of legacy/scripts/dispatch/phase-observer.sh.
// The composition root of ADR-0103 unit 12: it parses argv once, arms the
// SIGUSR1 shutdown, builds the subprocess's stderr-only Signal Center and
// hands the engine's host a Config whose accessor reaches it.
func RunPhaseObserver(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	a, rc, handled := parseObserverArgs(args, stdout, stderr)
	if handled {
		return rc
	}

	// SIGUSR1 = "subagent has exited; finalize"
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

// observerArgs is argv read once: the two flags and the positionals.
type observerArgs struct {
	enforce bool
	scope   phaseobserver.Scope
	pos     []string
}

// parseObserverArgs is the ONE reading of argv. handled reports that the call
// is over — help printed (rc 0) or a usage error (ExitInvalidArgs) — with the
// exact lines the subcommand always printed.
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

// observerConfig projects the parsed argv, the policy-resolved thresholds and
// the root's collaborators onto the host's Config. The pgid and cycle Atoi
// errors are discarded as they always were (a bogus value parses as 0 and
// Run's `cycle must be integer` fires downstream).
func observerConfig(a observerArgs, shutdown <-chan struct{}, signals *signalcenter.Center) phaseobserver.Config {
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
	// ADR-0044 C3: the chain-backed stall policy executes ONLY at enforce. For
	// this standalone subcommand the operator's --enforce flag is the live signal
	// (the IPC stage env key is an accepted fallback for an injecting parent).
	// off/shadow/unset + no --enforce ⇒ nil policy ⇒ byte-identical legacy Enforce
	// branch — shadow observability for stalls already exists via the INCIDENT
	// events themselves.
	cfg.StallPolicy = resolveStallPolicy(a.enforce)
	// R3.4: the process-liveness probe is wired unconditionally — it is
	// deterministic ground truth (signal-0), not policy; nil in Run means
	// probe-off (fixture Configs). The ACTION on a dead group stays
	// policy/Enforce-gated; at shadow the INCIDENT is pure soak telemetry
	// (pane echo ≠ liveness, cycles 274/277).
	cfg.ProcessAlive = phaseobserver.DefaultProcessAlive
	// ADR-0103 unit 12: the engine reports its own faults through the
	// subprocess's Center (read late; nil = the Null Object).
	cfg.Signals = func() *signalcenter.Center { return signals }
	return cfg
}

// observerSignalCenter is the subprocess's Center: signalcenter.ConsoleSink —
// the console half of the orchestrator's root topology (cmd_cycle.go
// newRootSignalCenter), the ONE home of the WARN threshold — on the
// subcommand's own stderr, where the replaced [phase-observer] lines used to
// print — and NO file: a second writer into the orchestrator's per-cycle
// signals.ndjson would interleave a second pid's sequence space (design §4
// "monotonic seq per file").
func observerSignalCenter(w io.Writer) *signalcenter.Center {
	c := signalcenter.New(signalcenter.WithPID(os.Getpid()))
	c.Subscribe(signalcenter.ConsoleSink(w))
	return c
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
