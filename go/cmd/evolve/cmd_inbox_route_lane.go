package main

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func runInboxRouteLane(root string, args []string, stdout, stderr io.Writer) int {
	asJSON, args := takeInboxJSON(args)
	if len(args) < 2 || strings.TrimSpace(args[0]) == "" || strings.TrimSpace(args[1]) == "" {
		fmt.Fprintln(stderr, inboxUsage("route-lane"))
		return 10
	}
	if refusedMidWaveAt(root, "route-lane", stderr) {
		return cliExitRefused
	}
	id, reason := strings.TrimSpace(args[0]), strings.TrimSpace(args[1])
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
	human := fmt.Sprintf("inbox route-lane: %s is lane-dispatchable (%s)", id, filepath.Base(res.Path))
	return finishInboxWrite(asJSON, inboxWriteResult{Verb: "route-lane", ID: id, Path: res.Path, Detail: "lane-dispatchable"}, human, stdout, stderr)
}
