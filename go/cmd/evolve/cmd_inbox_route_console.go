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

func runInboxRouteConsole(root string, args []string, stdout, stderr io.Writer) int {
	asJSON, args := takeInboxJSON(args)
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
	if rc, queued := inboxQueueGate(root, "route-console", args, asJSON, stdout, stderr); queued {
		return rc
	}
	opts := inboxmover.Options{ProjectRoot: root, Stderr: stderr}
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
	human := fmt.Sprintf("inbox route-console: %s is console-manual (%s)", id, filepath.Base(res.Path))
	return finishInboxWrite(asJSON, inboxWriteResult{Verb: "route-console", ID: id, Path: res.Path, Detail: "console-manual"}, human, stdout, stderr)
}
