package core

// Untagged (fast-tier) test fakes shared between fast logic tests and the
// integration-tagged real-git tests. Kept tag-free to satisfy the
// self-containment rule: an untagged fast test (orchestrator_spinegate_test.go)
// references insertedLeakRunner, so its definition cannot live in a
// //go:build integration file.

import "context"

// insertedLeakRunner is a no-git fake PhaseRunner whose onRun callback fires on
// each Run, always returning a PASS verdict. Used to script orchestrator
// sequencing without any real work.
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
