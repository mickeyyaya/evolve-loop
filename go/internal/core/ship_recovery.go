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
	o.recordShipError(ctx, cycle, *cs, se)
	budget := shipRecoveryBudget(se.Code, fleetWidth)
	if depth >= budget {
		fmt.Fprintf(os.Stderr, "[orchestrator] ship recovery exhausted after %d attempt(s) (%s/%s, budget %d, fleet width %d); aborting\n", depth, se.Code, se.Class, budget, fleetWidth)
		return "", false
	}
	if isContentionShipCode(se.Code) {
		pause := contentionBackoff(depth)
		fmt.Fprintf(os.Stderr, "[orchestrator] contention backoff %s before ship recovery attempt %d/%d (%s)\n", pause, depth+1, budget, se.Code)
		backoffSleep(pause)
	}
	recoverCode, recoverClass := se.Code, se.Class
	if se.Code == CodeGitFleetRebaseNeeded {
		predictedConflict := false
		if cs.ActiveWorktree != "" {
			switch verdict, perr := ClassifyFleetRebaseCandidate(ctx, cs.ActiveWorktree, "HEAD", "main"); {
			case perr != nil:
				fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d fleet-rebase pre-screen error (%v); falling through to rebase\n", cycle, perr)
			case verdict == FleetRebaseAlreadyLanded:
				fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d fleet-rebase candidate already landed on main (superseded); short-circuiting with no wasted replay/re-audit (948 duplicate-work fix)\n", cycle)
				return "", false
			case verdict == FleetRebaseConflict:
				predictedConflict = true
			case verdict == FleetRebaseClean:
			}
		}
		unwind := o.unwindUnlessPredictedConflict(ctx, projectRoot, cycle, *cs, predictedConflict)
		ok, conflict := rebaseRecordingConflicts(ctx, projectRoot, cs)
		if unwind == unwindDone || (ok && unwind == unwindKeepsConsumption) {
			if err := pendRebasedChange(ctx, cs.ActiveWorktree, gitCapture); err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d pend the audited change failed: %v\n", cycle, err)
				return "", false
			}
		}
		switch {
		case ok:
			if cs.ExplanationDocumentationVersion > 0 {
				return o.routeRebasedExplanation(ctx, projectRoot, cycle, cs)
			}
			if o.compositionCarryForward(ctx, cycle, *cs, projectRoot) {
				return PhaseShip, true
			}
			if o.scopedMergeCarryForward(ctx, cycle, *cs, projectRoot) {
				return PhaseShip, true
			}
		case conflict:
			fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d fleet rebase CONFLICT (overlapping work the partition should have separated) → debugger\n", cycle)
			recoverCode, recoverClass = CodeGitFleetRebaseConflict, ShipClassIntegrity
		default:
			fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d fleet rebase onto main failed (infra); aborting\n", cycle)
			return "", false
		}
	}
	dec := router.Recover(router.RouteInput{
		Blocker: &router.Blocker{
			Code:  string(recoverCode),
			Class: string(recoverClass),
			Stage: string(se.Stage),
		},
	})
	cand := o.candidatePhase(dec.NextPhase)
	if cand == "" || cand == PhaseEnd {
		fmt.Fprintf(os.Stderr, "[orchestrator] ship error %s (%s) is unrecoverable (%s); aborting\n", se.Code, se.Class, dec.Reason)
		return "", false
	}
	if !o.sm.CanTransition(PhaseShip, cand) {
		fmt.Fprintf(os.Stderr, "[orchestrator] ship recovery proposed illegal edge ship→%s (%s); aborting\n", cand, dec.Reason)
		return "", false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] ship error %s (%s) → recovery routes to %s (%s)\n", se.Code, se.Class, cand, dec.Reason)
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
	fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d identity proof skipped (%s): %s; Build re-authors the explanation\n", cycle, CodeIdentityProofSkipped, reason)
	o.signals.Emit(signalcenter.Event{
		Cycle: cycle, RunID: cs.RunID, Phase: string(PhaseShip), Module: signalcenter.ModuleOrchestrator,
		Origin: "Orchestrator.routeRebasedExplanation", Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn,
		Code: CodeIdentityProofSkipped, Reason: "identity proof skipped: " + reason, Fields: map[string]string{"reason": reason},
	})
}

// gitFn runs a git subcommand in dir and returns (stdout, exitCode, err); it
// matches gitCapture so production wiring passes gitCapture directly while tests
// inject a fake — the Humble Object seam for rebaseWithDerivedRegen.
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

// maxRebaseContinueSteps bounds the replay loop so a pathological rebase can never
// spin forever (each cycle branch has only a handful of commits).
const maxRebaseContinueSteps = 100

func rebaseCycleBranchOntoMain(ctx context.Context, projectRoot, worktree string) (ok bool, conflicts []string) {
	if worktree == "" {
		return false, nil
	}
	if inPlaceWorktree(worktree, projectRoot) {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN fleet rebase refused: the active worktree is the project root — a cycle never rebases the operator's tree\n")
		return false, nil
	}
	return rebaseWithDerivedRegen(ctx, worktree, gitCapture, regenerateDerivedArtifact, derivedConflictIn(worktree))
}

func rebaseWithDerivedRegen(ctx context.Context, worktree string, git gitFn, regen regenFn, classify derivedClassifier) (ok bool, conflicts []string) {
	abort := func() {
		cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _, _ = git(cctx, worktree, "rebase", "--abort")
	}
	if _, exit, err := git(ctx, worktree, "rebase", "main"); err == nil && exit == 0 {
		return true, nil
	}
	if len(unmergedRebasePaths(ctx, worktree, git)) == 0 {
		abort()
		fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s onto main failed without conflicts (infra); aborted\n", worktree)
		return false, nil
	}
	for step := 0; step < maxRebaseContinueSteps; step++ {
		unmerged := unmergedRebasePaths(ctx, worktree, git)
		if len(unmerged) == 0 {
			if _, c, e := git(ctx, worktree, "rebase", "--skip"); e == nil && c == 0 {
				return true, nil
			}
			continue
		}
		entries, genuine := groupDerivedConflicts(unmerged, classify)
		if len(genuine) > 0 {
			abort()
			fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s: non-derived conflict on %s (overlapping work) → debugger\n", worktree, strings.Join(genuine, ", "))
			return false, genuine
		}
		if !(derivedRebase{worktree: worktree, git: git, regen: regen}).regenerate(ctx, entries) {
			abort()
			return false, nil
		}
		if _, c, e := git(ctx, worktree, "-c", "core.editor=true", "rebase", "--continue"); e == nil && c == 0 {
			return true, nil
		}
	}
	abort()
	fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s exceeded %d replay steps; aborted\n", worktree, maxRebaseContinueSteps)
	return false, nil
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
	ok, conflicts := rebaseCycleBranchOntoMain(ctx, projectRoot, cs.ActiveWorktree)
	cs.ShipRecoveryConflicts = conflicts
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
	return derivedWorktree(worktree).Regenerate(ctx, e)
}

func refreshDerivedEntry(ctx context.Context, worktree, entry string) error {
	e, ok := derived.Lookup(entry)
	if !ok {
		return fmt.Errorf("no regenerator registered for %q", entry)
	}
	return derivedWorktree(worktree).Refresh(ctx, e)
}

func derivedWorktree(worktree string) derived.Worktree {
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
