package core

// gate_test.go — PA-DDK DDK-4 (ADR-0060): the artifact-floor THRESHOLDS are
// config-driven. The evaluator's verdict requirement comes from the loaded
// registry gate (config), evaluated against the trusted Go signal digest. Phases
// are resolved through the kerneltest fixture — no hardcoded names.

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/kerneltest"
	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

func TestGateSatisfied_ConfigThresholds(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	sm := NewStateMachine().WithCatalog(specForCatalog(ref.Catalog))
	eval := phaseFromRouter(ref.Evaluator()) // the verdict-gated anchor (audit-class)

	present := func(verdict string) router.RoutingSignals {
		return router.RoutingSignals{Audit: router.AuditSignals{Present: true, Verdict: verdict}}
	}
	if !sm.gateSatisfied(eval, present(VerdictPASS)) {
		t.Error("evaluator gate must pass on a present PASS handoff")
	}
	if !sm.gateSatisfied(eval, present(VerdictWARN)) {
		t.Error("evaluator gate must pass on a present WARN handoff (soft-pass floor)")
	}
	if sm.gateSatisfied(eval, present(VerdictFAIL)) {
		t.Error("evaluator gate must FAIL on a FAIL verdict — the config verdict_in excludes it")
	}
	if sm.gateSatisfied(eval, router.RoutingSignals{}) {
		t.Error("evaluator gate must fail when the handoff is absent (anti-fabrication)")
	}
}

// TestGateSatisfied_DegradesToLiteral: with no catalog (bare SM) the gate falls
// back to the literal artifact map — byte-identical to pre-DDK-4.
func TestGateSatisfied_DegradesToLiteral(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine() // no catalog → literal fallback
	if !sm.gateSatisfied(PhaseAudit, router.RoutingSignals{Audit: router.AuditSignals{Present: true, Verdict: VerdictPASS}}) {
		t.Error("literal fallback: audit PASS present must satisfy")
	}
	if sm.gateSatisfied(PhaseAudit, router.RoutingSignals{Audit: router.AuditSignals{Present: true, Verdict: VerdictFAIL}}) {
		t.Error("literal fallback: audit FAIL must not satisfy")
	}
}
