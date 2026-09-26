package verdict

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// settled is the ladder's last probe. attempts counts re-probes, not the first probe; staleGate
// stamps stale and refused afterwards (refused: it turned an OK probe into a refusal).
type settled struct {
	res      deliverable.Result
	err      error
	attempts int
	stale    bool
	refused  bool
}

// settle re-probes a not-OK deliverable at most SettleRetries times. A probe error (no contract,
// or an IO fault) returns at once, since waiting cannot fix it. ctx is checked before and after
// each sleep because the injected sleep cannot be interrupted.
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

// rootsFor is the one translation from a dispatch to contract roots, so the clean-exit and teardown
// probes can never resolve different paths or cycles for one phase.
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
