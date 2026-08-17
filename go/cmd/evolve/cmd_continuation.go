package main

// cmd_continuation.go — `evolve continuation list` / `evolve continuation
// release <scope-id>`: the operator surface for the scope-keyed continuation
// registry.
//
// The registry is written under a flock sidecar by the runtime, and until now
// the only way for console to inspect or drop a stale binding was to hand-edit
// .evolve/continuation-registry.json — outside that lock, with no preservation
// of the salvage pointer it destroyed. Both subcommands reach the SAME paths
// the runtime uses (continuation.ListRegistryEntries for the read,
// inboxmover.ReleaseContinuationBinding for the preserve-then-delete
// transaction) rather than re-implementing them here; a second copy of the
// release order is exactly the drift that produced audit cycle-1507's H2.

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
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

// continuationRoot resolves the project root from the flag, else the process
// working directory. Deliberately NOT $EVOLVE_PROJECT_ROOT: an operator running
// this inside a lane's environment would otherwise release bindings in a
// different tree than the one they are standing in.
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

// runContinuationList prints every scope→binding pair. An absent registry is
// the normal state of a healthy project, so it is a clean exit-0 empty report,
// never an error.
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

// runContinuationRelease drops exactly one binding through the shared
// preserve-then-delete transaction. A scope holding no binding is an ERROR, not
// a silent success: a typo'd id must never read as a completed release.
func runContinuationRelease(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("continuation release", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rootFlag := fs.String("project-root", "", "project root (default: the working directory)")
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

	// Read first purely to tell "no such binding" (operator error, non-zero)
	// apart from "released" — ReleaseContinuationBinding reports both as a
	// clean miss because releasing nothing is not a failure for the runtime.
	if _, bound, err := continuation.ReadRegistryEntry(root, scopeID); err != nil {
		fmt.Fprintf(stderr, "evolve continuation release: registry unreadable while looking up %q: %v\n", scopeID, err)
		return 1
	} else if !bound {
		fmt.Fprintf(stderr, "evolve continuation release: scope %q holds no continuation binding in %s — nothing released\n", scopeID, continuation.RegistryPath(root))
		return 1
	}

	c, released, err := inboxmover.ReleaseContinuationBinding(
		inboxmover.Options{ProjectRoot: root, Stderr: stderr}, scopeID, "operator-release")
	if err != nil {
		fmt.Fprintf(stderr, "evolve continuation release: %q: %v\n", scopeID, err)
		return 1
	}
	if !released {
		fmt.Fprintf(stderr, "evolve continuation release: scope %q was rebound by a live lane between the read and the release (cycle %d no longer owns it) — left intact\n", scopeID, c.Cycle)
		return 1
	}
	safe := continuation.RedactHostPaths(c)
	fmt.Fprintf(stdout, "released continuation binding for scope %q (snapshot_sha=%s base_sha=%s branch=%s cycle=%d); pointer preserved in the scope's inbox item where one exists\n",
		scopeID, safe.SnapshotSHA, safe.BaseSHA, safe.Branch, safe.Cycle)
	return 0
}
