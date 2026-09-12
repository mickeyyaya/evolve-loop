package ship

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// verifyStagedTree checks the audit binding before commit, so a mismatch never
// creates an orphan commit. Dry runs skip it because their index is unstaged.
func (s *worktreeShip) verifyStagedTree() error {
	if s.opts.DryRun || s.opts.internalAuditBoundTreeSHA == "" {
		return nil
	}
	stagedTree, err := captureGitOutputAtDir(s.ctx, s.opts, s.worktree, "write-tree")
	if err != nil {
		return err
	}
	stagedTree = strings.TrimSpace(stagedTree)
	if stagedTree == "" {
		return shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageAtomicShip,
			"ship: git write-tree produced no tree SHA — cannot verify audit-bound tree binding before commit",
			"worktree", s.worktree)
	}
	if s.opts.internalAuditBoundTreeSHA == stagedTree {
		s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship]   OK: pre-commit tree-SHA binding verified (audit=%s staged=%s)", s.opts.internalAuditBoundTreeSHA, stagedTree))
		return nil
	}
	ok, offending := auditBindingSatisfied(s.ctx, s.opts, s.worktree, stagedTree)
	if ok {
		s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship]   OK: pre-commit tree drift (audit=%s staged=%s) fully explained by sanctioned inbox consumption (%d path(s)) — accepted", s.opts.internalAuditBoundTreeSHA, stagedTree, len(s.opts.internalConsumedPaths)))
		return nil
	}
	return shipErr(core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, core.StageAtomicShip,
		fmt.Sprintf("INTEGRITY BREACH (pre-commit): audit-bound tree SHA %s != staged tree SHA %s — refused to commit; worktree changes preserved (staged) for operator triage%s",
			s.opts.internalAuditBoundTreeSHA, stagedTree, offending),
		"audit_bound_tree", s.opts.internalAuditBoundTreeSHA, "worktree_tree", stagedTree, "phase", "pre-commit")
}

// verifyCommittedTree checks the same binding after push and returns the tree
// SHA used by ship-binding.json.
func (s *worktreeShip) verifyCommittedTree() (string, error) {
	committedTree, _ := captureGitOutput(s.ctx, s.opts, "rev-parse", "HEAD^{tree}")
	committedTree = strings.TrimSpace(committedTree)
	if s.opts.internalAuditBoundTreeSHA == "" || committedTree == "" {
		return committedTree, nil
	}
	if s.opts.internalAuditBoundTreeSHA == committedTree {
		s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship] OK: tree-SHA binding verified (audit=%s committed=%s)", s.opts.internalAuditBoundTreeSHA, committedTree))
		return committedTree, nil
	}
	ok, offending := auditBindingSatisfied(s.ctx, s.opts, "", committedTree)
	if ok {
		s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship] OK: post-push tree drift (audit=%s committed=%s) fully explained by sanctioned inbox consumption — accepted", s.opts.internalAuditBoundTreeSHA, committedTree))
		return committedTree, nil
	}
	return "", shipErr(core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, core.StagePostShip,
		fmt.Sprintf("INTEGRITY BREACH: audit-bound tree SHA %s != committed tree SHA %s — worktree-to-main tree drift detected%s", s.opts.internalAuditBoundTreeSHA, committedTree, offending),
		"audit_bound_tree", s.opts.internalAuditBoundTreeSHA, "committed_tree", committedTree, "phase", "post-push")
}

// auditBindingSatisfied is the ONE rule both ship paths apply to an audit
// binding: the tree matches, or the drift is fully explained by sanctioned
// inbox consumption. It takes only what the rule needs — no receiver — so the
// direct path can apply the identical decision.
//
// It exists because the direct path shipped for a long time with NO check at
// all while still writing audit_bound_tree_sha into ship-binding.json,
// recording a verification that never happened. A single predicate means the
// two paths cannot disagree about what "bound" means; each still formats its
// own operator-facing message, because pre-commit ("refused to commit;
// changes preserved") and post-push ("drift detected") are genuinely
// different situations for whoever has to read them.
func auditBindingSatisfied(ctx context.Context, opts *Options, worktree, actual string) (bool, string) {
	if opts.internalAuditBoundTreeSHA == actual {
		return true, ""
	}
	return treeDriftExplainedByConsumption(ctx, opts, worktree, opts.internalAuditBoundTreeSHA, actual)
}
