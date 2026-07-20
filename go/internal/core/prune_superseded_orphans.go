// prune_superseded_orphans.go — housekeeping walker for stale orphan `cycle-*`
// branches (cycle 962, dependent on carryforward-real-cherrypick-filter).
//
// Over a fleet campaign the local ref namespace accumulates `cycle-*` branches
// whose work has already landed on the base under a different sha. Left alone
// they silently regrow the carry-forward candidate backlog. This walker reuses
// the Task-1 supersession screen (refSuperseded) to flag those functional
// duplicates and prune the stale refs — honoring
// verify_remote_pr_before_branch_delete: a branch with an open PR (or remote
// presence) is flagged but NEVER deleted. Distinct, not-yet-landed
// (different-goal) orphans are left untouched.
package core

import (
	"context"
	"fmt"
	"strings"
)

// OrphanVerdict is the per-branch outcome of a PruneSupersededOrphans walk.
type OrphanVerdict struct {
	// Ref is the branch's short name (e.g. "cycle-100").
	Ref string
	// Superseded is true when the branch is already landed on base (ancestor
	// or patch-id functional duplicate).
	Superseded bool
	// Pruned is true only when the branch was both Superseded and safe to
	// delete (hasOpenPR reported false), and the delete succeeded.
	Pruned bool
}

// PruneSupersededOrphans walks the local `cycle-*` branches in dir and, for
// each, decides whether it is a superseded functional duplicate of base and
// whether it may be pruned. A superseded branch is deleted (`git branch -D`)
// ONLY when hasOpenPR(ref) reports false — an open PR / remote leaves it
// flagged-but-kept (verify_remote_pr_before_branch_delete). Non-superseded
// (different-goal) branches are reported untouched. An error from any git call
// or from hasOpenPR aborts the walk; it never silently skips a branch.
func PruneSupersededOrphans(ctx context.Context, dir, base string, hasOpenPR func(ref string) (bool, error)) ([]OrphanVerdict, error) {
	out, code, err := gitCapture(ctx, dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/cycle-*")
	if err != nil {
		return nil, fmt.Errorf("prune orphans: list cycle-* refs: %w", err)
	}
	if code != 0 {
		return nil, fmt.Errorf("prune orphans: for-each-ref exited %d", code)
	}

	var verdicts []OrphanVerdict
	for _, ref := range strings.Split(out, "\n") {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		superseded, err := refSuperseded(ctx, dir, ref, base)
		if err != nil {
			return nil, err
		}
		v := OrphanVerdict{Ref: ref, Superseded: superseded}
		if superseded {
			open, err := hasOpenPR(ref)
			if err != nil {
				return nil, fmt.Errorf("prune orphans: hasOpenPR(%s): %w", ref, err)
			}
			if !open {
				if _, code, err := gitCapture(ctx, dir, "branch", "-D", ref); err != nil {
					return nil, fmt.Errorf("prune orphans: delete %s: %w", ref, err)
				} else if code != 0 {
					return nil, fmt.Errorf("prune orphans: git branch -D %s exited %d", ref, code)
				}
				v.Pruned = true
			}
		}
		verdicts = append(verdicts, v)
	}
	return verdicts, nil
}
