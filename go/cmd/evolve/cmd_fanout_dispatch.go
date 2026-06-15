package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
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
	cfg := fanoutEnvConfig()
	cfg.CommandsFile = pos[0]
	cfg.ResultsFile = pos[1]
	cfg.CachePrefixFile = cachePrefix
	cfg.CycleStateHelperBin = locateCycleStateHelper()
	return fanoutdispatch.Run(cfg, stderr)
}

// fanoutEnvConfig reads the EVOLVE_FANOUT_* knobs through the envchain
// precedence chain so the truthy/falsy/default vocabulary is uniform (P2). The
// int knobs default to 0 (the package's "use built-in default" sentinel);
// CancelOnConsensus is a default-off `== "1"` flag, TrackWorkers default-on.
func fanoutEnvConfig() fanoutdispatch.Config {
	return fanoutdispatch.Config{
		Concurrency:       envchain.Int("EVOLVE_FANOUT_CONCURRENCY", nil, 0),
		TimeoutSecs:       envchain.Int("EVOLVE_FANOUT_TIMEOUT", nil, 0),
		CancelOnConsensus: envchain.Bool("EVOLVE_FANOUT_CANCEL_ON_CONSENSUS", nil, false),
		ConsensusK:        envchain.Int("EVOLVE_FANOUT_CONSENSUS_K", nil, 0),
		ConsensusPollSecs: envchain.Int("EVOLVE_FANOUT_CONSENSUS_POLL_S", nil, 0),
		TrackWorkers:      envchain.Bool("EVOLVE_FANOUT_TRACK_WORKERS", nil, true),
	}
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
