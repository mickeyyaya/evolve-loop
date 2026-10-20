package main

import (
	"fmt"
	"io"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func batchLaneClaims(projectRoot string, firstCycle int, stderr io.Writer) []string {
	if firstCycle <= 0 {
		return nil
	}
	survey, err := inboxmover.SurveyClaims(inboxmover.Options{ProjectRoot: projectRoot, Stderr: io.Discard})
	if err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: cannot list the lane claims of this batch: %v\n", err)
		return nil
	}
	var lanes []string
	for _, c := range survey.Claims {
		if c.Holder.Cycle >= firstCycle {
			lanes = append(lanes, fmt.Sprintf("cycle %d holds %s", c.Holder.Cycle, c.ID))
		}
	}
	slices.Sort(lanes)
	return lanes
}

func fleetSignalStop(stdout, stderr io.Writer, lr *loopResult, where string, lanes []string) {
	fmt.Fprintf(stderr, "[loop] received interrupt (SIGINT/SIGTERM) %s; these lanes of this batch held inbox claims when it stopped. A signal stop writes no resumable checkpoint, so `evolve loop --resume` cannot continue them (it names each lane that it considered):\n", where)
	for _, l := range lanes {
		fmt.Fprintf(stderr, "  %s\n", l)
	}
	fmt.Fprintln(stderr, "[loop] release their claims with `evolve inbox-mover recover-orphans`, then launch the goal again")
	lr.StopReason = "signal"
	lr.emit(stdout)
}
