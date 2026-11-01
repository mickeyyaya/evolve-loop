package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

const continuationAdoptUsage = "evolve continuation adopt: usage: continuation adopt <item-id> --worktree <path> [--project-root P]"

func runContinuationAdopt(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("continuation adopt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rootFlag := fs.String("project-root", "", "project root (default: the working directory)")
	worktree := fs.String("worktree", "", "path of the new worktree")
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(stderr, continuationAdoptUsage)
		return 10
	}
	id := args[0]
	root, ok := continuationRoot(fs, args[1:], rootFlag, stderr)
	if !ok || fs.NArg() > 0 || *worktree == "" {
		fmt.Fprintln(stderr, continuationAdoptUsage)
		return 10
	}
	c, err := adoptableBinding(root, id)
	if err != nil {
		fmt.Fprintf(stderr, "evolve continuation adopt: %v\n", err)
		return 1
	}
	a, err := core.AdoptContinuationForConsole(context.Background(), core.ConsoleAdoptRequest{ProjectRoot: root, Worktree: *worktree, ItemID: id, Binding: c, Warn: stderr})
	if err != nil {
		fmt.Fprintf(stderr, "evolve continuation adopt: %s: %v\n", id, err)
		return 1
	}
	fmt.Fprintf(stdout, "adopted the continuation of cycle %d for %s: worktree %s (branch %s, base %s); findings %s\n",
		c.Cycle, id, a.Worktree, a.Branch, a.BaseSHA, a.FindingsFile)
	return 0
}

func adoptableBinding(root, id string) (continuation.Continuation, error) {
	opts := inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}
	if path := inboxmover.ResolveDispatchState(opts, id).Path; path != "" {
		item, _, err := inboxbatch.LoadFile(path)
		if err != nil {
			return continuation.Continuation{}, fmt.Errorf("item %s unreadable: %w", path, err)
		}
		if item.Continuation != nil && item.Continuation.SnapshotSHA != "" {
			return continuation.ExpandHostPaths(*item.Continuation), nil
		}
	}
	c, bound, err := continuation.ReadRegistryEntry(root, id)
	if err != nil {
		return continuation.Continuation{}, fmt.Errorf("registry unreadable at %s: %w", continuation.RegistryPath(root), err)
	}
	if !bound || c.SnapshotSHA == "" {
		return continuation.Continuation{}, fmt.Errorf("%s has no continuation binding: no stamp on the item and none in %s", id, continuation.RegistryPath(root))
	}
	return continuation.ExpandHostPaths(c), nil
}
