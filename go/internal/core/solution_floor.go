package core

import (
	"context"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/solutioncheck"
)

const CtxKeyDeliverableKindDefault = "deliverable_kind_default"

func SolutionFloorChecks(spec config.DeliverableKindSpec) BuildFloorCheckFn {
	return func(_ context.Context, in ReviewInput) []string {
		if in.Phase != string(PhaseBuild) {
			return nil
		}
		return SolutionViolations(in.Workspace, in.Worktree, in.ProjectRoot, spec)
	}
}

func SolutionViolations(workspace, worktree, projectRoot string, spec config.DeliverableKindSpec) []string {
	if !DocumentCycle(workspace) {
		return nil
	}
	tree := worktree
	if tree == "" {
		tree = projectRoot
	}
	ids := BoundTaskIDs(workspace)
	if len(ids) == 0 {
		return []string{"document cycle binds no task (triage-decision.json top_n is empty or unreadable) — the solution contract cannot be verified for any deliverable"}
	}
	var out []string
	for _, slug := range ids {
		for _, f := range solutioncheck.Check(tree, slug, spec) {
			out = append(out, f.String())
		}
	}
	return out
}

func DocumentCycle(workspace string) bool {
	if workspace == "" {
		return false
	}
	return kindSignals(workspace, os.Stderr).DeliverableKind() == config.DeliverableKindDocument
}

func (o *Orchestrator) seedDispatchContext(ctx context.Context, base map[string]string, next Phase, cs CycleState, projectRoot string) map[string]string {
	out := o.seedTaskContract(ctx, base, next, cs, projectRoot)
	out = seedDomainDefault(out, next, projectRoot)
	if spec, ok := o.cfg.DocumentSpec(); ok {
		out = seedDeliverableRoot(out, next, spec.Root)
	}
	return out
}

const CtxKeyDeliverableRoot = "deliverable_root"

func seedDeliverableRoot(base map[string]string, next Phase, root string) map[string]string {
	if root == "" || !kindDeclaringPhase(next) {
		return base
	}
	out := make(map[string]string, len(base)+1)
	for k, v := range base {
		out[k] = v
	}
	out[CtxKeyDeliverableRoot] = root
	return out
}

func seedDomainDefault(base map[string]string, next Phase, projectRoot string) map[string]string {
	if !kindDeclaringPhase(next) {
		return base
	}
	kind, ok := domainDefaultKind(projectRoot)
	if !ok {
		return base
	}
	out := make(map[string]string, len(base)+1)
	for k, v := range base {
		out[k] = v
	}
	out[CtxKeyDeliverableKindDefault] = kind
	return out
}
