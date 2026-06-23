package main

import (
	"fmt"
	"io"

	"github.com/mickeyyaya/evolveloop/go/internal/aggregator"
)

// runAggregator is the `evolve aggregator <phase> <output> <worker>...` subcommand.
// Ports legacy/scripts/dispatch/aggregator.sh.
func runAggregator(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var pos []string
	for _, a := range args {
		switch a {
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve aggregator <phase> <output> <worker-artifact>...")
			fmt.Fprintln(stdout, "Phases: scout|research|discover|audit|learn|retrospective|plan-review|audit-consensus|cross-cli-vote")
			return 0
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) < 3 {
		fmt.Fprintln(stderr, "[aggregator] usage: aggregator <phase> <output> <worker-artifact>...")
		return aggregator.ExitUsageErr
	}
	return aggregator.Aggregate(aggregator.Inputs{
		Phase:   pos[0],
		Output:  pos[1],
		Workers: pos[2:],
	}, stderr)
}
