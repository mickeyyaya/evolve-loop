package core

// safety_invariants_test.go — PA-DDK DDK-1/DDK-3 (ADR-0060). The validator is the
// relocated trust anchor. These tests LOAD the real phase configuration via the
// kerneltest fixture and reference phases through STRUCTURAL accessors
// (FirstAnchor/ShipTerminal/Evaluator), never by hardcoded name — so renaming a
// phase in the registry does not require rewriting any test. Adversarial cases
// are built by mutating the loaded config with fixture-derived names.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/kerneltest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// TestValidateSafetyInvariants_ReferenceFlowPasses: the real registry the
// composition root loads must satisfy the floor invariants. A future registry
// edit that breaks a branch target, strands an anchor, or adds an illegal spine
// edge fails here in CI before it can ship.
func TestValidateSafetyInvariants_ReferenceFlowPasses(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	if v := ValidateSafetyInvariants(NewStateMachine(), ref.Config, ref.Catalog); len(v) != 0 {
		t.Errorf("the loaded reference flow must pass the safety invariants; got: %v", v)
	}
}

// TestValidateSafetyInvariants_IllegalBranchTarget: a verdict-branch target that
// is NOT a legal successor is rejected — config may only select among legal
// edges. The evaluator routed back to the first anchor (e.g. audit→scout) is
// illegal; both phases come from the loaded config, not literals.
func TestValidateSafetyInvariants_IllegalBranchTarget(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cat := mustCatalog(t, phasespec.PhaseSpec{Name: ref.Evaluator(), OnPass: ref.FirstAnchor(), OnFail: ref.Evaluator()})
	if !containsSubstr(ValidateSafetyInvariants(NewStateMachine(), ref.Config, cat), "legal successor") {
		t.Error("an illegal verdict-branch target must be rejected")
	}
}

// TestValidateSafetyInvariants_UnknownBranchTarget: a target resolving to no
// known phase is rejected. The sentinel is an intentionally-invalid name, not a
// real phase, so renaming real phases never affects it.
func TestValidateSafetyInvariants_UnknownBranchTarget(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cat := mustCatalog(t, phasespec.PhaseSpec{Name: ref.Evaluator(), OnPass: ref.ShipTerminal(), OnFail: "__nonexistent_phase__"})
	if !containsSubstr(ValidateSafetyInvariants(NewStateMachine(), ref.Config, cat), "no known phase") {
		t.Error("an unresolvable verdict-branch target must be rejected")
	}
}

// TestValidateSafetyInvariants_StrandedMandatoryAnchor: a mandatory anchor
// unreachable from start cannot gate the floor and is rejected. PhaseSwarmPlan
// is a structural kernel phase (not an operator-renamed flow phase) that is
// valid but off any start→ship path — the rename-stable unreachable sentinel.
func TestValidateSafetyInvariants_StrandedMandatoryAnchor(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cfg := ref.Config
	// Mark the unreachable sentinel mandatory AND order-present so it becomes an
	// anchor (mandatoryAnchorsFor intersects Order ∩ Mandatory).
	cfg.Order = append([]string{string(PhaseSwarmPlan)}, cfg.Order...)
	cfg.Mandatory = append([]string{string(PhaseSwarmPlan)}, cfg.Mandatory...)
	if !containsSubstr(ValidateSafetyInvariants(NewStateMachine(), cfg, ref.Catalog), "unreachable") {
		t.Error("a stranded mandatory anchor must be rejected")
	}
}

// TestValidateSafetyInvariants_IllegalSpineEdge (DDK-3): a config-declared spine
// that jumps from the first anchor straight to the ship terminal — bypassing the
// floor — is rejected, since Next walks the spine without re-checking legality.
func TestValidateSafetyInvariants_IllegalSpineEdge(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cfg := ref.Config
	cfg.SpineOrder = []string{ref.FirstAnchor(), ref.ShipTerminal()}
	if !containsSubstr(ValidateSafetyInvariants(NewStateMachine(), cfg, ref.Catalog), "not a legal transition") {
		t.Error("an illegal spine edge must be rejected")
	}
}

// TestValidateSafetyInvariants_UnknownSpinePhase (DDK-3): a typo in spine_order
// (a name resolving to no phase) is rejected, not silently dropped.
func TestValidateSafetyInvariants_UnknownSpinePhase(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cfg := ref.Config
	cfg.SpineOrder = append([]string{"__nonexistent_phase__"}, ref.Spine()...)
	if !containsSubstr(ValidateSafetyInvariants(NewStateMachine(), cfg, ref.Catalog), "no known phase") {
		t.Error("an unknown spine_order phase must be rejected")
	}
}

// TestValidateSafetyInvariants_FloorGateAcceptsOnlyShippableVerdict (DDK-4): a
// mandatory phase whose artifact gate accepts a non-shippable verdict (FAIL)
// would let the floor ship a failed evaluation — the validator rejects it.
func TestValidateSafetyInvariants_FloorGateAcceptsOnlyShippableVerdict(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cat := mustCatalog(t, phasespec.PhaseSpec{
		Name: ref.Evaluator(),
		Gate: &phasespec.ArtifactGate{RequiresPresent: true, VerdictIn: []string{VerdictFAIL}},
	})
	if !containsSubstr(ValidateSafetyInvariants(NewStateMachine(), ref.Config, cat), "shippable verdict") {
		t.Error("a floor gate accepting a non-shippable verdict must be rejected")
	}
}

// TestValidateSafetyInvariants_NoMandatoryEvaluatorRejected (DDK-5 hardening,
// ADR-0060 §4 F⊆M): dropping the evaluator from mandatory_phases means no
// mandatory phase gates the verdict, so SpineSatisfiedUpTo would admit ship with
// no audit (the runtime anchor goes inert). The validator must reject it at load.
// The evaluator name comes from the loaded config, not a literal.
func TestValidateSafetyInvariants_NoMandatoryEvaluatorRejected(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cfg := ref.Config
	cfg.Mandatory = without(cfg.Mandatory, ref.Evaluator())
	if !containsSubstr(ValidateSafetyInvariants(NewStateMachine(), cfg, ref.Catalog), "mandatory evaluator") {
		t.Error("dropping the evaluator from mandatory_phases must be rejected — the ship floor goes inert")
	}
}

// TestValidateSafetyInvariants_PresenceOnlyEvaluatorRejected (DDK-5 hardening): an
// evaluator declared with a presence-only gate (requires_present, no verdict_in)
// is not a real evaluator — a FAIL verdict slips through. With no other mandatory
// evaluator the floor has none, so the validator must reject it.
func TestValidateSafetyInvariants_PresenceOnlyEvaluatorRejected(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cat := mustCatalog(t, phasespec.PhaseSpec{Name: ref.Evaluator(), Gate: &phasespec.ArtifactGate{RequiresPresent: true}})
	if !containsSubstr(ValidateSafetyInvariants(NewStateMachine(), ref.Config, cat), "mandatory evaluator") {
		t.Error("a presence-only evaluator gate (no verdict_in) must not satisfy the floor-evaluator requirement")
	}
}

// TestValidateSafetyInvariants_UnknownLegalSuccessor (DDK-5 hardening): a typo in
// config.legal_successors (a name resolving to no phase) is silently dropped by
// legalGraphFrom — the validator must report it loudly, not degrade the graph.
func TestValidateSafetyInvariants_UnknownLegalSuccessor(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cfg := ref.Config
	cfg.LegalSuccessors = cloneSuccessors(ref.Config.LegalSuccessors)
	cfg.LegalSuccessors["__bogus_from__"] = []string{ref.ShipTerminal()}
	if !containsSubstr(ValidateSafetyInvariants(NewStateMachine(), cfg, ref.Catalog), "no known phase") {
		t.Error("an unresolvable legal_successors phase name must be rejected at load")
	}
}

func containsSubstr(ss []string, sub string) bool {
	for _, s := range ss {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
