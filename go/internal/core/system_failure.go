package core

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// detectVerdictIncoherence is the ADR-0072 Go floor for the verdict-incoherence
// category: the deterministic, non-negotiable check that catches a pipeline
// forging a verdict. It reads the cycle's own on-disk phase artifacts
// (audit-report evolve-verdict + acs-verdict.json) and, if the recorded verdict
// is FAIL/WARN while both artifacts are green, returns a system-failure signal.
//
// This floor fires regardless of orchestrator judgment or strict_audit: a
// broken pipeline cannot be talked out of halting. It is gated on the
// failure_policy IsFloor(verdict-incoherence) predicate, so an operator can
// never narrow it below the compiled floor.
//
// Scope (deliberate for the deterministic slice): it fires ONLY on a recorded
// FAIL/WARN with green artifacts — the exact cycle 862→899 forgery signature.
// A RED artifact is coherent (a genuine task-code failure → never-stop). The
// "silent no-ship" (CycleOutcomeSkippedUnknown) case is intentionally NOT
// hard-halted here — a benign no-op cycle can also produce it, so its
// disambiguation is left to the orchestrator's judgment layer. The other floor
// category, infra-systemic (all CLI families exhausted), is enforced by the
// pre-existing resumable quota-pause path (cmd_loop) — NOT this function; the
// two floor categories have distinct, deliberate detection sites.
func (o *Orchestrator) detectVerdictIncoherence(cs CycleState, finalVerdict string) *SystemFailureSignal {
	audit, acs, auditRan := coherence.ReadCycleVerdicts(cs.WorkspacePath)
	coh := coherence.CheckVerdictCoherence(coherence.VerdictInputs{
		Recorded: finalVerdict,
		Audit:    audit,
		ACS:      acs,
		AuditRan: auditRan,
	})
	if !coh.Incoherent || !o.failurePolicy.IsFloor(coh.Category) {
		return nil
	}
	return &SystemFailureSignal{
		Category: coh.Category,
		Level:    policy.LevelSystem,
		Evidence: coh.Evidence,
		Halt:     true,
	}
}
