package ship

import (
	"context"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

// See ADR-0064.
func verifyNoControlPlaneEdits(ctx context.Context, opts *Options, res *RunResult) error {
	paths, err := cycleChangedPaths(ctx, opts)
	if err != nil {
		return err
	}
	var hits []string
	for _, p := range paths {
		if guards.IsProtectedSurface(p) {
			hits = append(hits, p)
		}
	}
	if len(hits) > 0 {
		return shipErr(core.CodeControlPlaneViolation, core.ShipClassPrecondition, core.StageVerifyClass,
			"INTEGRITY VIOLATION: a --class cycle commit modifies the pipeline control plane "+
				"(a cycle may not edit the gate/metric/guard/contract that grades it): "+
				strings.Join(hits, ", ")+
				". Ship an intentional control-plane change with `evolve ship --class manual` instead.",
			"protected_paths", strings.Join(hits, ","))
	}
	res.Logs = append(res.Logs, "[ship] OK: no control-plane (integrity-surface) paths in cycle diff")
	return nil
}

func cycleChangedPaths(ctx context.Context, opts *Options) ([]string, error) {
	tracked, err := captureGitOutput(ctx, opts, "diff", "--name-only", "--no-renames", "HEAD")
	if err != nil {
		return nil, err
	}
	untracked, err := captureGitOutput(ctx, opts, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	// `git diff HEAD` (tracked) and `ls-files --others` (untracked) are disjoint by
	// definition, so a plain concat is the complete set — no dedup needed.
	return append(splitNonEmpty(tracked), splitNonEmpty(untracked)...), nil
}
