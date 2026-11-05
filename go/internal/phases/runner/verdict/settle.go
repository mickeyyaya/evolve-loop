package verdict

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

type settled struct {
	res      deliverable.Result
	err      error
	attempts int
	stale    bool
	refused  bool
}

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

func rootsFor(d Dispatch) phasecontract.Roots {
	roots := deliverable.RootsFor(core.ReviewInput{
		Cycle: d.Cycle, Phase: d.Phase, Workspace: d.Workspace, Worktree: d.Worktree,
		ProjectRoot: d.ProjectRoot, ExplanationDocumentationVersion: d.ExplanationDocumentationVersion,
	})
	roots.DispatchedArtifact = d.ArtifactPath
	return roots
}
