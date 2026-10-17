package ship

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipmanifest"
)

const ManifestGateEnforce = "enforce"

func manifestGateEnforced(mode string) bool {
	return mode == ManifestGateEnforce
}

func reconcileManifest(ctx context.Context, opts *Options, res *RunResult, worktree, branch, cycleBranch string) error {
	if opts.WorkspacePath == "" {
		return nil
	}
	manifest := shipmanifest.Declared(opts.WorkspacePath, shipmanifest.ReportFiles())
	if len(manifest) == 0 {
		res.Logs = append(res.Logs, "[ship] manifest-gate: no readable phase reports in workspace — reconciliation skipped")
		return nil
	}
	changedSet := map[string]bool{}
	if out, err := captureGitOutputAtDir(ctx, opts, worktree, "status", "--porcelain", "-uall"); err == nil {
		for _, p := range shipmanifest.ChangedPaths(out) {
			changedSet[p] = true
		}
	}
	if out, err := captureGitOutputAtDir(ctx, opts, worktree, "diff", "--name-only", branch, cycleBranch); err == nil {
		for _, line := range strings.Split(out, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				changedSet[line] = true
			}
		}
	}
	changed := make([]string, 0, len(changedSet))
	for p := range changedSet {
		changed = append(changed, p)
	}
	sort.Strings(changed)
	extras := shipmanifest.OutOfManifest(changed, manifest)
	if len(extras) == 0 {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] manifest-gate: OK — all %d bound path(s) covered by the declared phase-report manifest (build, tdd, test-amplification)", len(changed)))
		return nil
	}
	if manifestGateEnforced(opts.ManifestGate) {
		return shipErr(core.CodeManifestGate, core.ShipClassPrecondition, core.StageAtomicShip,
			fmt.Sprintf("ship: manifest-gate (enforce): refusing to commit %d path(s) no phase report (build, tdd, test-amplification) declared (likely a cross-lane untracked leak): %s", len(extras), strings.Join(extras, ", ")),
			"out_of_manifest", strings.Join(extras, ","))
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] manifest-gate: %d out-of-manifest path(s) about to be bound (SHADOW — would block under enforce): %s", len(extras), strings.Join(extras, ", ")))
	return nil
}
