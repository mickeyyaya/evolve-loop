package core

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// amplNewSkipOrchestrator builds the minimal *Orchestrator that
// optionalInfraSkip/postShipObserverSkip read: cfg.Mandatory, catalog, and
// shipFloor. Constructed entirely through phasespec's exported Merge so no
// unexported catalog internals are assumed.
func amplNewSkipOrchestrator(t *testing.T, mandatory, floor []string, specs []phasespec.PhaseSpec) *Orchestrator {
	t.Helper()
	cat, warnings := (phasespec.Catalog{}).Merge(specs)
	if len(warnings) != 0 {
		t.Fatalf("unexpected catalog merge warnings: %v", warnings)
	}
	return &Orchestrator{cfg: config.RoutingConfig{Mandatory: mandatory}, catalog: cat, shipFloor: floor}
}

// These target the exact two predicates (optionalInfraSkip / postShipObserverSkip)
// that Task 3 (evaluate-batch-retry-parity) newly wires into dispatchRunnerWithRetry's
// give-up path. The AC table names them directly ("optionalInfraSkip parity",
// "postShipObserverSkip parity"), so their edge-case correctness is exactly what the
// batch dispatch's new behavior now depends on for every phase, not just the three
// scenarios the RED suite already covers.

func TestOptionalInfraSkip_WrappedArtifactTimeoutError_StillMatches(t *testing.T) {
	o := amplNewSkipOrchestrator(t, nil, nil, []phasespec.PhaseSpec{
		{Name: "learn", Optional: true},
	})
	wrapped := fmt.Errorf("bridge dispatch: %w", ErrArtifactTimeout)
	if !o.optionalInfraSkip(Phase("learn"), wrapped) {
		t.Fatalf("expected a wrapped ErrArtifactTimeout to still satisfy errors.Is and match")
	}
}

func TestOptionalInfraSkip_NonInfraError_NeverMatches(t *testing.T) {
	o := amplNewSkipOrchestrator(t, nil, nil, []phasespec.PhaseSpec{
		{Name: "learn", Optional: true},
	})
	if o.optionalInfraSkip(Phase("learn"), errors.New("logic bug: nil pointer")) {
		t.Fatalf("expected an arbitrary non-infra error to never be swallowed, even on an optional off-floor phase")
	}
}

func TestOptionalInfraSkip_MandatoryOverridesOptionalFlag(t *testing.T) {
	// A phase mis-marked Optional=true in the catalog but ALSO listed in
	// cfg.Mandatory must never skip: the mandatory guard is generic and
	// config-driven, so it must win regardless of the catalog flag.
	o := amplNewSkipOrchestrator(t, []string{"build"}, nil, []phasespec.PhaseSpec{
		{Name: "build", Optional: true},
	})
	if o.optionalInfraSkip(Phase("build"), ErrArtifactTimeout) {
		t.Fatalf("expected the mandatory guard to override a mis-marked Optional=true catalog entry")
	}
}

func TestOptionalInfraSkip_OnFloorPhase_NeverMatches(t *testing.T) {
	o := amplNewSkipOrchestrator(t, nil, []string{"audit"}, []phasespec.PhaseSpec{
		{Name: "audit", Optional: true},
	})
	if o.optionalInfraSkip(Phase("audit"), ErrArtifactTimeout) {
		t.Fatalf("expected a phase inside the resolved ship floor to never skip, even if catalog-Optional")
	}
}

func TestOptionalInfraSkip_UnknownPhaseNotInCatalog_NeverMatches(t *testing.T) {
	o := amplNewSkipOrchestrator(t, nil, nil, nil)
	if o.optionalInfraSkip(Phase("ghost-phase"), ErrArtifactTimeout) {
		t.Fatalf("expected a phase absent from the catalog to never skip (fail closed on a catalog miss)")
	}
}

func TestPostShipObserverSkip_NotYetShipped_NeverMatchesRegardlessOfPhase(t *testing.T) {
	o := amplNewSkipOrchestrator(t, nil, nil, []phasespec.PhaseSpec{
		{Name: "memo", Optional: true, Role: string(phasespec.RoleControl)},
	})
	if o.postShipObserverSkip(Phase("memo"), false) {
		t.Fatalf("expected shipped=false to short-circuit to false regardless of phase shape")
	}
}

func TestPostShipObserverSkip_ShipItself_NeverMatchesEvenIfShipped(t *testing.T) {
	o := amplNewSkipOrchestrator(t, nil, nil, []phasespec.PhaseSpec{
		{Name: string(PhaseShip), Optional: true, Role: string(phasespec.RoleControl)},
	})
	if o.postShipObserverSkip(PhaseShip, true) {
		t.Fatalf("expected ship itself to be explicitly excluded from the post-ship skip, even when shipped=true")
	}
}

func TestPostShipObserverSkip_WrongRole_NeverMatches(t *testing.T) {
	// Optional + shipped + not-ship, but role is Evaluate not Control: must
	// not match; only RoleControl observers (memo, post-ship-monitor) qualify.
	o := amplNewSkipOrchestrator(t, nil, nil, []phasespec.PhaseSpec{
		{Name: "evaluate-batch-check", Optional: true, Role: string(phasespec.RoleEvaluate)},
	})
	if o.postShipObserverSkip(Phase("evaluate-batch-check"), true) {
		t.Fatalf("expected a non-RoleControl optional phase to never match postShipObserverSkip")
	}
}

func TestPostShipObserverSkip_MandatoryOverridesEvenWhenShippedAndControl(t *testing.T) {
	o := amplNewSkipOrchestrator(t, []string{"memo"}, nil, []phasespec.PhaseSpec{
		{Name: "memo", Optional: true, Role: string(phasespec.RoleControl)},
	})
	if o.postShipObserverSkip(Phase("memo"), true) {
		t.Fatalf("expected the mandatory guard to override an otherwise-matching post-ship RoleControl observer")
	}
}
