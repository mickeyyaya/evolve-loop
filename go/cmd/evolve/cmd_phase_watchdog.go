package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasewatchdog"
)

// runPhaseWatchdog is the `evolve phase-watchdog <workspace> <pgid> <cycle> <cycle-state>` subcommand.
// Ports legacy/scripts/dispatch/phase-watchdog.sh.
func runPhaseWatchdog(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var pos []string
	for _, a := range args {
		switch a {
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve phase-watchdog <workspace> <target_pgid> <cycle> <cycle_state_path>")
			fmt.Fprintln(stdout, "Config: .evolve/policy.json observer.{stall_s,watchdog_poll_s,watchdog_warn_pct,watchdog_grace_s,watchdog_disabled}")
			return 0
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) < 4 {
		fmt.Fprintln(stderr, "[phase-watchdog] ERROR: requires 4 arguments: <workspace> <target_pgid> <cycle> <cycle_state_path>")
		return phasewatchdog.ExitInvalidArg
	}
	pgid, err := strconv.Atoi(pos[1])
	if err != nil {
		fmt.Fprintf(stderr, "[phase-watchdog] ERROR: target_pgid must be a positive integer, got: %s\n", pos[1])
		return phasewatchdog.ExitInvalidArg
	}
	cycle, err := strconv.Atoi(pos[2])
	if err != nil {
		fmt.Fprintf(stderr, "[phase-watchdog] ERROR: cycle must be a positive integer, got: %s\n", pos[2])
		return phasewatchdog.ExitInvalidArg
	}
	cfg := watchdogEnvConfig()
	cfg.Workspace = pos[0]
	cfg.TargetPGID = pgid
	cfg.Cycle = cycle
	cfg.CycleStatePath = pos[3]
	return phasewatchdog.Run(cfg, stderr)
}

// watchdogEnvConfig resolves watchdog settings from .evolve/policy.json.
// EVOLVE_PROJECT_ROOT remains the bootstrap path to that file.
func watchdogEnvConfig() phasewatchdog.Config {
	cfg := loadObserverPolicy()
	return phasewatchdog.Config{
		ProjectRoot: os.Getenv("EVOLVE_PROJECT_ROOT"),
		ThresholdS:  *cfg.StallS,
		PollS:       *cfg.WatchdogPollS,
		WarnPct:     *cfg.WatchdogWarnPct,
		GraceS:      *cfg.WatchdogGraceS,
		Disabled:    cfg.WatchdogDisabled,
	}
}
