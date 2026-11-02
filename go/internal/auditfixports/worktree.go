package auditfixports

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditfix"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

type Worktree struct {
	Git func(dir string) gitexec.Git
}

var _ auditfix.Worktree = Worktree{}

func (w Worktree) Snapshot(ctx context.Context, dir string) (string, error) {
	tree, err := w.Git(dir).TreeOfIndexCopy(ctx, []string{"add", "-A", "--", "."})
	if err != nil {
		return "", fmt.Errorf("auditfixports: snapshot %s: %w", dir, err)
	}
	return tree, nil
}

func (w Worktree) Diff(ctx context.Context, dir, base string) (auditfix.Patch, error) {
	current, err := w.Snapshot(ctx, dir)
	if err != nil {
		return auditfix.Patch{}, err
	}
	git := w.Git(dir)
	numstat, err := git.Output(ctx, diffArgs("--numstat", base, current)...)
	if err != nil {
		return auditfix.Patch{}, fmt.Errorf("auditfixports: diff: %w", err)
	}
	diff, err := git.Output(ctx, diffArgs(base, current)...)
	if err != nil {
		return auditfix.Patch{}, fmt.Errorf("auditfixports: diff: %w", err)
	}
	changes, err := parseNumstat(numstat)
	return auditfix.Patch{Changes: changes, Diff: diff}, err
}

func parseNumstat(out string) ([]auditfix.Change, error) {
	var changes []auditfix.Change
	for _, line := range strings.Split(out, "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 {
			continue
		}
		added, errAdded := numstatCount(fields[0])
		deleted, errDeleted := numstatCount(fields[1])
		if err := errors.Join(errAdded, errDeleted); err != nil {
			return nil, fmt.Errorf("auditfixports: numstat %q: %w", line, err)
		}
		changes = append(changes, auditfix.Change{Path: fields[2], Lines: added + deleted})
	}
	return changes, nil
}

func numstatCount(field string) (int, error) {
	if field == "-" {
		return 1, nil
	}
	return strconv.Atoi(field)
}

func (w Worktree) Restore(ctx context.Context, dir, base string) error {
	current, err := w.Snapshot(ctx, dir)
	if err != nil {
		return err
	}
	git := w.Git(dir)
	status, err := git.Output(ctx, diffArgs("--name-status", base, current)...)
	if err != nil {
		return fmt.Errorf("auditfixports: restore: %w", err)
	}
	for _, line := range strings.Split(status, "\n") {
		kind, path, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		if err := (restorer{git: git, dir: dir, base: base}).path(ctx, kind, path); err != nil {
			return err
		}
	}
	return nil
}

func diffArgs(args ...string) []string {
	return append([]string{"-c", "core.quotePath=false", "diff", "--no-renames"}, args...)
}

type restorer struct {
	git       gitexec.Git
	dir, base string
}

func (r restorer) path(ctx context.Context, kind, path string) error {
	if kind == "A" {
		if err := os.Remove(filepath.Join(r.dir, path)); err != nil {
			return fmt.Errorf("auditfixports: restore %s: %w", path, err)
		}
		return nil
	}
	if err := r.git.Run(ctx, "restore", "--source="+r.base, "--worktree", "--", path); err != nil {
		return fmt.Errorf("auditfixports: restore %s: %w", path, err)
	}
	return nil
}
