package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseobserver"
)

// runPhaseObserver is the `evolve phase-observer [--enforce] [--scope=...] <ws> <pgid> <cycle> <phase> <agent> [state]` subcommand.
// Ports the core stall-detection behavior of legacy/scripts/dispatch/phase-observer.sh.
func runPhaseObserver(args []string, _ io.Reader, stdout, stderr io.Writer) int {
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

	return phaseobserver.Run(phaseobserver.Config{
		Workspace:    pos[0],
		SubagentPGID: pgid,
		Cycle:        cycle,
		Phase:        pos[3],
		Agent:        pos[4],
		CycleState:   cycleState,
		Scope:        scope,
		Enforce:      enforce,
		PollS:        atoiOr(os.Getenv("EVOLVE_OBSERVER_POLL_S"), 0),
		StallS:       atoiOr(envOr("EVOLVE_OBSERVER_STALL_S", os.Getenv("EVOLVE_INACTIVITY_THRESHOLD_S")), 0),
		EOFGraceS:    atoiOr(os.Getenv("EVOLVE_OBSERVER_EOF_GRACE_S"), 0),
		ShutdownSig:  shutdown,
	}, "", stderr)
}

func envOr(primary, fallback string) string {
	if v := os.Getenv(primary); v != "" {
		return v
	}
	return fallback
}
