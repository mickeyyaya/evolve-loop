package core

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/solutioncheck"
)

// CtxKeyDeliverableKindDefault carries the project's default deliverable kind
// (.evolve/domain.json) to the scout/triage prompts, so a task that declares
// no kind inherits the project's.
const CtxKeyDeliverableKindDefault = "deliverable_kind_default"

// SolutionFloorChecks returns the build-floor check for the document contract.
// It is silent for code cycles and for every phase but build.
func SolutionFloorChecks(spec config.DeliverableKindSpec) BuildFloorCheckFn {
	return func(_ context.Context, in ReviewInput) []string {
		if in.Phase != string(PhaseBuild) {
			return nil
		}
		return SolutionViolations(in.Workspace, in.Worktree, in.ProjectRoot, spec)
	}
}

// ChainBuildFloorChecks composes deterministic check engines in order; every
// engine runs and every failure reaches the correction ladder.
func ChainBuildFloorChecks(fns ...BuildFloorCheckFn) BuildFloorCheckFn {
	return func(ctx context.Context, in ReviewInput) []string {
		var out []string
		for _, fn := range fns {
			if fn != nil {
				out = append(out, fn(ctx, in)...)
			}
		}
		return out
	}
}

// SolutionViolations judges every bound task's deliverable for a document
// cycle: nil for a code cycle; otherwise one line per contract violation.
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

// DocumentCycle reports whether the cycle's authoritative deliverable kind is
// document.
func DocumentCycle(workspace string) bool {
	if workspace == "" {
		return false
	}
	return kindSignals(workspace).DeliverableKind() == config.DeliverableKindDocument
}

// seedDispatchContext is the one place both dispatch surfaces enrich a
// phase's context from the cycle's records.
func (o *Orchestrator) seedDispatchContext(ctx context.Context, base map[string]string, next Phase, cs CycleState, projectRoot string) map[string]string {
	out := o.seedTaskContract(ctx, base, next, cs, projectRoot)
	out = seedDomainDefault(out, next, projectRoot)
	if spec, ok := o.cfg.DocumentSpec(); ok {
		out = seedDeliverableRoot(out, next, spec.Root)
	}
	return out
}

// CtxKeyDeliverableRoot carries the registry's document deliverable root to
// the kind-declaring phases, whose prompts carry no Task Contract block.
const CtxKeyDeliverableRoot = "deliverable_root"

// seedDeliverableRoot adds the configured root to a kind-declaring phase's
// context; other phases read it from the rendered Task Contract instead.
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

// seedDomainDefault adds the project's default deliverable kind to the scout
// and triage dispatch context when .evolve/domain.json declares one.
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
