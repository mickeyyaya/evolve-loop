package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/fanoutdispatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
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
			fmt.Fprintln(stdout, "Config: .evolve/policy.json fanout.{concurrency,timeout_secs,cancel_on_consensus,consensus_k,consensus_poll_secs,track_workers}")
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
	pol, err := policy.Load(filepath.Join(os.Getenv("EVOLVE_PROJECT_ROOT"), ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(stderr, "[fanout-dispatch] WARN: policy load: %v; using defaults\n", err)
		pol = policy.Policy{}
	}
	fc := pol.FanoutConfig()
	cfg := fanoutdispatch.Config{
		Concurrency:       fc.Concurrency,
		TimeoutSecs:       fc.TimeoutSecs,
		CancelOnConsensus: fc.CancelOnConsensus,
		ConsensusK:        fc.ConsensusK,
		ConsensusPollSecs: fc.ConsensusPollSecs,
		TrackWorkers:      *fc.TrackWorkers,
	}
	cfg.CommandsFile = pos[0]
	cfg.ResultsFile = pos[1]
	cfg.CachePrefixFile = cachePrefix
	cfg.CycleStateHelperBin = locateCycleStateHelper()
	return fanoutdispatch.Run(cfg, stderr)
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
