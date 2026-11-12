package baseattr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

var scratchSeq atomic.Int64

func (a Attributor) Attribute(ctx context.Context, base string, r Reds) (Verdict, error) {
	if !r.Attributable() {
		return Verdict{LaneOnly: slices.Clone(r.Keys), Unjudged: true}, nil
	}
	defer a.serialize(ctx)()
	start := time.Now()
	v, err := a.judge(ctx, base, r)
	v.Took = time.Since(start)
	return v, err
}

func (a Attributor) serialize(ctx context.Context) func() {
	if a.Serialize == nil {
		return func() {}
	}
	return a.Serialize(ctx)
}

func (a Attributor) judge(ctx context.Context, base string, r Reds) (Verdict, error) {
	sha, err := a.Resolve(ctx, base)
	if err != nil {
		return Verdict{}, fmt.Errorf("baseattr: resolve base %q: %w", base, err)
	}
	baseRed, err := a.RunAt(ctx, sha, r)
	if err != nil {
		return Verdict{Base: sha}, err
	}
	v := partition(r.Keys, baseRed)
	v.Base = sha
	if len(v.BaseRed) == 0 {
		return v, nil
	}
	return a.onMain(ctx, v, r.Flags)
}

func partition(asked, baseRed []Key) Verdict {
	var v Verdict
	for _, k := range asked {
		if slices.Contains(baseRed, k) {
			v.BaseRed = append(v.BaseRed, k)
		} else {
			v.LaneOnly = append(v.LaneOnly, k)
		}
	}
	return v
}

func (a Attributor) onMain(ctx context.Context, v Verdict, flags []string) (Verdict, error) {
	main, err := a.Resolve(ctx, a.MainRef)
	if err != nil {
		return v, nil
	}
	v.Main = main
	if main == v.Base {
		v.MainRed = slices.Clone(v.BaseRed)
		return v, nil
	}
	v.MainRed, err = a.RunAt(ctx, main, Reds{Keys: v.BaseRed, Flags: flags})
	return v, err
}

func (a Attributor) Resolve(ctx context.Context, rev string) (string, error) {
	return a.Git.Output(ctx, "rev-parse", "--verify", "--end-of-options", rev+"^{commit}")
}

func (a Attributor) RunAt(ctx context.Context, rev string, r Reds) (red []Key, err error) {
	dir, err := a.checkout(ctx, rev)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, a.dispose(ctx, dir)) }()
	red, err = a.runIn(ctx, dir, r)
	return red, runFailure(rev, a.Timeout, err)
}

func (a Attributor) RunIn(ctx context.Context, tree string, r Reds) ([]Key, error) {
	red, err := a.runIn(ctx, tree, r)
	return red, runFailure(tree, a.Timeout, err)
}

func (a Attributor) runIn(ctx context.Context, tree string, r Reds) ([]Key, error) {
	runCtx, cancel := a.runContext(ctx)
	defer cancel()
	red, err := a.Run(runCtx, moduleDir(tree), r)
	if err != nil {
		return nil, err
	}
	return red, nil
}

func runFailure(name string, limit time.Duration, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("baseattr: run at %s (limit %s): %w", name, limit, err)
}

func (a Attributor) runContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.Timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, a.Timeout)
}

func (a Attributor) checkout(ctx context.Context, rev string) (string, error) {
	scratch := a.Scratch
	if !filepath.IsAbs(scratch) {
		scratch = filepath.Join(a.Git.Dir, scratch)
	}
	dir := filepath.Join(scratch, fmt.Sprintf("wt-%d-%d", os.Getpid(), scratchSeq.Add(1)))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("baseattr: scratch directory: %w", err)
	}
	retry := gitexec.WorktreeAddRetry{Retryable: gitexec.RetryableWorktreeAddFailure}
	_, stderr, code, err := a.Git.AddWorktreeWithRetry(ctx, retry, "--detach", dir, rev)
	if err != nil || code != 0 {
		return "", errors.Join(fmt.Errorf("baseattr: add worktree at %s: exit %d: %s", rev, code, strings.TrimSpace(stderr)), err, a.dispose(ctx, dir))
	}
	return dir, nil
}

func (a Attributor) dispose(ctx context.Context, dir string) error {
	detached := context.WithoutCancel(ctx)
	_, _, _, _ = a.Git.Capture(detached, "worktree", "remove", "--force", dir)
	removeErr := os.RemoveAll(dir)
	_, _, _, _ = a.Git.Capture(detached, "worktree", "prune")
	return removeErr
}

func moduleDir(worktree string) string {
	if _, err := os.Stat(filepath.Join(worktree, "go", "go.mod")); err == nil {
		return filepath.Join(worktree, "go")
	}
	return worktree
}
