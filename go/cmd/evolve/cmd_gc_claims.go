package main

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func (r gcRun) claimDirs(projectRoot string) bool {
	opts := inboxmover.Options{ProjectRoot: projectRoot}
	survey, err := inboxmover.SurveyClaims(opts)
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: claim dir survey failed: %v\n", err)
		return true
	}
	var stale []string
	for _, dir := range survey.EmptyDirs {
		if !dir.Holder.Keeps() {
			stale = append(stale, dir.Path)
		}
	}
	r.summary("%d empty claim dir(s) of stale cycles would be removed", "removing %d empty claim dir(s) of stale cycles", len(stale))
	verb := "REMOVE"
	if r.dryRun {
		verb = "WOULD-REMOVE"
	}
	failed := false
	for _, dir := range stale {
		fmt.Fprintf(r.stdout, "  %s %s\n", verb, dir)
		if err := r.remove(dir); err != nil {
			fmt.Fprintf(r.stderr, "evolve gc: claim dir %s: %v\n", dir, err)
			failed = true
		}
	}
	return failed
}
