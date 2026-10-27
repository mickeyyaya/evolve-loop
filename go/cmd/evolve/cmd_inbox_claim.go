package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

type inboxClaimResult struct {
	ID    string `json:"id"`
	Cycle int    `json:"cycle"`
	From  string `json:"from"`
	Path  string `json:"path"`
}

func runInboxClaim(root string, args []string, stdout, stderr io.Writer) int {
	cycleText, args, hasCycle, cycleOK := takeFlagValue(args, "--cycle")
	flags, operands, ok := splitInboxFlags(args, "--json")
	cycle, err := strconv.Atoi(cycleText)
	if !ok || !cycleOK || !hasCycle || err != nil || cycle < 1 || len(operands) != 1 {
		fmt.Fprintln(stderr, inboxUsage("claim"))
		return cliExitUsage
	}
	opts := inboxmover.Options{ProjectRoot: root, Stderr: stderr, IsProtectedPath: laneForbidden(root, stderr)}
	res, err := inboxmover.ClaimDispatchable(opts, operands[0], strconv.Itoa(cycle))
	if rc := reportInboxClaimError(err, stderr); rc != cliExitOK {
		return rc
	}
	r := inboxClaimResult{ID: operands[0], Cycle: cycle, From: res.SrcPath, Path: res.DestPath}
	if flags["--json"] {
		return encodeInboxJSON("claim", r, stdout, stderr)
	}
	fmt.Fprintf(stdout, "inbox claim: %s claimed for cycle-%d: %s\n", r.ID, r.Cycle, r.Path)
	return cliExitOK
}

func reportInboxClaimError(err error, stderr io.Writer) int {
	if err == nil {
		return cliExitOK
	}
	fmt.Fprintf(stderr, "inbox claim: %v\n", err)
	switch {
	case errors.Is(err, inboxmover.ErrBadArgs):
		return cliExitUsage
	case errors.Is(err, inboxmover.ErrNotFound):
		return cliExitFailed
	case errors.Is(err, inboxmover.ErrConsoleRouted), errors.Is(err, inboxmover.ErrWaitingOnDependency):
		return cliExitRefused
	}
	return cliExitIO
}
