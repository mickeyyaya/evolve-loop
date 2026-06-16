package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// readUpstreamBuildPlan returns the build phase's upstream build-plan.md body to
// serve via the typed PhaseRequest.BuildPlan envelope (ADR-0050 Phase 3.7). It is
// the dispatch-seam relocation of the ad-hoc os.ReadFile the build phase did
// inside ComposePrompt: same file, read once at the seam so the phase no longer
// reaches to disk for an upstream artifact.
//
// It returns non-empty ONLY at EVOLVE_PHASE_IO>=advisory, for the build phase,
// with the build-planner enabled and a readable file. Every other case returns
// "" so the build phase falls back to its original disk read — byte-identical
// dispatch at off/shadow (the campaign's master no-regression invariant). The
// read is best-effort: a missing/unreadable file yields "" (the phase's own
// fallback then also reads nothing), never an error.
func readUpstreamBuildPlan(stage config.Stage, phase Phase, env map[string]string, workspace string) string {
	if stage < config.StageAdvisory || phase != PhaseBuild || workspace == "" || env["EVOLVE_BUILD_PLANNER"] != "1" {
		return ""
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "build-plan.md")); err == nil {
		return string(data)
	}
	return ""
}

// assemblePhaseIO is the dispatch-seam phase-I/O hook (ADR-0050 Phase 3.4 shadow
// comparison + Phase 3.10 enforce input), invoked only at EVOLVE_PHASE_IO>=shadow.
// It owns the SINGLE router.Digest of the upstream and then:
//   - runs the shadow comparison (the typed-vs-legacy divergence tripwire); and
//   - at >=enforce, returns the authoritative typed PhaseInput the phase consumes
//     in place of the legacy Context map.
//
// Below enforce it returns the zero PhaseInput, so the dispatch field stays
// zero-valued — byte-identical to the pre-cutover loop. A digest failure is
// non-fatal: the shadow comparison is skipped (it needs the digest) and, at
// enforce, the input is still assembled from the phase context with an empty
// Upstream view so the envelope remains authoritative.
func (cr *cycleRun) assemblePhaseIO(phase Phase, phaseWorktree string, phaseCtx map[string]string) phaseio.PhaseInput {
	sig, err := router.Digest(cr.cs.WorkspacePath, cr.cs.CompletedPhases)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phaseio shadow digest failed for %s: %v\n", phase, err)
		if cr.o.cfg.PhaseIO >= config.StageEnforce {
			return cr.buildPhaseInput(phase, phaseWorktree, phaseCtx, phaseio.Handoffs{})
		}
		return phaseio.PhaseInput{}
	}
	cr.emitPhaseIOShadowWithSig(phase, phaseCtx, sig)
	if cr.o.cfg.PhaseIO >= config.StageEnforce {
		return cr.buildPhaseInput(phase, phaseWorktree, phaseCtx, router.HandoffsFromSignals(sig))
	}
	return phaseio.PhaseInput{}
}

// buildPhaseInput assembles the sealed typed PhaseInput from the cycle-run
// identity/roots, the per-phase Context (via the soak-proven assembleCycleInputs/
// assembleErrorContext helpers), and the already-projected Upstream view. The env
// map is deep-copied by NewPhaseInput so the sealed input cannot be mutated by a
// later loop iteration touching cr.envSnap.
func (cr *cycleRun) buildPhaseInput(phase Phase, phaseWorktree string, phaseCtx map[string]string, up phaseio.Handoffs) phaseio.PhaseInput {
	return phaseio.NewPhaseInput(phaseio.PhaseInputInit{
		Cycle:         cr.cycle,
		RunID:         cr.cs.RunID,
		GoalHash:      cr.req.GoalHash,
		ProjectRoot:   cr.req.ProjectRoot,
		Workspace:     cr.cs.WorkspacePath,
		Worktree:      phaseWorktree,
		Phase:         string(phase),
		PreviousPhase: string(cr.current),
		Env:           cr.envSnap,
		Upstream:      up,
		CycleInputs:   assembleCycleInputs(phaseCtx),
		Error:         assembleErrorContext(phaseCtx),
	})
}
