package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/fanoutdispatch"
)

// runFanoutDispatch is the `evolve fanout-dispatch [--cache-prefix-file=PATH] <cmds> <results>` subcommand.
// Ports legacy/scripts/dispatch/fanout-dispatch.sh.
func runFanoutDispatch(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var pos []string
	var cachePrefix string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve fanout-dispatch [--cache-prefix-file=PATH] <commands.tsv> <results.tsv>")
			fmt.Fprintln(stdout, "Env: EVOLVE_FANOUT_CONCURRENCY (2), EVOLVE_FANOUT_TIMEOUT (600)")
			fmt.Fprintln(stdout, "     EVOLVE_FANOUT_CANCEL_ON_CONSENSUS, EVOLVE_FANOUT_CONSENSUS_K (2)")
			fmt.Fprintln(stdout, "     EVOLVE_FANOUT_PER_WORKER_BUDGET_USD (0.20)")
			return 0
		case a == "--cache-prefix-file":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "[fanout-dispatch] --cache-prefix-file requires value")
				return fanoutdispatch.ExitSetupErr
			}
			i++
			cachePrefix = args[i]
		case strings.HasPrefix(a, "--cache-prefix-file="):
			cachePrefix = strings.TrimPrefix(a, "--cache-prefix-file=")
		case strings.HasPrefix(a, "--"):
			fmt.Fprintf(stderr, "[fanout-dispatch] unknown flag: %s\n", a)
			return fanoutdispatch.ExitSetupErr
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) < 2 {
		fmt.Fprintln(stderr, "[fanout-dispatch] usage: fanout-dispatch [--cache-prefix-file=PATH] <commands.tsv> <results.tsv>")
		return fanoutdispatch.ExitSetupErr
	}
	return fanoutdispatch.Run(fanoutdispatch.Config{
		CommandsFile:        pos[0],
		ResultsFile:         pos[1],
		CachePrefixFile:     cachePrefix,
		Concurrency:         atoiOr(os.Getenv("EVOLVE_FANOUT_CONCURRENCY"), 0),
		TimeoutSecs:         atoiOr(os.Getenv("EVOLVE_FANOUT_TIMEOUT"), 0),
		CancelOnConsensus:   os.Getenv("EVOLVE_FANOUT_CANCEL_ON_CONSENSUS") == "1",
		ConsensusK:          atoiOr(os.Getenv("EVOLVE_FANOUT_CONSENSUS_K"), 0),
		ConsensusPollSecs:   atoiOr(os.Getenv("EVOLVE_FANOUT_CONSENSUS_POLL_S"), 0),
		PerWorkerBudgetUSD:  os.Getenv("EVOLVE_FANOUT_PER_WORKER_BUDGET_USD"),
		TrackWorkers:        envBoolDefault("EVOLVE_FANOUT_TRACK_WORKERS", true),
		CycleStateHelperBin: locateCycleStateHelper(),
	}, stderr)
}

func envBoolDefault(k string, dflt bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return dflt
	}
	return v == "1" || v == "true"
}

// locateCycleStateHelper returns the path to the bash cycle-state helper
// if present. In v12.0.0+ legacy/ is removed so this returns empty,
// signalling callers to use their native Go path.
func locateCycleStateHelper() string {
	if pluginRoot := os.Getenv("EVOLVE_PLUGIN_ROOT"); pluginRoot != "" {
		p := pluginRoot + "/legacy/scripts/lifecycle/cycle-state.sh"
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

// re-using atoiOr from cmd_phase_watchdog.go
var _ = strconv.Atoi
