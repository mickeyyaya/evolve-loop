package ship

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
)

const (
	landingBinary  = "go/evolve"
	landingResumed = "landing-resumed"
)

func (s *worktreeShip) land(pending string) error {
	commit, err := s.landingCommit(pending)
	if err != nil {
		return err
	}
	s.catchUpMain()
	if err := s.opts.landing().CheckFastForward(s.ctx, s.integration(commit)); err != nil {
		if merr := s.moveLaneRef(pending); merr != nil {
			return merr
		}
		return err
	}
	in, err := s.prepareLanding(commit)
	if err != nil {
		return s.abandonPrepared(pending, in, err)
	}
	if err := s.moveLaneRef(pending); err != nil {
		return err
	}
	return s.completeLanding(in)
}

func (s *worktreeShip) landingCommit(pending string) (string, error) {
	if pending != "" {
		return pending, nil
	}
	commit, err := captureGitOutputAtDir(s.ctx, s.opts, s.worktree, "rev-parse", "HEAD")
	return strings.TrimSpace(commit), err
}

func (s *worktreeShip) catchUpMain() {
	origin, err := captureGitOutput(s.ctx, s.opts, "rev-parse", "origin/"+s.branch)
	if origin = strings.TrimSpace(origin); err != nil || origin == "" {
		return
	}
	behind, err := captureGitOutput(s.ctx, s.opts, "rev-list", s.branch+".."+origin)
	commits := strings.Fields(behind)
	if err != nil || len(commits) == 0 || !isAncestor(s.ctx, s.opts, s.branch, origin) {
		return
	}
	for _, sha := range commits {
		if !journalHasSHA(s.opts.ProjectRoot, sha) {
			s.logf("[ship] LANDING: %s is behind origin/%s, but the ship journal does not hold %s, so ship does not fast-forward %s", s.branch, s.branch, sha, s.branch)
			return
		}
	}
	s.logf("[ship] LANDING: %s is behind origin/%s by %d journaled commit(s); ship fast-forwards %s to %s before its own check", s.branch, s.branch, len(commits), s.branch, origin)
	s.opts.landing().Integrate(s.ctx, landing.Integration{Branch: s.branch, CycleBranch: "origin/" + s.branch, Commit: origin,
		Binary: landingBinary, Log: logTo(s.result)})
}

func (s *worktreeShip) prepareLanding(commit string) (landing.Intent, error) {
	in, err := s.newIntent(commit)
	if err == nil {
		err = appendShipJournal(s.opts.ProjectRoot, shipJournalEntry{SHA: commit, Class: string(s.opts.Class), Cycle: in.Cycle})
	}
	if err == nil {
		err = writeLandingIntent(s.opts, in)
	}
	if err != nil {
		return in, shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageAtomicShip,
			"ship: ship cannot record the landing intent, so it pushed nothing: "+err.Error(), "commit", commit)
	}
	s.logf("[ship]   OK: ship journaled %s and prepared its landing intent before the lane ref moved", commit)
	return in, nil
}

func (s *worktreeShip) newIntent(commit string) (landing.Intent, error) {
	cid, ok, err := cycleIDForShip(s.opts)
	if err != nil || !ok {
		return landing.Intent{}, errors.Join(errors.New("no readable cycle_id for the run workspace"), err)
	}
	runID, err := runIDForShip(s.opts)
	if err != nil {
		return landing.Intent{}, err
	}
	in := landing.Intent{Cycle: cid, RunID: runID, AuditArtifactSHA256: s.opts.internalAuditArtifactSHA,
		AuditedTree: s.opts.internalAuditBoundTreeSHA, WorktreeBaseSHA: s.opts.WorktreeBaseSHA, CommitSHA: commit,
		ConsumedPaths: append([]string(nil), s.opts.internalConsumedPaths...), Branch: s.branch, LaneBranch: s.cycleBranch,
		Status: landing.IntentPrepared}
	return in, s.measure(&in)
}

func (s *worktreeShip) measure(in *landing.Intent) error {
	tree, err := captureGitOutput(s.ctx, s.opts, "rev-parse", in.CommitSHA+"^{tree}")
	if err != nil {
		return err
	}
	preMain, err := captureGitOutput(s.ctx, s.opts, "rev-parse", s.branch)
	if err != nil {
		return err
	}
	in.CommitTree, in.PreMain = strings.TrimSpace(tree), strings.TrimSpace(preMain)
	if in.WorktreeBaseSHA == "" {
		in.WorktreeBaseSHA = in.PreMain
	}
	if in.LaneTree, err = laneTreeBeforeConsumption(s.ctx, s.opts, in.CommitSHA); err != nil {
		return err
	}
	in.ExplanationViewSHA256, err = explanationViewSHA(s.opts.ProjectRoot, in.Cycle, in.RunID)
	return err
}

func (s *worktreeShip) abandonPrepared(pending string, in landing.Intent, err error) error {
	if in.LaneTree == "" {
		return err
	}
	if merr := s.moveLaneRef(pending); merr != nil {
		return errors.Join(err, merr)
	}
	if uerr := s.unwindLane(in, err.Error()); uerr != nil {
		return uerr
	}
	return err
}

func (s *worktreeShip) moveLaneRef(commit string) error {
	if commit == "" {
		return nil
	}
	var stderr strings.Builder
	exit, err := s.opts.run(s.ctx, "git", []string{"-C", s.worktree, "update-ref", "-m", "evolve ship: the landing commit", "HEAD", commit, commit + "^"}, io.Discard, &stderr)
	if err != nil || exit != 0 {
		return shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageAtomicShip,
			fmt.Sprintf("ship: git update-ref of the lane to %s failed (rc=%d): %v %s; the lane ref did not move", commit, exit, err, strings.TrimSpace(stderr.String())),
			"commit", commit, "worktree", s.worktree)
	}
	return nil
}

func (s *worktreeShip) integration(commit string) landing.Integration {
	return landingIntegration(s.opts, s.result, landing.Intent{Branch: s.branch, LaneBranch: s.cycleBranch, CommitSHA: commit})
}

func landingIntegration(opts *Options, res *RunResult, in landing.Intent) landing.Integration {
	label := in.LaneBranch
	if label == "" {
		label = in.CommitSHA
	}
	return landing.Integration{Branch: in.Branch, CycleBranch: label, Commit: in.CommitSHA, Binary: landingBinary,
		Fleet: opts.envBool(ipcenv.FleetKey), Log: logTo(res)}
}

func (s *worktreeShip) completeLanding(in landing.Intent) error {
	if err := s.verifyLandingCommit(in); err != nil {
		return err
	}
	if err := pushLanding(s.ctx, s.opts, s.result, in); err != nil {
		return s.unwindUnlessResumable(in, err)
	}
	return settleLanding(s.ctx, s.opts, s.result, in)
}

func (s *worktreeShip) verifyLandingCommit(in landing.Intent) error {
	bound := s.opts.internalAuditBoundTreeSHA
	if err := adoptIntentConsumption(s.opts, in.ConsumedPaths); err != nil {
		return shipErr(core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, core.StageAtomicShip,
			"INTEGRITY BREACH (pre-push): ship refuses the consumed paths of the landing intent and pushed nothing: "+err.Error(),
			"commit", in.CommitSHA, "phase", "pre-push")
	}
	if bound == "" {
		return nil
	}
	if ok, detail := auditBindingSatisfied(s.ctx, s.opts, "", in.CommitTree); !ok {
		return shipErr(core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, core.StageAtomicShip,
			fmt.Sprintf("INTEGRITY BREACH (pre-push): audit-bound tree SHA %s != landing commit tree SHA %s — refused to push%s", bound, in.CommitTree, detail),
			"audit_bound_tree", bound, "commit_tree", in.CommitTree, "phase", "pre-push")
	}
	return nil
}

func pushLanding(ctx context.Context, opts *Options, res *RunResult, in landing.Intent) error {
	if err := pushWithRepair(ctx, opts, res, landing.PushRequest{Branch: in.Branch, Commit: in.CommitSHA, Site: landing.SiteWorktree}); err != nil {
		return err
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: pushed to origin/%s", in.Branch))
	return nil
}

func settleLanding(ctx context.Context, opts *Options, res *RunResult, in landing.Intent) error {
	opts.landing().Integrate(ctx, landingIntegration(opts, res, in))
	committedTree, verified, err := verifyCommittedTree(ctx, opts, in.CommitSHA)
	if err != nil {
		return err
	}
	if verified != "" {
		res.Logs = append(res.Logs, verified)
	}
	res.CommitSHA = in.CommitSHA
	if err := writeShipBinding(opts, committedTree, in.CommitSHA); err != nil {
		res.Logs = append(res.Logs, "[ship] WARN: could not write ship-binding.json: "+err.Error())
	}
	in.Status = landing.IntentComplete
	if err := writeLandingIntent(opts, in); err != nil {
		res.Logs = append(res.Logs, "[ship] WARN: ship cannot mark the landing intent complete (a retry settles it again and changes nothing): "+err.Error())
	}
	return maybeCreateRelease(ctx, opts, res)
}

func (s *worktreeShip) unwindUnlessResumable(in landing.Intent, err error) error {
	if se, ok := core.AsShipError(err); ok && se.Class == core.ShipClassTransient {
		s.logf("[ship] LANDING: the push of %s had a transient failure; main did not move, and a retry resumes at the push", in.CommitSHA)
		return err
	}
	if uerr := s.unwindLane(in, err.Error()); uerr != nil {
		return uerr
	}
	if merr := s.markIntent(in, landing.IntentUnwound); merr != nil {
		return errors.Join(err, merr)
	}
	return err
}

func (s *worktreeShip) unwindLane(in landing.Intent, why string) error {
	audited := core.AuditedChange{Base: in.WorktreeBaseSHA, Tree: in.LaneTree, Label: fmt.Sprintf("cycle-%d/%s", in.Cycle, in.RunID)}
	declined, err := core.UnwindToAuditedShape(s.ctx, s.worktree, audited)
	if err != nil {
		declined = err.Error()
	}
	if declined != "" {
		return shipErr(core.CodeGitLandingUnwindDeclined, core.ShipClassIntegrity, core.StageAtomicShip,
			fmt.Sprintf("ship: the prepared landing of %s cannot complete (%s), and ship cannot unwind the lane to the audited shape (%s). The commit stays on the lane and in the journal, and %s did not move. %s",
				in.CommitSHA, why, declined, in.Branch, boundarySteps(in.CommitSHA)),
			"commit", in.CommitSHA, "worktree", s.worktree, "unwind_declined", declined)
	}
	s.logf("[ship] LANDING: ship unwound the lane to its tree %s before ship's consumption, staged on %s (%s)", in.LaneTree, in.WorktreeBaseSHA, why)
	return nil
}

func boundarySteps(commit string) string {
	return fmt.Sprintf("To land it, do these steps at the boundary: 1. In the plane, run `git merge --ff-only %s`. 2. Run `evolve sync-main`. 3. Run `evolve ship --push-only`.", commit)
}

func (s *worktreeShip) markIntent(in landing.Intent, status landing.IntentStatus) error {
	in.Status = status
	if err := writeLandingIntent(s.opts, in); err != nil {
		return shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageAtomicShip,
			fmt.Sprintf("ship: ship cannot mark the landing intent %s: %v", status, err), "commit", in.CommitSHA)
	}
	return nil
}

func resumeLanding(ctx context.Context, opts *Options, res *RunResult) (bool, error) {
	if opts.Class != ClassCycle || opts.DryRun || opts.PushOnly {
		return false, nil
	}
	in, found, err := readLandingIntent(opts)
	if err != nil || !found || in.Status != landing.IntentPrepared {
		return false, err
	}
	worktree, fromWorktree, err := landingTree(opts)
	if err != nil || !fromWorktree {
		return false, err
	}
	if in.Branch, err = currentBranch(ctx, opts); err != nil || in.Branch == "" {
		return false, err
	}
	release, err := opts.acquireShipLock()
	if err != nil {
		return false, shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageAtomicShip,
			"ship: acquire integrator lock (.evolve/ship.lock): "+err.Error())
	}
	defer release()
	return newWorktreeShip(ctx, opts, res, in.Branch, worktree).resume(in)
}

func (s *worktreeShip) resume(in landing.Intent) (bool, error) {
	if err := s.rollForward(in); err != nil {
		return false, err
	}
	w, audit, why := s.witness(in)
	verdict := landing.Verdict{Declined: why}
	if why == "" {
		verdict = s.opts.landing().Resume(s.ctx, in, w)
	}
	if verdict.Declined != "" {
		return false, s.decline(in, w, verdict.Declined)
	}
	bindResumedAudit(s.opts, audit)
	if err := s.finishResume(in, verdict); err != nil {
		return true, err
	}
	s.result.RepairAttempted, s.result.RepairOutcome = string(core.CodeGitPushRejected), landingResumed
	return true, nil
}

func (s *worktreeShip) finishResume(in landing.Intent, verdict landing.Verdict) error {
	if !verdict.OnOrigin {
		s.logf("[ship] LANDING: ship resumes the prepared landing of %s at its push; no gate runs again", in.CommitSHA)
		return s.completeLanding(in)
	}
	if err := s.verifyLandingCommit(in); err != nil {
		return err
	}
	s.logf("[ship] LANDING: origin already holds the prepared commit %s, so ship settles the landing with no push and no gate", in.CommitSHA)
	return settleLanding(s.ctx, s.opts, s.result, in)
}

func (s *worktreeShip) rollForward(in landing.Intent) error {
	if !s.stoppedBeforeTheRefMoved(in) {
		return nil
	}
	if err := s.moveLaneRef(in.CommitSHA); err != nil {
		return err
	}
	s.logf("[ship] LANDING: a stop before the lane ref moved left %s prepared; ship moves the lane ref forward to it", in.CommitSHA)
	return nil
}

func (s *worktreeShip) stoppedBeforeTheRefMoved(in landing.Intent) bool {
	tip, terr := captureGitOutputAtDir(s.ctx, s.opts, s.worktree, "rev-parse", "HEAD")
	parent, perr := captureGitOutputAtDir(s.ctx, s.opts, s.worktree, "rev-parse", "--verify", "--quiet", in.CommitSHA+"^")
	staged, serr := captureGitOutputAtDir(s.ctx, s.opts, s.worktree, "write-tree")
	tip = strings.TrimSpace(tip)
	return terr == nil && perr == nil && serr == nil && tip != in.CommitSHA && tip == strings.TrimSpace(parent) &&
		strings.TrimSpace(staged) == in.CommitTree && journalHasSHA(s.opts.ProjectRoot, in.CommitSHA)
}

func (s *worktreeShip) witness(in landing.Intent) (landing.Witness, *auditEntry, string) {
	var w landing.Witness
	var err error
	if w.Cycle, _, err = cycleIDForShip(s.opts); err != nil {
		return w, nil, "the cycle identity is unreadable: " + err.Error()
	}
	if w.RunID, err = runIDForShip(s.opts); err != nil {
		return w, nil, "the run identity is unreadable: " + err.Error()
	}
	tip, err := captureGitOutputAtDir(s.ctx, s.opts, s.worktree, "rev-parse", "HEAD")
	if err != nil {
		return w, nil, "the lane tip is unreadable: " + err.Error()
	}
	w.LaneTip, w.Journaled = strings.TrimSpace(tip), journalHasSHA(s.opts.ProjectRoot, in.CommitSHA)
	audit, err := latestRunAudit(s.opts, w.RunID)
	if err != nil {
		return w, nil, "the newest audit is unreadable: " + err.Error()
	}
	w.AuditArtifactSHA256, w.AuditedTree, w.AuditPassed = audit.ArtifactSHA256, audit.WorktreeTreeSHA, auditPassed(s.opts, audit)
	if w.ExplanationViewSHA256, err = explanationViewSHA(s.opts.ProjectRoot, w.Cycle, w.RunID); err != nil {
		return w, nil, "the sealed Build explanation is unreadable: " + err.Error()
	}
	return w, audit, ""
}

func (s *worktreeShip) decline(in landing.Intent, w landing.Witness, why string) error {
	s.logf("[ship] LANDING: the prepared landing of %s does not resume: %s", in.CommitSHA, why)
	if w.LaneTip != in.CommitSHA || w.Cycle != in.Cycle || w.RunID != in.RunID {
		s.logf("[ship] LANDING: the lane does not hold the prepared commit of this run, so ship marks the intent stale and runs the gates")
		return s.markIntent(in, landing.IntentStale)
	}
	if err := s.unwindLane(in, why); err != nil {
		return err
	}
	return s.markIntent(in, landing.IntentUnwound)
}

func explanationViewSHA(projectRoot string, cycle int, runID string) (string, error) {
	view, err := explanationdocs.LoadSnapshot(explanationdocs.CycleBinding{ProjectRoot: projectRoot, Cycle: cycle, RunID: runID})
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(view)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func laneTreeBeforeConsumption(ctx context.Context, opts *Options, commit string) (string, error) {
	if len(opts.internalConsumedPaths) == 0 {
		tree, err := captureGitOutput(ctx, opts, "rev-parse", commit+"^{tree}")
		return strings.TrimSpace(tree), err
	}
	dir, err := os.MkdirTemp("", "evolve-lane-tree-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	index := scratchIndex{ctx: ctx, opts: opts, env: append(os.Environ(), "GIT_INDEX_FILE="+filepath.Join(dir, "index"))}
	if _, err := index.git("read-tree", commit); err != nil {
		return "", err
	}
	for _, path := range opts.internalConsumedPaths {
		if err := index.restore(commit+"^", path); err != nil {
			return "", err
		}
	}
	tree, err := index.git("write-tree")
	if tree = strings.TrimSpace(tree); err == nil && tree == "" {
		err = errors.New("git write-tree in a scratch index wrote no tree")
	}
	return tree, err
}

type scratchIndex struct {
	ctx  context.Context
	opts *Options
	env  []string
}

func (x scratchIndex) git(args ...string) (string, error) {
	var stdout, stderr strings.Builder
	exit, err := x.opts.runner()(x.ctx, "git", x.opts.ProjectRoot, args, x.env, nil, &stdout, &stderr)
	if err != nil || exit != 0 {
		return "", fmt.Errorf("git %v in a scratch index (rc=%d): %v %s", args, exit, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func (x scratchIndex) restore(parent, path string) error {
	entry, err := x.git("ls-tree", parent, "--", path)
	if err != nil {
		return err
	}
	fields := strings.Fields(entry)
	if len(fields) < 3 {
		_, err = x.git("update-index", "--force-remove", "--", path)
		return err
	}
	_, err = x.git("update-index", "--add", "--cacheinfo", fields[0]+","+fields[2]+","+path)
	return err
}

func intentPath(opts *Options, cycle int) string {
	return landing.IntentPath(filepath.Join(opts.ProjectRoot, ".evolve"), cycle)
}

func writeLandingIntent(opts *Options, in landing.Intent) error {
	return landing.WriteIntent(intentPath(opts, in.Cycle), in)
}

func readLandingIntent(opts *Options) (landing.Intent, bool, error) {
	cid, ok, err := cycleIDForShip(opts)
	if err != nil {
		return landing.Intent{}, false, shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageAtomicShip,
			"ship: read the landing intent: the cycle identity is unreadable: "+err.Error())
	}
	if !ok {
		return landing.Intent{}, false, nil
	}
	path := intentPath(opts, cid)
	in, found, err := landing.ReadIntent(path)
	if err != nil {
		return in, false, shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageAtomicShip,
			"ship: read the landing intent: "+err.Error(), "path", path)
	}
	return in, found, nil
}

func (s *worktreeShip) logf(format string, args ...any) {
	s.result.Logs = append(s.result.Logs, fmt.Sprintf(format, args...))
}
