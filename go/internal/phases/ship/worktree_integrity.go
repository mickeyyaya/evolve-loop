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
	ok, detail := auditBindingSatisfied(s.ctx, s.opts, s.worktree, stagedTree)
	if ok {
		s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship]   OK: pre-commit tree drift (audit=%s staged=%s) explained%s — accepted", s.opts.internalAuditBoundTreeSHA, stagedTree, detail))
		return nil
	}
	return shipErr(core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, core.StageAtomicShip,
		fmt.Sprintf("INTEGRITY BREACH (pre-commit): audit-bound tree SHA %s != staged tree SHA %s — refused to commit; worktree changes preserved (staged) for operator triage%s",
			s.opts.internalAuditBoundTreeSHA, stagedTree, detail),
		"audit_bound_tree", s.opts.internalAuditBoundTreeSHA, "worktree_tree", stagedTree, "phase", "pre-commit")
}

func verifyCommittedTree(ctx context.Context, opts *Options, landed string) (string, string, error) {
	committedTree, _ := captureGitOutput(ctx, opts, "rev-parse", landed+"^{tree}")
	committedTree = strings.TrimSpace(committedTree)
	bound := opts.internalAuditBoundTreeSHA
	if bound == "" || committedTree == "" {
		return committedTree, "", nil
	}
	if bound == committedTree {
		return committedTree, fmt.Sprintf("[ship] OK: tree-SHA binding verified (audit=%s committed=%s)", bound, committedTree), nil
	}
	ok, detail := auditBindingSatisfied(ctx, opts, "", committedTree)
	if ok {
		return committedTree, fmt.Sprintf("[ship] OK: post-push tree drift (audit=%s committed=%s) explained%s — accepted", bound, committedTree, detail), nil
	}
	return "", "", shipErr(core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, core.StagePostShip,
		fmt.Sprintf("INTEGRITY BREACH: audit-bound tree SHA %s != committed tree SHA %s — worktree-to-main tree drift detected%s", bound, committedTree, detail),
		"audit_bound_tree", bound, "committed_tree", committedTree, "phase", "post-push")
}

func auditBindingSatisfied(ctx context.Context, opts *Options, worktree, actual string) (bool, string) {
	if opts.internalAuditBoundTreeSHA == actual {
		return true, ""
	}
	carried, carry := carrySatisfied(ctx, opts, worktree, actual)
	if carried {
		return true, carry
	}
	ok, offending := treeDriftExplainedByConsumption(ctx, opts, worktree, opts.internalAuditBoundTreeSHA, actual)
	if ok {
		return true, fmt.Sprintf(" (sanctioned inbox consumption, %d path(s))", len(opts.internalConsumedPaths))
	}
	return false, offending + " (carry not re-proven: " + carry + ")"
}
