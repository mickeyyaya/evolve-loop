package core

// safety_invariants_test.go — PA-DDK DDK-1 (ADR-0060): the phase-agnostic
// load-time safety-invariant validator is the relocated trust anchor. These
// tests prove it accepts a floor-valid config and rejects the structural
// attacks that would let a config bypass the non-gameable ship floor. The
// invariants are quantified over the graph + config roles, never phase names.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// TestValidateSafetyInvariants_CleanConfigPasses: a config whose verdict-branch
// targets are legal edges and whose mandatory anchors are reachable raises no
// violation.
func TestValidateSafetyInvariants_CleanConfigPasses(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	cat := mustCatalog(t, phasespec.PhaseSpec{Name: "audit", OnPass: "ship", OnFail: "retrospective"})
	cfg := config.RoutingConfig{Mandatory: []string{"scout", "build", "audit", "ship"}}
	if v := ValidateSafetyInvariants(sm, cfg, cat); len(v) != 0 {
		t.Errorf("clean config must pass; got violations: %v", v)
	}
}

// TestValidateSafetyInvariants_IllegalBranchTarget: a verdict-branch target that
// is NOT a legal successor of its phase is rejected — config may only SELECT
// among already-legal edges (ADR-0058 §1, enforced as a data check).
func TestValidateSafetyInvariants_IllegalBranchTarget(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	// audit's legal successors are {ship, retrospective}; "build" is not one.
	cat := mustCatalog(t, phasespec.PhaseSpec{Name: "audit", OnPass: "build", OnFail: "retrospective"})
	cfg := config.RoutingConfig{Mandatory: []string{"scout", "build", "audit", "ship"}}
	v := ValidateSafetyInvariants(sm, cfg, cat)
	if len(v) == 0 {
		t.Fatal("an illegal on_pass target must be rejected")
	}
	if !containsSubstr(v, "legal successor") {
		t.Errorf("violation should name the illegal-edge reason; got %v", v)
	}
}

// TestValidateSafetyInvariants_UnknownTarget: a verdict-branch target that
// resolves to no known phase is rejected.
func TestValidateSafetyInvariants_UnknownTarget(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	cat := mustCatalog(t, phasespec.PhaseSpec{Name: "audit", OnPass: "ship", OnFail: "no-such-phase"})
	cfg := config.RoutingConfig{Mandatory: []string{"scout", "build", "audit", "ship"}}
	v := ValidateSafetyInvariants(sm, cfg, cat)
	if !containsSubstr(v, "no known phase") {
		t.Errorf("an unresolvable target must be rejected; got %v", v)
	}
}

// TestValidateSafetyInvariants_StrandedMandatoryAnchor: a configured-mandatory
// anchor unreachable from the start node cannot gate the floor and is rejected.
func TestValidateSafetyInvariants_StrandedMandatoryAnchor(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	cat := mustCatalog(t, phasespec.PhaseSpec{Name: "audit", OnPass: "ship", OnFail: "retrospective"})
	// "swarm-plan" is a valid Phase but not on any start→ship path in the graph.
	cfg := config.RoutingConfig{Mandatory: []string{"scout", "swarm-plan", "build", "audit", "ship"}}
	v := ValidateSafetyInvariants(sm, cfg, cat)
	if !containsSubstr(v, "unreachable") {
		t.Errorf("a stranded mandatory anchor must be rejected; got %v", v)
	}
}

// TestValidateSafetyInvariants_IllegalSpineEdge (DDK-3): a config-declared spine
// that jumps an illegal edge — scout→ship, bypassing audit — is rejected, since
// Next walks the spine without re-checking legality.
func TestValidateSafetyInvariants_IllegalSpineEdge(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	cat := mustCatalog(t, phasespec.PhaseSpec{Name: "audit", OnPass: "ship", OnFail: "retrospective"})
	cfg := config.RoutingConfig{
		Mandatory:  []string{"scout", "build", "audit", "ship"},
		SpineOrder: []string{"scout", "ship"}, // scout→ship is not a legal edge
	}
	if !containsSubstr(ValidateSafetyInvariants(sm, cfg, cat), "not a legal transition") {
		t.Error("an illegal spine edge must be rejected")
	}
}

// TestValidateSafetyInvariants_UnknownSpinePhase (DDK-3): a typo in spine_order
// (a name that resolves to no phase) is rejected, not silently dropped.
func TestValidateSafetyInvariants_UnknownSpinePhase(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	cat := mustCatalog(t, phasespec.PhaseSpec{Name: "audit", OnPass: "ship", OnFail: "retrospective"})
	cfg := config.RoutingConfig{
		Mandatory:  []string{"scout", "build", "audit", "ship"},
		SpineOrder: []string{"scout", "scouut", "build"}, // typo
	}
	if !containsSubstr(ValidateSafetyInvariants(sm, cfg, cat), "no known phase") {
		t.Error("an unknown spine_order phase must be rejected")
	}
}

// TestValidateSafetyInvariants_ShippedRegistryPasses is the guard: the real
// registry the composition root loads must satisfy the floor invariants. If a
// future registry edit breaks a branch target or strands an anchor, this fails
// in CI before it can ship.
func TestValidateSafetyInvariants_ShippedRegistryPasses(t *testing.T) {
	t.Parallel()
	cat, err := phasespec.Load(filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatalf("load shipped registry: %v", err)
	}
	cfg, _ := config.Load(filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json"), nil)
	if v := ValidateSafetyInvariants(NewStateMachine(), cfg, cat); len(v) != 0 {
		t.Errorf("shipped registry must satisfy the safety invariants; got: %v", v)
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
