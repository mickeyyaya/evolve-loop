package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func (o *Orchestrator) recoverFromShipError(ctx context.Context, projectRoot string, cycle int, cs *CycleState, se *ShipError, depth, fleetWidth int) (Phase, bool) {
	return o.keepShipWindowOnlyForShip(o.recoverShipErrorRoute(ctx, projectRoot, cycle, cs, se, depth, fleetWidth))
}

func (o *Orchestrator) recoverShipErrorRoute(ctx context.Context, projectRoot string, cycle int, cs *CycleState, se *ShipError, depth, fleetWidth int) (Phase, bool) {
	o.recordShipError(ctx, cycle, *cs, se)
	o.persistShipErrorWitness(projectRoot, *cs, se)
	budget := shipRecoveryBudget(se.Code, fleetWidth)
	if depth >= budget {
		fmt.Fprintf(o.errOut(), "[orchestrator] ship recovery exhausted after %d attempt(s) (%s/%s, budget %d, fleet width %d); aborting\n", depth, se.Code, se.Class, budget, fleetWidth)
		return "", false
	}
	if isContentionShipCode(se.Code) {
		pause := contentionBackoff(depth)
		fmt.Fprintf(o.errOut(), "[orchestrator] contention backoff %s before ship recovery attempt %d/%d (%s)\n", pause, depth+1, budget, se.Code)
		backoffSleep(pause)
	}
	recoverCode, recoverClass := se.Code, se.Class
	if se.Code == CodeGitFleetRebaseNeeded {
		predictedConflict := false
		if cs.ActiveWorktree != "" {
			switch verdict, perr := ClassifyFleetRebaseCandidate(ctx, cs.ActiveWorktree, "HEAD", "main"); {
			case perr != nil:
				fmt.Fprintf(o.errOut(), "[orchestrator] cycle %d fleet-rebase pre-screen error (%v); falling through to rebase\n", cycle, perr)
			case verdict == FleetRebaseAlreadyLanded:
				fmt.Fprintf(o.errOut(), "[orchestrator] cycle %d fleet-rebase candidate already landed on main (superseded); short-circuiting with no wasted replay/re-audit (948 duplicate-work fix)\n", cycle)
				return "", false
			case verdict == FleetRebaseConflict:
				predictedConflict = true
			case verdict == FleetRebaseClean:
			}
		}
		o.holdShipWindow(ctx, projectRoot, cycle, cs.RunID)
		unwind := o.unwindUnlessPredictedConflict(ctx, projectRoot, cycle, *cs, predictedConflict)
		ok, conflict := rebaseReport{o.signals, cycle, "Orchestrator.recoverFromShipError"}.run(ctx, projectRoot, cs)
		if unwind == unwindDone || (ok && unwind == unwindKeepsConsumption) {
			if err := pendRebasedChange(ctx, cs.ActiveWorktree, gitCapture); err != nil {
				fmt.Fprintf(o.errOut(), "[orchestrator] cycle %d pend the audited change failed: %v\n", cycle, err)
				return "", false
			}
		}
		switch {
		case ok:
			if cs.ExplanationDocumentationVersion > 0 {
				return o.routeRebasedExplanation(ctx, projectRoot, cycle, cs)
			}
			if o.scopedMergeCarryForward(ctx, cycle, *cs, projectRoot) {
				return PhaseShip, true
			}
			fmt.Fprintf(o.errOut(), "[orchestrator] cycle %d has no explanation contract, so the identity carry cannot prove the rebased change; auditing it again\n", cycle)
		case conflict:
			fmt.Fprintf(o.errOut(), "[orchestrator] cycle %d fleet rebase CONFLICT (overlapping work the partition should have separated) → debugger\n", cycle)
			o.emitFleetRebaseConflict(cycle, *cs)
			recoverCode, recoverClass = CodeGitFleetRebaseConflict, ShipClassIntegrity
		default:
			fmt.Fprintf(o.errOut(), "[orchestrator] cycle %d fleet rebase onto main failed (infra); aborting\n", cycle)
			return "", false
		}
	}
	return o.routeShipRecovery(se, recoverCode, recoverClass)
}

func recoveryReasonCode(code string, conflicts []string) string {
	if code == string(CodeGitFleetRebaseNeeded) && len(conflicts) > 0 {
		return string(CodeGitFleetRebaseConflict)
	}
	return code
}

func (o *Orchestrator) routeShipRecovery(se *ShipError, recoverCode ShipErrorCode, recoverClass ShipErrorClass) (Phase, bool) {
	dec := router.Recover(router.RouteInput{
		Blocker: &router.Blocker{
			Code:  string(recoverCode),
			Class: string(recoverClass),
			Stage: string(se.Stage),
		},
	})
	cand := o.candidatePhase(dec.NextPhase)
	if cand == "" || cand == PhaseEnd {
		fmt.Fprintf(o.errOut(), "[orchestrator] ship error %s (%s) is unrecoverable (%s); aborting\n", se.Code, se.Class, dec.Reason)
		return "", false
	}
	if !o.sm.CanTransition(PhaseShip, cand) {
		fmt.Fprintf(o.errOut(), "[orchestrator] ship recovery proposed illegal edge ship→%s (%s); aborting\n", cand, dec.Reason)
		return "", false
	}
	fmt.Fprintf(o.errOut(), "[orchestrator] ship error %s (%s) → recovery routes to %s (%s)\n", se.Code, se.Class, cand, dec.Reason)
	return cand, true
}

func (o *Orchestrator) routeRebasedExplanation(ctx context.Context, projectRoot string, cycle int, cs *CycleState) (Phase, bool) {
	newBase, err := forkPoint(ctx, gitCapture, cs.ActiveWorktree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d resolve rebased explanation base failed: %v\n", cycle, err)
		return "", false
	}
	base0 := cs.WorktreeBaseSHA
	rebasedState := *cs
	rebasedState.WorktreeBaseSHA = newBase
	persist := func() error { return o.storage.WriteCycleState(ctx, rebasedState) }
	binding := explanationBinding(projectRoot, *cs)
	rebound, skipped, err := rebindPendingChange(ctx, binding, newBase, persist)
	switch {
	case errors.Is(err, explanationdocs.ErrRebindIncomplete):
		fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d %v; aborting, resume recovers the split\n", cycle, err)
		return "", false
	case err != nil:
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d identity-preserving rebind not attempted: %v; returning to Build\n", cycle, err)
	case rebound:
		*cs = rebasedState
		if o.identityCarryForward(ctx, cycle, rebasedState, base0, projectRoot) {
			return PhaseShip, true
		}
		fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d rebase is byte-identical on %s: explanation rebound, re-auditing without a Build (ADR-0105)\n", cycle, newBase)
		return PhaseAudit, true
	case skipped != "":
		o.reportIdentityProofSkipped(cycle, *cs, skipped)
	default:
		fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d rebased change is not proven identical and pending on %s; Build re-authors the explanation\n", cycle, newBase)
	}
	if err := explanationdocs.RebaseBuildAndPersist(ctx, binding, newBase, persist); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d invalidate rebased Build explanation failed: %v\n", cycle, err)
		return "", false
	}
	*cs = rebasedState
	return PhaseBuild, true
}

func rebindPendingChange(ctx context.Context, binding explanationdocs.CycleBinding, newBase string, persist func() error) (rebound bool, skipped string, err error) {
	head, err := gitStdout(ctx, gitCapture, binding.Worktree, "rev-parse", "HEAD")
	if err != nil {
		return false, "", err
	}
	if head != newBase {
		return false, fmt.Sprintf("the rebased change is committed on %s, not pending on %s, and Audit reads git diff HEAD", head, newBase), nil
	}
	rebound, err = explanationdocs.RebindIdenticalRebase(ctx, binding, newBase, persist)
	return rebound, "", err
}

const CodeIdentityProofSkipped signalcenter.Code = "ORCHESTRATOR_IDENTITY_PROOF_SKIPPED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeIdentityProofSkipped, "the fleet-rebase recovery did not run the identity proof, so Build re-authors the explanation; fields.reason says why the proof could not run (for example, the rebased change is committed, not pending on the new base)")
}

func (o *Orchestrator) unwindUnlessPredictedConflict(ctx context.Context, projectRoot string, cycle int, cs CycleState, predictedConflict bool) unwindOutcome {
	if cs.ExplanationDocumentationVersion <= 0 || predictedConflict {
		return unwindNone
	}
	return o.unwindBeforeFleetRebase(ctx, projectRoot, cycle, cs)
}

func (o *Orchestrator) reportIdentityProofSkipped(cycle int, cs CycleState, reason string) {
	fmt.Fprintf(o.errOut(), "[orchestrator] WARN cycle %d identity proof skipped (%s): %s; Build re-authors the explanation\n", cycle, CodeIdentityProofSkipped, reason)
	o.signals.Emit(signalcenter.Event{
		Cycle: cycle, RunID: cs.RunID, Phase: string(PhaseShip), Module: signalcenter.ModuleOrchestrator,
		Origin: "Orchestrator.routeRebasedExplanation", Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn,
		Code: CodeIdentityProofSkipped, Reason: "identity proof skipped: " + reason, Fields: map[string]string{"reason": reason},
	})
}

type gitFn = func(ctx context.Context, dir string, args ...string) (string, int, error)

func gitStdout(ctx context.Context, git gitFn, dir string, args ...string) (string, error) {
	out, code, err := git(ctx, dir, args...)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("git %s: exit %d", strings.Join(args, " "), code)
	}
	return strings.TrimSpace(out), nil
}

type regenFn = func(ctx context.Context, worktree, entry string) error

type derivedClassifier = func(relPath string) (entry string, ok bool)

func derivedEntryOf(relPath string) (string, bool) {
	e, _, ok := derived.OutputOf(relPath)
	return e.Name, ok
}

func derivedConflictIn(worktree string) derivedClassifier {
	return func(relPath string) (string, bool) {
		merged, _ := os.ReadFile(filepath.Join(worktree, filepath.FromSlash(relPath)))
		if !derived.IsDerivedConflict(relPath, merged) {
			return "", false
		}
		return derivedEntryOf(relPath)
	}
}

func regenStaleProjections(ctx context.Context, worktree string, stale []derived.Entry, regen, stage regenFn) []string {
	var done []string
	for _, e := range stale {
		if err := regen(ctx, worktree, e.Name); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-derived-regen: %s: %v; docs gate remains the backstop\n", e.Name, err)
			continue
		}
		if err := stage(ctx, worktree, e.Name); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-derived-regen: stage %s: %v\n", e.Name, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "[orchestrator] build-derived-regen: refreshed %s from its inputs before audit\n", e.Name)
		done = append(done, e.Name)
	}
	return done
}

func stageDerivedEntry(ctx context.Context, worktree, entry string) error {
	return stageEntryWith(ctx, gitCapture, worktree, entry)
}

func stageEntryWith(ctx context.Context, git gitFn, worktree, entry string) error {
	e, ok := derived.Lookup(entry)
	if !ok {
		return fmt.Errorf("no derived entry %q", entry)
	}
	paths, err := gitStdout(ctx, git, worktree, append([]string{"ls-files", "-z", "--cached", "--others", "--exclude-standard", "--"}, e.Pathspecs()...)...)
	if err != nil || len(nulPaths(paths)) == 0 {
		return err
	}
	_, err = gitStdout(ctx, git, worktree, append([]string{"add", "-A", "--"}, nulPaths(paths)...)...)
	return err
}

func goInputsFor(ctx context.Context, worktree string) derived.GoInputDirs {
	dirs, err := derived.ResolveGoInputs(ctx, worktree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-derived-regen: %v; only the path inputs decide staleness\n", err)
	}
	return dirs
}

const maxRebaseContinueSteps = 100

func rebaseCycleBranchOntoMain(ctx context.Context, projectRoot, worktree string) (ok bool, conflicts []string) {
	ok, conflicts, _ = rebaseLane(ctx, projectRoot, laneRebase{worktree: worktree})
	return ok, conflicts
}

type laneRebase struct {
	worktree string
	own      []string
}

func rebaseLane(ctx context.Context, projectRoot string, lane laneRebase) (ok bool, conflicts, stamped []string) {
	worktree := lane.worktree
	if worktree == "" {
		return false, nil, nil
	}
	if inPlaceWorktree(worktree, projectRoot) {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN fleet rebase refused: the active worktree is the project root — a cycle never rebases the operator's tree\n")
		return false, nil, nil
	}
	if remote, ok := originMainRef(ctx, gitCapture, worktree); ok {
		if err := refuseUnprovenAhead(ctx, gitCapture, worktree, remote, strings.TrimPrefix(remote, "origin/")); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN fleet rebase refused: %v\n", err)
			return false, nil, nil
		}
	}
	fr := &fleetRebase{worktree: worktree, git: gitCapture, regen: regenerateDerivedArtifact, classify: derivedConflictIn(worktree), inbox: newInboxStamps(lane.own)}
	ok, conflicts = fr.run(ctx)
	if !ok {
		return false, conflicts, nil
	}
	return true, conflicts, fr.stamped
}

func rebaseWithDerivedRegen(ctx context.Context, worktree string, git gitFn, regen regenFn, classify derivedClassifier) (ok bool, conflicts []string) {
	return (&fleetRebase{worktree: worktree, git: git, regen: regen, classify: classify}).run(ctx)
}

type fleetRebase struct {
	worktree string
	git      gitFn
	regen    regenFn
	classify derivedClassifier
	inbox    inboxStamps
	stamped  []string
}

func (f *fleetRebase) abort() {
	cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _, _ = f.git(cctx, f.worktree, "rebase", "--abort")
	f.stamped = nil
}

func (f *fleetRebase) anchor(ctx context.Context) (string, error) {
	if len(f.inbox.own) == 0 {
		return "", nil
	}
	return gitStdout(ctx, f.git, f.worktree, "rev-parse", "HEAD")
}

func (f *fleetRebase) mergedTree(ctx context.Context, head string) (string, error) {
	out, code, err := f.git(ctx, f.worktree, "merge-tree", "--write-tree", "--no-messages", "main", head)
	if err != nil {
		return "", err
	}
	tree, _, _ := strings.Cut(out, "\n")
	if tree = strings.TrimSpace(tree); code > 1 || tree == "" {
		return "", fmt.Errorf("git merge-tree main %s: exit %d", head, code)
	}
	return tree, nil
}

func derivedExcludes() []string {
	var specs []string
	for _, e := range derived.Catalog() {
		for _, spec := range e.Pathspecs() {
			specs = append(specs, ":(exclude)"+spec)
		}
	}
	return specs
}

func (f *fleetRebase) auditedDrift(ctx context.Context, head string) ([]string, error) {
	tree, err := f.mergedTree(ctx, head)
	if err != nil {
		return nil, err
	}
	args := append([]string{"diff", "--name-only", "-z", tree, "HEAD", "--", ".", ":(exclude)" + inboxRoot}, derivedExcludes()...)
	out, err := gitStdout(ctx, f.git, f.worktree, args...)
	return nulPaths(out), err
}

func (f *fleetRebase) restore(ctx context.Context, head string) {
	if _, err := gitStdout(ctx, f.git, f.worktree, "reset", "--hard", head); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s: restore of the audited commit %s failed: %v\n", f.worktree, head, err)
	}
	f.stamped = nil
}

func (f *fleetRebase) run(ctx context.Context) (ok bool, conflicts []string) {
	head, err := f.anchor(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s: cannot read the audited commit (%v); not rebasing\n", f.worktree, err)
		return false, nil
	}
	ok, conflicts = f.rebaseAll(ctx)
	if !ok || len(f.stamped) == 0 {
		return ok, conflicts
	}
	drift, err := f.auditedDrift(ctx, head)
	if err == nil && len(drift) == 0 {
		return ok, conflicts
	}
	return false, f.refuse(ctx, head, drift, err)
}

func (f *fleetRebase) refuse(ctx context.Context, head string, drift []string, err error) []string {
	stamped := f.stamped
	fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s: the rebased change does not hold the audited code after the inbox resolve (drift %v, proof error %v); restoring %s\n", f.worktree, drift, err, head)
	f.restore(ctx, head)
	if err != nil {
		return nil
	}
	return append(drift, stamped...)
}

func (f *fleetRebase) rebaseAll(ctx context.Context) (ok bool, conflicts []string) {
	if _, exit, err := f.git(ctx, f.worktree, "rebase", "main"); err == nil && exit == 0 {
		return true, nil
	}
	if len(unmergedRebasePaths(ctx, f.worktree, f.git)) == 0 {
		f.abort()
		fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s onto main failed without conflicts (infra); aborted\n", f.worktree)
		return false, nil
	}
	for step := 0; step < maxRebaseContinueSteps; step++ {
		done, ok, conflicts := f.replay(ctx)
		if done {
			return ok, conflicts
		}
	}
	f.abort()
	fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s exceeded %d replay steps; aborted\n", f.worktree, maxRebaseContinueSteps)
	return false, nil
}

func (f *fleetRebase) replay(ctx context.Context) (done, ok bool, conflicts []string) {
	unmerged := unmergedRebasePaths(ctx, f.worktree, f.git)
	if len(unmerged) == 0 {
		return f.skipEmpty(ctx)
	}
	unmerged, settled, err := f.inbox.resolve(ctx, f.worktree, f.git, unmerged)
	if err != nil {
		f.abort()
		fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s: a git step failed while resolving the inbox (%v); aborted\n", f.worktree, err)
		return true, false, nil
	}
	for _, p := range settled {
		if !slices.Contains(f.stamped, p) {
			f.stamped = append(f.stamped, p)
		}
	}
	entries, genuine := groupDerivedConflicts(unmerged, f.classify)
	if len(genuine) > 0 {
		f.abort()
		fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s: non-derived conflict on %s (overlapping work) → debugger\n", f.worktree, strings.Join(genuine, ", "))
		return true, false, genuine
	}
	if !(derivedRebase{worktree: f.worktree, git: f.git, regen: f.regen}).regenerate(ctx, entries) {
		f.abort()
		return true, false, nil
	}
	if _, c, e := f.git(ctx, f.worktree, "-c", "core.editor=true", "rebase", "--continue"); e == nil && c == 0 {
		return true, true, nil
	}
	return false, false, nil
}

func (f *fleetRebase) skipEmpty(ctx context.Context) (done, ok bool, conflicts []string) {
	if _, code, err := f.git(ctx, f.worktree, "diff", "--cached", "--quiet"); err != nil || code != 0 {
		f.abort()
		fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s: the replay holds a staged change that did not commit; aborted so the audited commit is kept\n", f.worktree)
		return true, false, nil
	}
	if _, c, e := f.git(ctx, f.worktree, "rebase", "--skip"); e == nil && c == 0 {
		return true, true, nil
	}
	return false, false, nil
}

type derivedRebase struct {
	worktree string
	git      gitFn
	regen    regenFn
}

func (r derivedRebase) regenerate(ctx context.Context, entries []string) bool {
	for _, entry := range entries {
		if err := r.regen(ctx, r.worktree, entry); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s: regenerate %s failed: %v; aborting\n", r.worktree, entry, err)
			return false
		}
		if err := stageEntryWith(ctx, r.git, r.worktree, entry); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s: stage %s failed: %v; aborting\n", r.worktree, entry, err)
			return false
		}
		fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s: regenerated derived entry %s from merged source\n", r.worktree, entry)
	}
	return true
}

func groupDerivedConflicts(paths []string, classify derivedClassifier) (entries, genuine []string) {
	for _, p := range paths {
		entry, ok := classify(p)
		switch {
		case !ok:
			genuine = append(genuine, p)
		case !slices.Contains(entries, entry):
			entries = append(entries, entry)
		}
	}
	return entries, genuine
}

func rebaseRecordingConflicts(ctx context.Context, projectRoot string, cs *CycleState) (ok, conflict bool) {
	ok, conflicts, stamped := rebaseLane(ctx, projectRoot, laneRebase{worktree: cs.ActiveWorktree, own: LaneScopeIDs(cs.WorkspacePath)})
	cs.ShipRecoveryConflicts = conflicts
	cs.ShipRecoveryStamped = stamped
	return ok, len(conflicts) > 0
}

func unmergedRebasePaths(ctx context.Context, worktree string, git gitFn) []string {
	out, _, _ := git(ctx, worktree, "diff", "--name-only", "--diff-filter=U", "-z")
	return nulPaths(out)
}

func nulPaths(out string) []string {
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

func regenerateDerivedArtifact(ctx context.Context, worktree, entry string) error {
	e, ok := derived.Lookup(entry)
	if !ok {
		return fmt.Errorf("no regenerator registered for %q", entry)
	}
	return DerivedWorktree(worktree).Regenerate(ctx, e)
}

func refreshDerivedEntry(ctx context.Context, worktree, entry string) error {
	e, ok := derived.Lookup(entry)
	if !ok {
		return fmt.Errorf("no regenerator registered for %q", entry)
	}
	return DerivedWorktree(worktree).Refresh(ctx, e)
}

func DerivedWorktree(worktree string) derived.Worktree {
	return derived.Worktree{
		Run: worktreeGenerator(worktree),
		Git: func(ctx context.Context, args ...string) (string, int, error) {
			return gitCapture(ctx, worktree, args...)
		},
		Base: "HEAD",
	}
}

func worktreeGenerator(worktree string) derived.Generator {
	return func(ctx context.Context, evolveArgs ...string) error {
		inv := WorktreeEvolveInvocation(worktree, evolveArgs...)
		cmd := sysexec.Command(ctx, "go", inv.Args...)
		cmd.Dir = inv.Dir
		cmd.Env = inv.Env
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
}
