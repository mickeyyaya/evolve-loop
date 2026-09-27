package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// postShipObserverOrchestrator builds a minimal orchestrator: nil
// storage/ledger/runners are fine because postShipObserverSkip is a pure
// decision over (catalog, cfg, phase, shipped).
func postShipObserverOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()
	cat, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{Name: "memo", Optional: true, After: "ship"},
	})
	if err != nil {
		t.Fatalf("setup: catalog merge: %v", err)
	}
	cfg := config.RoutingConfig{Mandatory: []string{"build", "audit", "ship"}}
	return NewOrchestrator(nil, nil, nil, WithCatalog(cat), WithRouting(cfg, nil))
}

func TestPostShipObserverSkip_MemoAfterShipIsNonFatal(t *testing.T) {
	t.Parallel()
	o := postShipObserverOrchestrator(t)
	if !o.postShipObserverSkip(Phase("memo"), true) {
		t.Error("a memo failure AFTER a healthy ship must be non-fatal (degrade to WARN + advance), " +
			"never a cycle-level failure that turns a shipped cycle abnormal (inbox memo-phase-tier-envelope)")
	}
}

func TestPostShipObserverSkip_MemoBeforeShipStaysFatal(t *testing.T) {
	t.Parallel()
	o := postShipObserverOrchestrator(t)
	if o.postShipObserverSkip(Phase("memo"), false) {
		t.Error("a memo failure BEFORE ship must stay cycle-fatal — the non-fatal downgrade applies only once a " +
			"ship has landed (shipped==true); swallowing it pre-ship would hide real failures")
	}
}

func TestPostShipObserverSkip_MandatoryFloorNeverSkipped(t *testing.T) {
	t.Parallel()
	o := postShipObserverOrchestrator(t)
	if o.postShipObserverSkip(Phase("build"), true) {
		t.Error("a mandatory/floor phase (build) must never be post-ship-skipped — the observer downgrade must not " +
			"weaken the integrity floor")
	}
}

func TestPostShipObserverSkip_ShipItselfNeverSkipped(t *testing.T) {
	t.Parallel()
	o := postShipObserverOrchestrator(t)
	if o.postShipObserverSkip(PhaseShip, true) {
		t.Error("ship must never be post-ship-skipped — it is the ship gate itself, not a best-effort observer")
	}
}
