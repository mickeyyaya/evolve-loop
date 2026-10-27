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
	asJSON, gateArgs := takeInboxJSON(args)
	blocker, args, ok := takeBlockedBy(gateArgs)
	if !ok || len(args) < 2 || strings.TrimSpace(args[0]) == "" || strings.TrimSpace(args[1]) == "" {
		fmt.Fprintln(stderr, inboxUsage("route-lane"))
		return 10
	}
	if rc, queued := inboxQueueGate(root, "route-lane", gateArgs, asJSON, stdout, stderr); queued {
		return rc
	}
	id, reason := strings.TrimSpace(args[0]), strings.TrimSpace(args[1])
	opts := inboxmover.Options{ProjectRoot: root, Stderr: stderr, IsProtectedPath: laneForbidden(root, stderr)}
	res, err := inboxmover.RouteLaneRequest(opts, inboxmover.RouteRequest{TaskID: id, Reason: reason, BlockedBy: blocker})
	switch {
	case errors.Is(err, inboxmover.ErrNotFound), errors.Is(err, inboxmover.ErrConsoleRouted), errors.Is(err, inboxmover.ErrInvalidItem):
		fmt.Fprintf(stderr, "inbox route-lane: %v\n", err)
		return 1
	case err != nil:
		fmt.Fprintf(stderr, "inbox route-lane: %v\n", err)
		return 2
	}
	detail := "lane-dispatchable"
	if blocker != "" {
		detail += " once " + blocker + " is consumed"
	}
	human := fmt.Sprintf("inbox route-lane: %s is %s (%s)", id, detail, filepath.Base(res.Path))
	return finishInboxWrite(asJSON, inboxWriteResult{Verb: "route-lane", ID: id, Path: res.Path, Detail: detail}, human, stdout, stderr)
}
