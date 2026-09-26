package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func runContinuation(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "evolve continuation: usage: continuation list | continuation release <scope-id>")
		return 10
	}
	switch args[0] {
	case "list":
		return runContinuationList(args[1:], stdout, stderr)
	case "release":
		return runContinuationRelease(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve continuation: unknown subcommand %q (want: list, release)\n", args[0])
		return 10
	}
}

// continuationRoot resolves the project root from the flag, else the working
// directory. Never $EVOLVE_PROJECT_ROOT: inside a lane it names another tree.
func continuationRoot(fs *flag.FlagSet, args []string, rootFlag *string, stderr io.Writer) (string, bool) {
	if err := fs.Parse(args); err != nil {
		return "", false
	}
	if *rootFlag != "" {
		return *rootFlag, true
	}
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "evolve continuation: cannot resolve the project root from the working directory: %v\n", err)
		return "", false
	}
	return wd, true
}

// runContinuationList prints every scope→binding pair. An absent registry is a
// healthy project's normal state, so it exits 0.
func runContinuationList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("continuation list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rootFlag := fs.String("project-root", "", "project root (default: the working directory)")
	root, ok := continuationRoot(fs, args, rootFlag, stderr)
	if !ok {
		return 10
	}
	if rest := fs.Args(); len(rest) > 0 {
		fmt.Fprintf(stderr, "evolve continuation list: takes no arguments, got %q\n", rest[0])
		return 10
	}

	byScope, err := continuation.ListRegistryEntries(root)
	if err != nil {
		fmt.Fprintf(stderr, "evolve continuation list: continuation registry unreadable at %s: %v\n", continuation.RegistryPath(root), err)
		return 1
	}
	if len(byScope) == 0 {
		fmt.Fprintf(stdout, "no continuation bindings (%s)\n", continuation.RegistryPath(root))
		return 0
	}
	ids := make([]string, 0, len(byScope))
	for id := range byScope {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	fmt.Fprintf(stdout, "%d continuation binding(s) in %s:\n", len(ids), continuation.RegistryPath(root))
	for _, id := range ids {
		c := continuation.RedactHostPaths(byScope[id])
		fmt.Fprintf(stdout, "  %s  snapshot_sha=%s  base_sha=%s  branch=%s  cycle=%d  worktree=%s\n",
			id, c.SnapshotSHA, c.BaseSHA, c.Branch, c.Cycle, c.Worktree)
	}
	return 0
}

// runContinuationRelease drops one binding through the shared
// preserve-then-delete transaction. Its refusals, cheapest first, leave the
// registry untouched: no authority (checked before any read), no such binding,
// and a live lane.
func runContinuationRelease(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("continuation release", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rootFlag := fs.String("project-root", "", "project root (default: the working directory)")
	operator := fs.Bool("operator", false, "authorize this release as the operator (or set "+continuation.OperatorConfirmEnv+"=1)")
	force := fs.Bool("force", false, "release even while the binding's cycle still holds a live lease")
	root, ok := continuationRoot(fs, args, rootFlag, stderr)
	if !ok {
		return 10
	}
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintf(stderr, "evolve continuation release: want exactly one <scope-id>, got %d — refusing (a release with no explicit scope could drop the wrong binding)\n", len(rest))
		return 10
	}
	scopeID := rest[0]

	if err := continuation.RequireOperatorAuthority(*operator); err != nil {
		fmt.Fprintf(stderr, "evolve continuation release: refusing to release %q: %v\n", scopeID, err)
		return 10
	}

	// Read first to tell "no such binding" from a release, which the runtime
	// reports as the same clean miss; the value also names the lane checked next.
	bound, isBound, err := continuation.ReadRegistryEntry(root, scopeID)
	if err != nil {
		fmt.Fprintf(stderr, "evolve continuation release: registry unreadable while looking up %q: %v\n", scopeID, err)
		return 1
	}
	if !isBound {
		fmt.Fprintf(stderr, "evolve continuation release: scope %q holds no continuation binding in %s — nothing released\n", scopeID, continuation.RegistryPath(root))
		return 1
	}

	if !*force && continuationLaneIsLive(root, bound, time.Now(), stderr) {
		fmt.Fprintf(stderr, "evolve continuation release: scope %q is bound to cycle %d, whose lane is still LIVE (lease heartbeat in %s is fresher than %s) — refusing to drop a running lane's lineage out from under it; wait for the lane to finish, or pass -force to override\n",
			scopeID, bound.Cycle, runlease.PathIn(paths.RunWorkspace(root, bound.Cycle)), runlease.DefaultTTL)
		return 10
	}

	authority := "operator via -operator"
	if !*operator {
		authority = "operator via " + continuation.OperatorConfirmEnv + "=1"
	}
	reason := "operator-release"
	if *force {
		reason = fmt.Sprintf("operator-release (-force override of cycle %d's live lease)", bound.Cycle)
	}

	c, released, err := inboxmover.ReleaseContinuationBinding(
		inboxmover.Options{ProjectRoot: root, Stderr: stderr}, scopeID, reason, authority)
	if err != nil {
		fmt.Fprintf(stderr, "evolve continuation release: %q: %v\n", scopeID, err)
		return 1
	}
	if !released {
		fmt.Fprintf(stderr, "evolve continuation release: scope %q was rebound by a live lane between the read and the release (cycle %d no longer owns it) — left intact\n", scopeID, c.Cycle)
		return 1
	}
	safe := continuation.RedactHostPaths(c)
	fmt.Fprintf(stdout, "released continuation binding for scope %q by %s (snapshot_sha=%s base_sha=%s branch=%s cycle=%d); pointer preserved in the scope's inbox item where one exists\n",
		scopeID, authority, safe.SnapshotSHA, safe.BaseSHA, safe.Branch, safe.Cycle)
	return 0
}

// continuationLaneIsLive judges by lease heartbeat freshness alone. A dead
// lane's stale or unreadable lease must not block a release, or it would brick
// the scope for good.
func continuationLaneIsLive(root string, c continuation.Continuation, now time.Time, stderr io.Writer) bool {
	if c.Cycle <= 0 {
		return false
	}
	runDir := paths.RunWorkspace(root, c.Cycle)
	lease, ok, err := runlease.Read(runDir)
	if err != nil {
		fmt.Fprintf(stderr, "evolve continuation release: WARN: the lease at %s is unreadable (%v) — treating cycle %d as not live\n", runlease.PathIn(runDir), err, c.Cycle)
		return false
	}
	return ok && runlease.Fresh(lease, now, 0)
}
