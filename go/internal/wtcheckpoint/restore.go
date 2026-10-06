package wtcheckpoint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

type RestoreResult struct {
	Ref      string `json:"ref"`
	Dir      string `json:"dir"`
	Base     string `json:"base"`
	Created  bool   `json:"created"`
	Detached bool   `json:"detached"`
}

type checkpoint struct{ ref, base, staged, full string }

func Restore(ctx context.Context, h Hub, ref, into string) (RestoreResult, error) {
	cp, err := resolveCheckpoint(ctx, h.git(), ref)
	if err != nil {
		return RestoreResult{}, err
	}
	res, err := prepareTarget(ctx, h, into, cp)
	if err != nil {
		return RestoreResult{}, err
	}
	if err := applyCheckpoint(ctx, gitexec.Isolated(res.Dir), cp); err != nil {
		return RestoreResult{}, fmt.Errorf("wtcheckpoint: restore %s into %s: %w", cp.ref, res.Dir, err)
	}
	return res, nil
}

func resolveCheckpoint(ctx context.Context, g gitexec.Git, ref string) (checkpoint, error) {
	name := refPrefix + strings.TrimPrefix(ref, refPrefix)
	out, err := g.Output(ctx, "rev-parse", name+"~2^{commit}", name+"^^{tree}", name+"^{tree}")
	if err != nil {
		return checkpoint{}, fmt.Errorf("wtcheckpoint: %s is not a checkpoint: %w", name, err)
	}
	ids := strings.Fields(out)
	if len(ids) != 3 {
		return checkpoint{}, fmt.Errorf("wtcheckpoint: %s resolved to %q, want base, staged and full", name, out)
	}
	return checkpoint{ref: name, base: ids[0], staged: ids[1], full: ids[2]}, nil
}

func prepareTarget(ctx context.Context, h Hub, into string, cp checkpoint) (RestoreResult, error) {
	res := RestoreResult{Ref: cp.ref, Base: cp.base}
	abs, err := filepath.Abs(into)
	if err != nil {
		return res, err
	}
	if _, err := os.Lstat(abs); errors.Is(err, os.ErrNotExist) {
		retry := gitexec.WorktreeAddRetry{Retryable: gitexec.RetryableWorktreeAddFailure}
		if _, stderr, code, err := h.git().AddWorktreeWithRetry(ctx, retry, "--detach", abs, cp.base); err != nil || code != 0 {
			return res, fmt.Errorf("wtcheckpoint: git worktree add %s rc=%d err=%v: %s", abs, code, err, strings.TrimSpace(stderr))
		}
		res.Dir, res.Created, res.Detached = abs, true, true
		return res, nil
	} else if err != nil {
		return res, err
	}
	w, err := h.Worktree(ctx, abs)
	if err != nil {
		return res, err
	}
	res.Dir = w.Dir
	res.Detached, err = readyExisting(ctx, gitexec.Isolated(w.Dir), cp)
	return res, err
}

func readyExisting(ctx context.Context, tree gitexec.Git, cp checkpoint) (bool, error) {
	dirty, err := tree.DirtyPaths(ctx)
	if err != nil {
		return false, err
	}
	if len(dirty) > 0 {
		return false, fmt.Errorf("wtcheckpoint: refused: %s is dirty (%d path(s): %s); restore never overwrites work", tree.Dir, len(dirty), strings.Join(dirty, ", "))
	}
	head, err := tree.HEAD(ctx)
	if err != nil {
		return false, err
	}
	hits, err := ignoredCollisions(ctx, tree, head, cp)
	if err != nil {
		return false, err
	}
	if len(hits) > 0 {
		return false, fmt.Errorf("wtcheckpoint: refused: %s holds ignored file(s) the restore would overwrite: %s", tree.Dir, strings.Join(hits, ", "))
	}
	if head == cp.base {
		return false, nil
	}
	return true, tree.Run(ctx, "checkout", "-q", "--detach", cp.base)
}

func ignoredCollisions(ctx context.Context, tree gitexec.Git, head string, cp checkpoint) ([]string, error) {
	moved, err := changedPaths(ctx, tree, head, cp.base)
	if err != nil {
		return nil, err
	}
	restored, err := changedPaths(ctx, tree, cp.base, cp.full)
	if err != nil {
		return nil, err
	}
	listed, err := tree.Output(ctx, "ls-files", "--others", "--ignored", "--exclude-standard", "--directory", "-z")
	if err != nil {
		return nil, err
	}
	ignored := map[string]bool{}
	for _, p := range strings.FieldsFunc(listed, func(r rune) bool { return r == 0 }) {
		ignored[p] = true
	}
	var hits []string
	for _, p := range slices.Concat(moved, restored) {
		if ignored[p] || ignoredAncestor(p, ignored) {
			hits = append(hits, p)
		}
	}
	return hits, nil
}

func ignoredAncestor(p string, ignored map[string]bool) bool {
	for d := path.Dir(p); d != "." && d != "/"; d = path.Dir(d) {
		if ignored[d+"/"] {
			return true
		}
	}
	return false
}

func applyCheckpoint(ctx context.Context, tree gitexec.Git, cp checkpoint) error {
	if err := tree.Run(ctx, "read-tree", "-m", "-u", "HEAD", cp.full); err != nil {
		return err
	}
	if err := tree.Run(ctx, "read-tree", cp.staged); err != nil {
		return err
	}
	if _, stderr, code, err := tree.Capture(ctx, "update-index", "-q", "--refresh"); err != nil || code > 1 {
		return fmt.Errorf("update-index --refresh rc=%d err=%v: %s", code, err, strings.TrimSpace(stderr))
	}
	return nil
}
