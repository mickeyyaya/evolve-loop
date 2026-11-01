package core

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

var ErrContinuationConflict = errors.New("continuation: the snapshot conflicts with main")

type ConsoleAdoption struct {
	Worktree     string
	Branch       string
	BaseSHA      string
	FindingsFile string
}

type ConsoleAdoptRequest struct {
	ProjectRoot string
	Worktree    string
	ItemID      string
	Binding     continuation.Continuation
	Warn        io.Writer
}

func AdoptContinuationForConsole(ctx context.Context, r ConsoleAdoptRequest) (ConsoleAdoption, error) {
	projectRoot, worktree, itemID, c := r.ProjectRoot, r.Worktree, r.ItemID, r.Binding
	if r.Warn == nil {
		r.Warn = io.Discard
	}
	if err := validateContinuation(ctx, projectRoot, &c); errors.Is(err, ErrContinuationConflict) {
		return ConsoleAdoption{}, fmt.Errorf("%w: snapshot %s of cycle %d: %s", ErrContinuationConflict, c.SnapshotSHA, c.Cycle, strings.Join(snapshotConflicts(ctx, projectRoot, c.SnapshotSHA), ", "))
	} else if err != nil {
		return ConsoleAdoption{}, err
	}
	if !filepath.IsAbs(worktree) {
		worktree = filepath.Join(projectRoot, worktree)
	}
	worktree = filepath.Clean(worktree)
	a := ConsoleAdoption{Worktree: worktree, Branch: "adopt/" + itemID, FindingsFile: worktree + ".findings.md"}
	g := gitexec.Git{Dir: projectRoot, Exec: gitRunner}
	if _, stderr, code, err := g.Capture(ctx, "worktree", "add", "-b", a.Branch, worktree, c.SnapshotSHA); err != nil || code != 0 {
		return ConsoleAdoption{}, fmt.Errorf("continuation adopt: git worktree add -b %s %s %s: rc=%d %v: %s", a.Branch, worktree, c.SnapshotSHA, code, err, strings.TrimSpace(stderr))
	}
	base, err := advanceContinuationBase(ctx, worktree, c.Cycle)
	if err == nil {
		a.BaseSHA = base
		err = os.WriteFile(a.FindingsFile, []byte(consoleFindings(c, a, r.Warn)), 0o644)
	}
	if err != nil {
		return ConsoleAdoption{}, errors.Join(fmt.Errorf("continuation adopt: %w", err), undoConsoleWorktree(ctx, g, a))
	}
	return a, nil
}

func undoConsoleWorktree(ctx context.Context, g gitexec.Git, a ConsoleAdoption) error {
	ctx = context.WithoutCancel(ctx)
	var errs []error
	for _, args := range [][]string{{"worktree", "remove", "--force", a.Worktree}, {"branch", "-D", a.Branch}} {
		if _, stderr, code, err := g.Capture(ctx, args...); err != nil || code != 0 {
			errs = append(errs, fmt.Errorf("undo: git %s: rc=%d %v: %s", strings.Join(args, " "), code, err, strings.TrimSpace(stderr)))
		}
	}
	return errors.Join(errs...)
}

func snapshotConflicts(ctx context.Context, dir, snapshot string) []string {
	out, _, _ := gitCapture(ctx, dir, "merge-tree", "--write-tree", "--name-only", "--no-messages", "main", snapshot)
	return strings.Split(strings.TrimSpace(out), "\n")[1:]
}

func consoleFindings(c continuation.Continuation, a ConsoleAdoption, warn io.Writer) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Continuation of cycle %d\n\n- snapshot: %s\n- branch of the lane: %s\n- base after the merge of main: %s\n- worktree: %s\n\n",
		c.Cycle, c.SnapshotSHA, c.Branch, a.BaseSHA, a.Worktree)
	b.WriteString("## Prior findings\n\n")
	b.WriteString(cmp.Or(readContinuationFindings(c.FindingsPath, warn), "none"))
	b.WriteString("\n\n## Red predicates\n\n")
	b.WriteString(cmp.Or(continuationRepairFirst(&c, a.Worktree, warn), "none"))
	b.WriteString("\n")
	return b.String()
}
