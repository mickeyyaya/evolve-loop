package ship

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipmanifest"
)

// manifest.go — ship-bind tree-manifest reconciliation (shadow + enforce).
//
// Cycle-653 second seam: ship binds the whole `git diff HEAD` tree, so any
// path present in the worktree ships (or blocks) regardless of whether the
// cycle's build/TDD phases declared it. reconcileManifest reconciles the paths
// ship is about to bind against the cycle's DECLARED file manifest (paths named
// in build-report.md + test-report.md).
//
// Two modes (opts.ManifestGate, config-sourced; default shadow — the
// ReportSizeGate "new gates default shadow" precedent):
//   - shadow (default): log out-of-manifest paths, never block — behavior-preserving.
//   - enforce: FAIL CLOSED on any out-of-manifest path — the cross-lane
//     untracked-leak guard (inbox `ship-stage-explicit-paths`, cycle-645).
//
// BEFORE enabling enforce in production (the deferred policy.json→Options
// wiring), prerequisites (2026-07-14 review):
//   1. DONE (2026-07-14): shipmanifest's pathToken now also extracts bare root-level
//      filenames (CHANGELOG.md, go.mod) via an extension allow-list, so a legit
//      root-file change is no longer a FALSE-BLOCK under enforce.
//   2. DONE (cycle-1064): the enforce branch carries the dedicated
//      core.CodeManifestGate (mirroring CodeCommitPrefixGate) instead of reusing
//      CodeGitStageFailed, so the ledger/debugger can tell a manifest block from
//      a real `git add` failure; router.shipLocalCodes routes it to the debugger.
//   3. DONE (cycle-1064): policy.json `gates.manifest_gate` resolves through
//      policy.GatesConfig() into ship.Config.ManifestGate → Options.ManifestGate,
//      so enforce is operator-activatable without a code edit. Default stays
//      "shadow" — behavior-preserving.

// ManifestGateEnforce is the opts.ManifestGate value that switches the gate from
// shadow (log-only) to fail-closed. Any other value (including "") is shadow.
const ManifestGateEnforce = "enforce"

// manifestGateEnforced reports whether the manifest gate BLOCKS on out-of-
// manifest paths (enforce) vs only logs them (shadow, the default). Pure;
// config-sourced via opts.ManifestGate, never a code literal.
func manifestGateEnforced(mode string) bool {
	return mode == ManifestGateEnforce
}

// reconcileManifest reconciles the paths ship is about to bind against the
// cycle's DECLARED file manifest (build-report.md + test-report.md). Changed
// set = worktree porcelain dirt plus commits already on the cycle branch (same
// inputs detectColliders binds). In SHADOW mode (the default) it only REPORTS
// out-of-manifest paths and returns nil — behavior-preserving. In ENFORCE mode
// it FAILS CLOSED on any out-of-manifest path: refusing to commit files no
// phase declared, which under a fleet are typically a sibling lane's untracked
// leak (cycle-645) whose commit reddens main. A loud, recoverable block beats a
// silent contaminated commit. Returns nil (skips) when no reports are readable.
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
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] manifest-gate: OK — all %d bound path(s) covered by the declared build/TDD manifest", len(changed)))
		return nil
	}
	if manifestGateEnforced(opts.ManifestGate) {
		// FAIL-CLOSED — see reconcileManifest's doc + the error string below.
		return shipErr(core.CodeManifestGate, core.ShipClassPrecondition, core.StageAtomicShip,
			fmt.Sprintf("ship: manifest-gate (enforce): refusing to commit %d path(s) no build/TDD report declared (likely a cross-lane untracked leak): %s", len(extras), strings.Join(extras, ", ")),
			"out_of_manifest", strings.Join(extras, ","))
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] manifest-gate: %d out-of-manifest path(s) about to be bound (SHADOW — would block under enforce): %s", len(extras), strings.Join(extras, ", ")))
	return nil
}
