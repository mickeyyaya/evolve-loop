package core

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

func advanceContinuationBase(ctx context.Context, worktree string, cycle int) (string, error) {
	g := gitexec.Git{Dir: worktree, Exec: gitRunner}
	tip, stderr, code, err := g.Capture(ctx, "rev-parse", "--verify", "main^{commit}")
	if err != nil || code != 0 {
		return "", fmt.Errorf("base-advance resolve main rc=%d: %v: %s", code, err, strings.TrimSpace(stderr))
	}
	base := strings.TrimSpace(tip)
	if _, _, code, err := g.Capture(ctx, "merge-base", "--is-ancestor", base, "HEAD"); err == nil && code == 0 {
		return base, nil
	}
	args := append(append([]string{}, snapshotIdentity...), "merge", "--no-edit", "--no-verify", base)
	if _, stderr, code, err := g.Capture(ctx, args...); err != nil || code != 0 {
		conflicts, _, _, _ := g.Capture(ctx, "diff", "--name-only", "--diff-filter=U")
		_, abortStderr, abortCode, abortErr := g.Capture(context.WithoutCancel(ctx), "merge", "--abort")
		return "", fmt.Errorf("base-advance merge %s failed rc=%d: %v: %s; conflicts [%s]; abort rc=%d: %v: %s", base, code, err, strings.TrimSpace(stderr), strings.Join(strings.Fields(conflicts), ", "), abortCode, abortErr, strings.TrimSpace(abortStderr))
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d continuation: base ADVANCED to pinned main %s\n", cycle, base)
	return base, nil
}
