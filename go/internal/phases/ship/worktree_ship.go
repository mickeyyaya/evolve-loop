package ship

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// worktreeShip is the transaction for a cycle worktree. Its methods follow
// the mutation order enforced by shipFromWorktree: resolve, lock, preflight,
// stage, commit, integrate, verify, then record the binding.
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
	if changes == worktreeStagedChanges {
		if err := s.commit(); err != nil {
			return err
		}
	}
	if s.opts.DryRun {
		s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship]   [DRY-RUN] would ff-merge + push %s into %s", s.cycleBranch, s.branch))
		return nil
	}
	return s.integrate()
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

func (s *worktreeShip) prepareChanges() (worktreeChangeState, error) {
	if !s.opts.DryRun {
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
		s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship] no changes in worktree AND branch not ahead of %s; exiting cleanly", s.branch))
		return worktreeNoChanges, nil
	}
	s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship]   no uncommitted worktree changes but branch is %s commit(s) ahead; will merge", ahead))
	return worktreeBranchAhead, nil
}

func (s *worktreeShip) commit() error {
	footer, err := buildDiffFooterAtDir(s.ctx, s.opts, s.worktree)
	if err != nil {
		return err
	}
	message := s.opts.CommitMessage + footer
	if err := runCommitPrefixGate(s.ctx, s.opts, message, s.worktree); err != nil {
		return shipErr(core.CodeCommitPrefixGate, core.ShipClassPrecondition, core.StageAtomicShip,
			"ship: commit-prefix-gate rejected worktree commit (Layer 1 of ADR-0012). To bypass for manual class only: --bypass-prefix-gate: "+err.Error(),
			"gate_err", err.Error(), "worktree", s.worktree)
	}
	if err := s.verifyStagedTree(); err != nil {
		return err
	}
	if s.opts.DryRun {
		s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship]   [DRY-RUN] would commit in worktree on %s", s.cycleBranch))
		return nil
	}
	exit, err := s.opts.run(s.ctx, "git", []string{"-C", s.worktree, "-c", "commit.gpgsign=false", "commit", "-m", message},
		s.opts.Stdout, s.opts.Stderr)
	if err != nil || exit != 0 {
		return shipErr(core.CodeGitCommitFailed, core.ShipClassPrecondition, core.StageAtomicShip,
			fmt.Sprintf("ship: git commit in worktree failed (rc=%d): %v", exit, err),
			"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(err), "worktree", s.worktree)
	}
	s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship]   OK: committed in worktree on %s", s.cycleBranch))
	return nil
}

func (s *worktreeShip) integrate() error {
	if exit, err := s.opts.run(s.ctx, "git", []string{"checkout", "HEAD", "--", "go/evolve"}, io.Discard, io.Discard); exit != 0 || err != nil {
		fmt.Fprintf(s.opts.Stderr, "[ship] WARN: could not reset go/evolve to HEAD (exit=%d, err=%v); ff-merge may still fail if it is dirty\n", exit, err)
	}

	exit, err := s.opts.run(s.ctx, "git", []string{"merge", "--ff-only", s.cycleBranch}, s.opts.Stdout, s.opts.Stderr)
	if err != nil || exit != 0 {
		if s.opts.envBool(ipcenv.FleetKey) {
			return shipErr(core.CodeGitFleetRebaseNeeded, core.ShipClassTransient, core.StageAtomicShip,
				fmt.Sprintf("ship: fleet ff-merge %s into %s diverged (a peer cycle moved %s mid-pipeline); rebase + re-verify the merged tree, then re-ship", s.cycleBranch, s.branch, s.branch),
				"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(err), "cycle_branch", s.cycleBranch, "branch", s.branch)
		}
		return shipErr(core.CodeGitFFMergeDiverged, core.ShipClassPrecondition, core.StageAtomicShip,
			fmt.Sprintf("ship: ff-merge %s into %s failed (rc=%d; divergent history): %v", s.cycleBranch, s.branch, exit, err),
			"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(err), "cycle_branch", s.cycleBranch, "branch", s.branch)
	}
	s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship]   OK: ff-merged %s into %s", s.cycleBranch, s.branch))

	exit, err = s.opts.run(s.ctx, "git", []string{"push", "origin", s.branch}, s.opts.Stdout, s.opts.Stderr)
	if err != nil || exit != 0 {
		head, _ := captureGitOutput(s.ctx, s.opts, "rev-parse", "HEAD")
		originalErr := shipErr(core.CodeGitPushRejected, core.ShipClassTransient, core.StageAtomicShip,
			fmt.Sprintf("ship: git push failed (rc=%d); main is at %s: %v", exit, strings.TrimSpace(head), err),
			"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(err), "branch", s.branch, "head", strings.TrimSpace(head))
		if repairErr := repairPushRace(s.ctx, s.opts, s.result, s.branch, originalErr); repairErr != nil {
			return repairErr
		}
	}
	s.result.Logs = append(s.result.Logs, fmt.Sprintf("[ship] OK: pushed to origin/%s", s.branch))

	headSHA, _ := captureGitOutput(s.ctx, s.opts, "rev-parse", "HEAD")
	s.result.CommitSHA = strings.TrimSpace(headSHA)
	committedTree, err := s.verifyCommittedTree()
	if err != nil {
		return err
	}
	if err := writeShipBinding(s.opts, committedTree, headSHA); err != nil {
		s.result.Logs = append(s.result.Logs, "[ship] WARN: could not write ship-binding.json: "+err.Error())
	}
	return maybeCreateRelease(s.ctx, s.opts, s.result)
}
