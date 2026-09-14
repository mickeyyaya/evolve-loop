package verdict

// settle.go — the bounded settle ladder (one function, called two ways) and
// the ONE translation from a dispatch to the contract roots.

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// settled is what the ladder returned: the last probe result, its error, the
// number of RE-probes it took (0-SettleRetries — "never flushed" vs
// "malformed" in one number), and the stale-leftover flags the gate stamps
// afterwards (stale: byte-identical to the pre-dispatch snapshot; refused: the
// gate turned an OK probe into a refusal).
type settled struct {
	res      deliverable.Result
	err      error
	attempts int
	stale    bool
	refused  bool
}

// settle waits (bounded) for a contracted deliverable to become well-formed,
// re-probing up to SettleRetries times with SettleInterval between attempts.
// It serves BOTH the reconcile-on-teardown path (cycles 824/825: a next-phase
// context-cancel laundered into ErrArtifactTimeout fires while the deliverable
// is still being written) AND the clean-exit artifact-read path
// (cycle-603/899/921: a cleanly-exited agent idles while its `Write` flush
// lands).
//
// It retries ONLY while the report is contracted-but-not-yet-well-formed
// (err == nil && !res.OK) — the state a late flush passes through (a missing
// file is CodeMissingArtifact, err == nil). An ERROR means "no contract for
// this phase" or an IO fault; neither resolves by waiting, so those return at
// once — uncontracted phases pay ZERO retries. Waiting can only UPGRADE toward
// the agent's real on-disk verdict; a never-settling deliverable still returns
// not-OK.
//
// CANCELLATION: the wait observes ctx, so a cancelled phase stops waiting
// instead of sleeping out the ladder. The first probe always runs (a
// deliverable already on disk is still caught); cancellation then stops both
// the sleeping and the re-probing, and the last result stands. The check
// straddles the sleep (before and after) rather than racing a timer against
// it: the sleep is an injected, uninterruptible seam, so post-cancel cost is
// bounded at ONE interval with no further probe. The teardown call site passes
// context.WithoutCancel (see reconcileTeardown): there a cancelled ctx is
// frequently the CAUSE of the teardown; on the clean-exit path the agent
// already exited 0, so cancellation genuinely means nothing more is coming.
func (e *Engine) settle(ctx context.Context, id Identity, phase string, roots phasecontract.Roots) settled {
	res, err := e.verify(id, phase, roots)
	s := settled{res: res, err: err}
	for attempt := 0; attempt < SettleRetries && s.err == nil && !s.res.OK; attempt++ {
		if ctx.Err() != nil {
			return s
		}
		e.sleep(SettleInterval)
		if ctx.Err() != nil {
			return s
		}
		s.res, s.err = e.verify(id, phase, roots)
		s.attempts++
	}
	return s
}

// rootsFor is the ONE translation from a dispatch to the contract roots the
// engine verifies against — shared by the clean-exit check and the teardown
// reconcile so they can never resolve different paths, or a different cycle,
// for the same phase. EvolveDir completes the roots (orchestrator-target
// deliverables, the declared-effects lifecycle state) AND locates the merged
// catalog for the catalog-aware default; Cycle names the processing/cycle-N/
// a declared effect is judged under (ADR-0100).
func rootsFor(d Dispatch) phasecontract.Roots {
	roots := phasecontract.Roots{
		Workspace: d.Workspace, Worktree: d.Worktree, DispatchedArtifact: d.ArtifactPath,
		ExplanationDocumentationVersion: d.ExplanationDocumentationVersion,
		Cycle:                           d.Cycle,
	}
	if d.ProjectRoot != "" {
		roots.EvolveDir = paths.EvolveDirOf(d.ProjectRoot)
	}
	return roots
}
