package main

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func runInboxRouteLane(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 || strings.TrimSpace(args[0]) == "" || strings.TrimSpace(args[1]) == "" {
		fmt.Fprintln(stderr, inboxUsage("route-lane"))
		return 10
	}
	if refusedMidWave("route-lane", stderr) {
		return 1
	}
	id, reason := strings.TrimSpace(args[0]), strings.TrimSpace(args[1])
	root := envOrCwd("EVOLVE_PROJECT_ROOT")
	opts := inboxmover.Options{ProjectRoot: root, Stderr: stderr, IsProtectedPath: laneForbidden(root, stderr)}
	res, err := inboxmover.RouteLane(opts, id, reason)
	switch {
	case errors.Is(err, inboxmover.ErrNotFound), errors.Is(err, inboxmover.ErrConsoleRouted):
		fmt.Fprintf(stderr, "inbox route-lane: %v\n", err)
		return 1
	case err != nil:
		fmt.Fprintf(stderr, "inbox route-lane: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "inbox route-lane: %s is lane-dispatchable (%s)\n", id, filepath.Base(res.Path))
	return 0
}
