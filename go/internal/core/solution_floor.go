package core

import (
	"context"
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/solutioncheck"
)

// solution_floor.go — ADR-0099 slice 2: the document deliverable's deterministic
// handoff floor. A `document` cycle delivers <root>/<slug>/ (candidate options +
// recommendation + assumptions-and-evidence); the ONE engine that judges its
// shape (internal/solutioncheck) is projected here as a BuildFloorCheckFn so the
// E2 correction ladder fixes a malformed deliverable in-phase, exactly as the Go
// floors do for code. The kind and the bound slugs come from the kernel's own
// reads — the triage-authoritative report header and the triage decision —
// never from the builder's report. SolutionViolations is the single
// classification + collection every projection (floor, audit gate) calls.

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
// cycle: nil for a code cycle; otherwise one line per contract violation. The
// tree is the worktree when the cycle has one, else the project root (degraded
// provisioning) — the ONE tree-resolution rule for the floor and the audit
// gate. A document cycle that binds NO task is itself a violation: the
// contract cannot be verified for any deliverable, and "nothing to check" must
// never read as "checked, clean" (the topngate class).
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

// DocumentCycle reports whether the cycle's authoritative deliverable kind (the
// kernel's own digest of the scout/triage report headers) is document — the
// single classification predicate the floor, the audit gate, the task contract
// and ship share.
func DocumentCycle(workspace string) bool {
	if workspace == "" {
		return false
	}
	sig, _ := router.Digest(workspace, []string{"scout", "triage"})
	return sig.DeliverableKind() == config.DeliverableKindDocument
}

// seedDispatchContext is the ONE place both dispatch surfaces (the live loop
// and resume) enrich a phase's context from the cycle's records: the Task
// Contract (ADR-0098) and the project's default deliverable kind (ADR-0099).
func (o *Orchestrator) seedDispatchContext(ctx context.Context, base map[string]string, next Phase, cs CycleState, projectRoot string) map[string]string {
	out := o.seedTaskContract(ctx, base, next, cs, projectRoot)
	return seedDomainDefault(out, next, projectRoot)
}

// seedDomainDefault adds the project's default deliverable kind to the scout
// and triage dispatch context when .evolve/domain.json declares one — the
// first Go reader of that file. Other phases and projects without the file are
// untouched; a file that exists but cannot be parsed is reported loudly and
// leaves the default absent (the code side), never a silent reclassification.
func seedDomainDefault(base map[string]string, next Phase, projectRoot string) map[string]string {
	if next != PhaseScout && next != PhaseTriage {
		return base
	}
	d, ok, err := config.LoadDomain(projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN .evolve/domain.json unreadable — no default deliverable kind seeded: %v\n", err)
		return base
	}
	if !ok {
		return base
	}
	out := make(map[string]string, len(base)+1)
	for k, v := range base {
		out[k] = v
	}
	out[CtxKeyDeliverableKindDefault] = d.DefaultDeliverableKind()
	return out
}
