package ship

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
)

// worktreeShip is the transaction for a cycle worktree; its methods run in
// the order shipFromWorktree enforces: resolve, lock, preflight, stage,
// commit, integrate.
type worktreeShip struct {
	ctx      context.Context
	opts     *Options
	result   *RunResult
	branch   string
	worktree string

	cycleBranch string
}

type worktreeChangeState uint8

const (
	worktreeNoChanges worktreeChangeState = iota
	worktreeBranchAhead
	worktreeStagedChanges
)

func newWorktreeShip(ctx context.Context, opts *Options, result *RunResult, branch, worktree string) *worktreeShip {
	return &worktreeShip{
		ctx:      ctx,
		opts:     opts,
		result:   result,
		branch:   branch,
		worktree: worktree,
	}
}

func (s *worktreeShip) run() error {
	s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship] v8.43.0: worktree-aware ship — committing in active_worktree=%s", s.worktree))
	if err := s.resolveCycleBranch(); err != nil {
		return err
	}
	if !s.opts.DryRun {
		release, err := s.opts.acquireShipLock()
		if err != nil {
			return shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageAtomicShip,
				"ship: acquire integrator lock (.evolve/ship.lock): "+err.Error())
		}
		defer release()
	}

	if err := s.preflight(); err != nil {
		return err
	}
	changes, err := s.prepareChanges()
	if err != nil || changes == worktreeNoChanges {
		return err
	}
	var pending string
	if changes == worktreeStagedChanges {
		if pending, err = s.commit(); err != nil {
			return err
		}
	}
	if s.opts.DryRun {
		s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship]   [DRY-RUN] would ff-merge + push %s into %s", s.cycleBranch, s.branch))
		return nil
	}
	return s.land(pending)
}

func (s *worktreeShip) resolveCycleBranch() error {
	branch, err := captureGitOutputAtDir(s.ctx, s.opts, s.worktree, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return shipErr(core.CodeWorktreeResolve, core.ShipClassPrecondition, core.StageAtomicShip,
			fmt.Sprintf("ship: could not resolve cycle branch from worktree %s: %v", s.worktree, err),
			"worktree", s.worktree, "git_err", err.Error())
	}
	s.cycleBranch = strings.TrimSpace(branch)
	if s.cycleBranch == "" {
		return shipErr(core.CodeWorktreeResolve, core.ShipClassPrecondition, core.StageAtomicShip,
			"ship: empty cycle branch from worktree "+s.worktree, "worktree", s.worktree)
	}
	s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship]   cycle branch: %s", s.cycleBranch))
	return nil
}

func (s *worktreeShip) preflight() error {
	colliders, err := detectColliders(s.ctx, s.opts, s.worktree, s.branch, s.cycleBranch)
	if err != nil {
		return err
	}
	if len(colliders) > 0 {
		return shipErr(core.CodeGitFFMergeDiverged, core.ShipClassPrecondition, core.StageAtomicShip,
			fmt.Sprintf("ship: untracked files in main working tree would be overwritten by merge: %s", strings.Join(colliders, ", ")),
			"colliders", strings.Join(colliders, ","))
	}
	return reconcileManifest(s.ctx, s.opts, s.result, s.worktree, s.branch, s.cycleBranch)
}

func (s *worktreeShip) stageTrackedEdits() error {
	var errTail strings.Builder
	exit, err := s.opts.run(s.ctx, "git", []string{"-C", s.worktree, "add", "-u"}, io.Discard, &errTail)
	if err != nil || exit != 0 {
		return shipErr(core.CodeGitStageFailed, core.ShipClassTransient, core.StageAtomicShip,
			fmt.Sprintf("ship: git add failed (rc=%d): %v: add -u: %s", exit, err, strings.TrimSpace(errTail.String())),
			"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(err), "worktree", s.worktree)
	}
	return nil
}

func (s *worktreeShip) prepareChanges() (worktreeChangeState, error) {
	if !s.opts.DryRun {
		if err := s.stageTrackedEdits(); err != nil {
			return worktreeNoChanges, err
		}
		_ = discardBinaryChurn(s.ctx, s.opts, s.worktree)
		consumeCommittedItems(s.ctx, s.opts, s.result, s.worktree)
		if err := stageExplicitPaths(s.ctx, s.opts, s.result, s.worktree); err != nil {
			return worktreeNoChanges, err
		}
	}

	exit, err := s.opts.run(s.ctx, "git", []string{"-C", s.worktree, "diff", "--cached", "--quiet"}, io.Discard, io.Discard)
	if err != nil {
		return worktreeNoChanges, shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageAtomicShip,
			"ship: worktree diff --cached --quiet failed: "+err.Error(), "git_err", err.Error(), "worktree", s.worktree)
	}
	if exit != 0 {
		return worktreeStagedChanges, nil
	}
	aheadOutput, err := captureGitOutput(s.ctx, s.opts, "rev-list", "--count", s.branch+".."+s.cycleBranch)
	if err != nil {
		return worktreeNoChanges, err
	}
	ahead := strings.TrimSpace(aheadOutput)
	if ahead == "0" || ahead == "" {
		if err := s.requireLaneOnOrigin(); err != nil {
			return worktreeNoChanges, err
		}
		s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship] no changes in worktree AND branch not ahead of %s; exiting cleanly", s.branch))
		return worktreeNoChanges, nil
	}
	s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship]   no uncommitted worktree changes but branch is %s commit(s) ahead; will merge", ahead))
	return worktreeBranchAhead, nil
}

func (s *worktreeShip) commit() (string, error) {
	footer, err := buildDiffFooterAtDir(s.ctx, s.opts, s.worktree)
	if err != nil {
		return "", err
	}
	message := s.opts.CommitMessage + footer
	if err := runCommitPrefixGate(s.ctx, s.opts, message, s.worktree); err != nil {
		return "", shipErr(core.CodeCommitPrefixGate, core.ShipClassPrecondition, core.StageAtomicShip,
			"ship: commit-prefix-gate rejected worktree commit (Layer 1 of ADR-0012). To bypass for manual class only: --bypass-prefix-gate: "+err.Error(),
			"gate_err", err.Error(), "worktree", s.worktree)
	}
	if err := s.verifyStagedTree(); err != nil {
		return "", err
	}
	if s.opts.DryRun {
		s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship]   [DRY-RUN] would commit in worktree on %s", s.cycleBranch))
		return "", nil
	}
	commit, err := s.commitObject(message)
	if err != nil {
		return "", err
	}
	s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship]   OK: committed in worktree on %s as %s; the lane ref moves after the landing intent", s.cycleBranch, commit))
	return commit, nil
}

func (s *worktreeShip) commitObject(message string) (string, error) {
	tree, err := captureGitOutputAtDir(s.ctx, s.opts, s.worktree, "write-tree")
	var commit string
	if err == nil {
		commit, err = captureGitOutputAtDir(s.ctx, s.opts, s.worktree, "-c", "commit.gpgsign=false", "commit-tree", strings.TrimSpace(tree), "-p", "HEAD", "-m", message)
	}
	if commit = strings.TrimSpace(commit); err != nil || commit == "" {
		return "", shipErr(core.CodeGitCommitFailed, core.ShipClassPrecondition, core.StageAtomicShip,
			fmt.Sprintf("ship: git commit-tree in worktree failed: %v", err), "git_err", errStr(err), "worktree", s.worktree)
	}
	return commit, nil
}

func (s *worktreeShip) requireLaneOnOrigin() error {
	out, err := captureGitOutput(s.ctx, s.opts, "rev-list", "origin/"+s.branch+".."+s.cycleBranch)
	if err != nil {
		s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship] WARN: no origin/%s to compare the lane with (%v); nothing to commit", s.branch, err))
		return nil
	}
	stranded := strandedLandings(s.opts, strings.Fields(out))
	if len(stranded) == 0 {
		return nil
	}
	return shipErr(core.CodeGitLaneNotOnOrigin, core.ShipClassIntegrity, core.StageAtomicShip,
		fmt.Sprintf("ship: nothing to commit, but the lane %s holds %d journaled lane commit(s) that origin/%s does not hold, and their landing is not complete (newest %s). A PASS with no push reports a landing that did not occur. %s",
			s.cycleBranch, len(stranded), s.branch, stranded[0], boundarySteps(stranded[0])),
		"cycle_branch", s.cycleBranch, "branch", s.branch, "stranded", strings.Join(stranded, ","))
}

func strandedLandings(opts *Options, commits []string) []string {
	journal := readShipJournal(opts.ProjectRoot)
	var stranded []string
	for _, sha := range commits {
		entry, journaled := journal[sha]
		if journaled && entry.Class == string(ClassCycle) && !landingComplete(opts, entry.Cycle) {
			stranded = append(stranded, sha)
		}
	}
	return stranded
}

func landingComplete(opts *Options, cycle int) bool {
	if cycle <= 0 {
		return false
	}
	in, found, err := landing.ReadIntent(intentPath(opts, cycle))
	return err == nil && found && in.Status == landing.IntentComplete
}
