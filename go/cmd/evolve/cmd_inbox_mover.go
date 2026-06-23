package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolveloop/go/internal/inboxmover"
)

// runInboxMover is `evolve inbox-mover <subcmd> [args]`. Mirrors
// legacy/scripts/utility/inbox-mover.sh exit codes:
//
//	0  — success (or promote no-op for ship.sh compat)
//	1  — not-found / bad args (claim only)
//	2  — mv failed (claim only)
func runInboxMover(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printInboxMoverUsage(stderr)
		return 1
	}
	subcmd := args[0]
	rest := args[1:]
	projectRoot := envOrCwd("EVOLVE_PROJECT_ROOT")
	opts := inboxmover.Options{
		ProjectRoot: projectRoot,
		Stderr:      stderr,
	}

	switch subcmd {
	case "--help", "-h":
		printInboxMoverUsage(stdout)
		return 0
	case "claim":
		if len(rest) < 2 {
			fmt.Fprintln(stderr, "[inbox-mover] ERROR: usage: claim <task_id> <cycle>")
			return 1
		}
		_, err := inboxmover.Claim(opts, rest[0], rest[1])
		if err == nil {
			return 0
		}
		if errors.Is(err, inboxmover.ErrBadArgs) || errors.Is(err, inboxmover.ErrNotFound) {
			return 1
		}
		if errors.Is(err, inboxmover.ErrMvFailed) {
			return 2
		}
		fmt.Fprintf(stderr, "[inbox-mover] ERROR: %v\n", err)
		return 1
	case "promote":
		if len(rest) < 2 {
			fmt.Fprintln(stderr, "[inbox-mover] ERROR: usage: promote <task_id> <new_state> [<cycle>] [--commit-sha <sha>]")
			return 1
		}
		taskID := rest[0]
		newState := rest[1]
		p, parseErr := parsePromoteArgs(rest[2:])
		if parseErr != nil {
			fmt.Fprintf(stderr, "[inbox-mover] ERROR: %v\n", parseErr)
			return 1
		}
		_, err := inboxmover.Promote(opts, taskID, newState, p)
		if err == nil {
			return 0
		}
		if errors.Is(err, inboxmover.ErrBadArgs) || errors.Is(err, inboxmover.ErrBadState) {
			return 1
		}
		// ship.sh compat: all other paths exit 0.
		return 0
	case "recover-orphans":
		_, err := inboxmover.RecoverOrphans(opts)
		if err != nil {
			fmt.Fprintf(stderr, "[inbox-mover] ERROR: %v\n", err)
			return 1
		}
		return 0
	default:
		printInboxMoverUsage(stderr)
		return 1
	}
}

func printInboxMoverUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: evolve inbox-mover <claim|promote|recover-orphans> [args]")
	fmt.Fprintln(w, "  claim <task_id> <cycle>")
	fmt.Fprintln(w, "  promote <task_id> <new_state> [<cycle>] [--commit-sha <sha>]")
	fmt.Fprintln(w, "    new_state: processed | rejected | retry")
	fmt.Fprintln(w, "  recover-orphans")
}

// parsePromoteArgs walks the optional [<cycle>] [--commit-sha <sha>] tail.
func parsePromoteArgs(args []string) (inboxmover.PromoteOpts, error) {
	var p inboxmover.PromoteOpts
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--commit-sha":
			i++
			if i >= len(args) {
				return p, fmt.Errorf("--commit-sha missing value")
			}
			p.CommitSHA = args[i]
		case len(a) >= 2 && a[:2] == "--":
			// Unknown flag — skip (bash semantics).
		default:
			if p.Cycle == "" {
				p.Cycle = a
			}
		}
		i++
	}
	return p, nil
}
