package core

import "context"

type insertedLeakRunner struct {
	name  string
	onRun func(req PhaseRequest)
}

func (r *insertedLeakRunner) Name() string { return r.name }

func (r *insertedLeakRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	if r.onRun != nil {
		r.onRun(req)
	}
	return PhaseResponse{Phase: r.name, Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}
