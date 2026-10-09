package main

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func runInboxRouteConsole(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 || strings.TrimSpace(args[0]) == "" || strings.TrimSpace(args[1]) == "" {
		fmt.Fprintln(stderr, inboxUsage("route-console"))
		return 10
	}
	id, reason := strings.TrimSpace(args[0]), strings.TrimSpace(args[1])
	cycle, err := strconv.Atoi(args[2])
	if err != nil || cycle < 0 {
		fmt.Fprintf(stderr, "inbox route-console: cycle %q is not a cycle number\n%s\n", args[2], inboxUsage("route-console"))
		return 10
	}
	if refusedMidWave("route-console", stderr) {
		return 1
	}
	opts := inboxmover.Options{ProjectRoot: envOrCwd("EVOLVE_PROJECT_ROOT"), Stderr: stderr}
	loc, err := inboxmover.Locate(filepath.Join(opts.ProjectRoot, ".evolve", "inbox"), id)
	if err != nil {
		fmt.Fprintf(stderr, "inbox route-console: %s: %v\n", id, err)
		if errors.Is(err, inboxmover.ErrNotFound) {
			return 1
		}
		return 2
	}
	if loc.Cycle != 0 {
		fmt.Fprintf(stderr, "inbox route-console: %s is held by cycle %d's claim; route it after that lane closes (or after evolve inbox-mover recover-orphans)\n", id, loc.Cycle)
		return 1
	}
	res, err := inboxmover.RouteConsole(opts, id, reason, cycle)
	if err != nil {
		fmt.Fprintf(stderr, "inbox route-console: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "inbox route-console: %s is console-manual (%s)\n", id, filepath.Base(res.Path))
	return 0
}
