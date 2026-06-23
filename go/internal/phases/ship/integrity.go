package ship

import (
	"context"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/guards"
)

// verifyNoControlPlaneEdits is the post-hoc backstop of the pipeline integrity
// boundary (ADR-0064). The real-time role-gate hook only intercepts Edit/Write;
// a phase could still mutate the control plane via Bash (sed -i, redirection) or
// any non-tool channel. This gate runs at the --class cycle ship chokepoint and
// rejects a commit whose diff touches the integrity surface (the deterministic
// gates, metric SSOT, guards, campaign contract, grading rubrics, hook wiring) —
// regardless of HOW the file was changed. A cycle may not edit the gate that
// grades it.
//
// It runs ONLY for ClassCycle (see verifyClass), so operator-driven control-plane
// changes shipped via `evolve ship --class manual` are exempt by construction —
// that is the sanctioned path for hardening a gate.
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

// cycleChangedPaths returns every path the cycle would commit: tracked changes vs
// HEAD (modified/deleted/renamed) unioned with untracked new files — so the
// protected-path check sees the file no matter how it was introduced.
func cycleChangedPaths(ctx context.Context, opts *Options) ([]string, error) {
	tracked, err := captureGitOutput(ctx, opts, "diff", "--name-only", "HEAD")
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
