package core

import (
	"context"
	"strings"
	"testing"
)

// ctxProbe records the Context each dispatched phase received.
type ctxProbe struct {
	phase string
	ctxs  []map[string]string
}

func (p *ctxProbe) Name() string { return p.phase }
func (p *ctxProbe) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	cp := make(map[string]string, len(req.Context))
	for k, v := range req.Context {
		cp[k] = v
	}
	p.ctxs = append(p.ctxs, cp)
	return PhaseResponse{Phase: p.phase, Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func runScopedCycle(t *testing.T, resolver func(projectRoot, taskID string) string, env map[string]string) *ctxProbe {
	t.Helper()
	probe := &ctxProbe{phase: string(PhaseBuild)}
	runners := buildRunners(nil)
	runners[PhaseBuild] = probe
	opts := []Option{}
	if resolver != nil {
		opts = append(opts, WithScopePathResolver(resolver))
	}
	o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 0}}, &fakeLedger{}, runners, opts...)
	_, _ = o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true, Env: env,
	})
	return probe
}

func TestScopePaths_ResolvedPathsReachThePhaseContext(t *testing.T) {
	resolver := func(_, taskID string) string {
		if taskID == "pipeline-defect-pipeline-blocker" {
			return "/abs/inbox/2026-08-22T15-02-52Z-pipeline-defect-pipeline-blocker.json"
		}
		return "" // not inbox-backed (carryover ids) — no entry, fail-open
	}
	p := runScopedCycle(t, resolver, map[string]string{
		"EVOLVE_FLEET_SCOPE": "pipeline-defect-pipeline-blocker,some-carryover-id",
	})
	if len(p.ctxs) == 0 {
		t.Fatalf("no dispatches captured")
	}
	got := p.ctxs[0]["fleet_scope_paths"]
	if !strings.Contains(got, "pipeline-defect-pipeline-blocker=/abs/inbox/2026-08-22T15-02-52Z-pipeline-defect-pipeline-blocker.json") {
		t.Fatalf("the live path must reach the phase Context; got %q", got)
	}
	if strings.Contains(got, "some-carryover-id") {
		t.Fatalf("an id with no live record gets NO entry (fail-open); got %q", got)
	}
}

func TestScopePaths_NilResolverAddsNothing(t *testing.T) {
	p := runScopedCycle(t, nil, map[string]string{"EVOLVE_FLEET_SCOPE": "some-id"})
	if len(p.ctxs) == 0 {
		t.Fatalf("no dispatches captured")
	}
	if v, ok := p.ctxs[0]["fleet_scope_paths"]; ok {
		t.Fatalf("nil resolver must add no key; got %q", v)
	}
}

func TestScopePaths_UnscopedCycleAddsNothing(t *testing.T) {
	p := runScopedCycle(t, func(_, _ string) string { return "/never" }, nil)
	if len(p.ctxs) == 0 {
		t.Fatalf("no dispatches captured")
	}
	if v, ok := p.ctxs[0]["fleet_scope_paths"]; ok {
		t.Fatalf("no scope means no paths key; got %q", v)
	}
}
