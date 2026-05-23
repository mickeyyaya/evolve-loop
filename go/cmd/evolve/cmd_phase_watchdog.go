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
			fmt.Fprintln(stdout, "Env: EVOLVE_INACTIVITY_THRESHOLD_S (default 600), POLL_S (15),")
			fmt.Fprintln(stdout, "     WARN_PCT (75), GRACE_S (10), DISABLE (0), EVOLVE_PROJECT_ROOT")
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
	return phasewatchdog.Run(phasewatchdog.Config{
		Workspace:      pos[0],
		TargetPGID:     pgid,
		Cycle:          cycle,
		CycleStatePath: pos[3],
		ProjectRoot:    os.Getenv("EVOLVE_PROJECT_ROOT"),
		ThresholdS:     atoiOr(os.Getenv("EVOLVE_INACTIVITY_THRESHOLD_S"), 0),
		PollS:          atoiOr(os.Getenv("EVOLVE_INACTIVITY_POLL_S"), 0),
		WarnPct:        atoiOr(os.Getenv("EVOLVE_INACTIVITY_WARN_PCT"), 0),
		GraceS:         atoiOr(os.Getenv("EVOLVE_INACTIVITY_GRACE_S"), 0),
		Disabled:       os.Getenv("EVOLVE_INACTIVITY_DISABLE") == "1",
	}, stderr)
}

func atoiOr(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}
