package core

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/kerneltest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestValidateSafetyInvariants_ReferenceFlowPasses(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	if v := ValidateSafetyInvariants(NewStateMachine(), ref.Config, ref.Catalog); len(v) != 0 {
		t.Errorf("the loaded reference flow must pass the safety invariants; got: %v", v)
	}
}

func TestValidateSafetyInvariants_IllegalBranchTarget(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cat := mustCatalog(t, phasespec.PhaseSpec{Name: ref.Evaluator(), OnPass: ref.FirstAnchor(), OnFail: ref.Evaluator()})
	if !containsSubstr(ValidateSafetyInvariants(NewStateMachine(), ref.Config, cat), "legal successor") {
		t.Error("an illegal verdict-branch target must be rejected")
	}
}

func TestValidateSafetyInvariants_UnknownBranchTarget(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cat := mustCatalog(t, phasespec.PhaseSpec{Name: ref.Evaluator(), OnPass: ref.ShipTerminal(), OnFail: "__nonexistent_phase__"})
	if !containsSubstr(ValidateSafetyInvariants(NewStateMachine(), ref.Config, cat), "no known phase") {
		t.Error("an unresolvable verdict-branch target must be rejected")
	}
}

// PhaseSwarmPlan is a structural kernel phase that is valid but off any
// start→ship path — the rename-stable unreachable sentinel.
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

func TestValidateSafetyInvariants_IllegalSpineEdge(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cfg := ref.Config
	cfg.SpineOrder = []string{ref.FirstAnchor(), ref.ShipTerminal()}
	if !containsSubstr(ValidateSafetyInvariants(NewStateMachine(), cfg, ref.Catalog), "not a legal transition") {
		t.Error("an illegal spine edge must be rejected")
	}
}

func TestValidateSafetyInvariants_UnknownSpinePhase(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cfg := ref.Config
	cfg.SpineOrder = append([]string{"__nonexistent_phase__"}, ref.Spine()...)
	if !containsSubstr(ValidateSafetyInvariants(NewStateMachine(), cfg, ref.Catalog), "no known phase") {
		t.Error("an unknown spine_order phase must be rejected")
	}
}

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

func TestValidateSafetyInvariants_NoMandatoryEvaluatorRejected(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cfg := ref.Config
	cfg.Mandatory = without(cfg.Mandatory, ref.Evaluator())
	if !containsSubstr(ValidateSafetyInvariants(NewStateMachine(), cfg, ref.Catalog), "mandatory evaluator") {
		t.Error("dropping the evaluator from mandatory_phases must be rejected — the ship floor goes inert")
	}
}

func TestValidateSafetyInvariants_PresenceOnlyEvaluatorRejected(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cat := mustCatalog(t, phasespec.PhaseSpec{Name: ref.Evaluator(), Gate: &phasespec.ArtifactGate{RequiresPresent: true}})
	if !containsSubstr(ValidateSafetyInvariants(NewStateMachine(), ref.Config, cat), "mandatory evaluator") {
		t.Error("a presence-only evaluator gate (no verdict_in) must not satisfy the floor-evaluator requirement")
	}
}

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

// Sibling to TestValidateSafetyInvariants_UnknownLegalSuccessor above, which
// exercises the key (from) side; this one exercises the successor (to) side.
func TestValidateSafetyInvariants_UnknownLegalSuccessorTarget(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cfg := ref.Config
	cfg.LegalSuccessors = cloneSuccessors(ref.Config.LegalSuccessors)
	cfg.LegalSuccessors[ref.FirstAnchor()] = []string{"__bogus_to__"}
	if !containsSubstr(ValidateSafetyInvariants(NewStateMachine(), cfg, ref.Catalog), "successor") {
		t.Error("an unresolvable legal_successors SUCCESSOR name must be rejected at load")
	}
}

// The synthetic 2-cycle is injected via WithLegalGraph; the Phase constants
// here are arbitrary bare-graph vocabulary, not flow identity.
func TestValidateSafetyInvariants_NoStartNodeRejected(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cyclic := NewStateMachine().WithLegalGraph(map[Phase]map[Phase]bool{
		PhaseBuild: {PhaseAudit: true},
		PhaseAudit: {PhaseBuild: true},
	})
	if !containsSubstr(ValidateSafetyInvariants(cyclic, ref.Config, ref.Catalog), "no start node") {
		t.Error("a transition graph with no source node must be rejected")
	}
}

// A legal_successors edge that lets a path reach the ship sink WITHOUT
// traversing the floor evaluator (here: the first anchor jumps straight to
// ship, bypassing the audit-class evaluator) is rejected at load. Evaluator
// and ship sink are identified by ROLE (mandatory shippable-verdict phase /
// last mandatory anchor), never by name — a rename does not weaken the check.
func TestValidateSafetyInvariants_EvaluatorBypassRejected(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	cfg := ref.Config
	cfg.LegalSuccessors = cloneSuccessors(ref.Config.LegalSuccessors)
	cfg.LegalSuccessors[ref.FirstAnchor()] = append(cfg.LegalSuccessors[ref.FirstAnchor()], ref.ShipTerminal())
	sm := NewStateMachine().WithLegalGraph(legalGraphFrom(cfg.LegalSuccessors))
	if !containsSubstr(ValidateSafetyInvariants(sm, cfg, ref.Catalog), "dominate the ship sink") {
		t.Error("an evaluator-bypass edge (a start→ship path avoiding the evaluator) must be rejected at load")
	}
}

// When two evaluators each gate a DISJOINT path to ship, every start→ship
// path still crosses AN evaluator, so the floor holds and the config is safe.
// The dominance check must delete the evaluator SET as a whole — deleting
// only one evaluator would leave the other's path open and false-positive.
// This locks the collective-dominance semantic against a future
// one-at-a-time regression. The Phase constants are bare-graph vocabulary.
func TestValidateSafetyInvariants_TwoEvaluatorsCollectiveDominance(t *testing.T) {
	t.Parallel()
	shippable := &phasespec.ArtifactGate{RequiresPresent: true, VerdictIn: []string{VerdictPASS, VerdictWARN}}
	cat := mustCatalog(t,
		phasespec.PhaseSpec{Name: string(PhaseTriage), Gate: shippable},
		phasespec.PhaseSpec{Name: string(PhaseAudit), Gate: shippable},
		phasespec.PhaseSpec{Name: string(PhaseShip)},
	)
	cfg := config.RoutingConfig{
		Order:     []string{string(PhaseIntent), string(PhaseTriage), string(PhaseAudit), string(PhaseShip)},
		Mandatory: []string{string(PhaseTriage), string(PhaseAudit), string(PhaseShip)},
	}
	// Two disjoint evaluator paths to ship: intent→triage→ship and intent→audit→ship.
	sm := NewStateMachine().WithLegalGraph(map[Phase]map[Phase]bool{
		PhaseIntent: {PhaseTriage: true, PhaseAudit: true},
		PhaseTriage: {PhaseShip: true},
		PhaseAudit:  {PhaseShip: true},
		PhaseShip:   {PhaseEnd: true},
	})
	if v := ValidateSafetyInvariants(sm, cfg, cat); len(v) != 0 {
		t.Errorf("two evaluators collectively dominating ship must pass; got: %v", v)
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
