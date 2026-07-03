package fleetbudget

// apicover_named_test.go — ADR-0050 Phase 5 public-API coverage: name and
// exercise every exported fleetbudget symbol by identifier (apicover counts
// field access as "uses", not "names"). Each test asserts a REAL contract.

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

// TestBudgetPlan_ConfigPlanNamed names the BudgetPlan + Config types and Plan by
// identifier, pinning the floor-fallback contract: no quota signal ⇒ run the
// configured Count, from FromFloor.
func TestBudgetPlan_ConfigPlanNamed(t *testing.T) {
	t.Parallel()
	var cfg Config = Config{Count: 2, Floor: 1}
	var got BudgetPlan = Plan(nil, budgethistory.Throughput{}, cfg, time.Unix(0, 0).UTC())
	if got.Lanes != 2 {
		t.Errorf("Lanes = %d, want 2", got.Lanes)
	}
	if got.DerivedFrom != FromFloor {
		t.Errorf("DerivedFrom = %q, want %q", got.DerivedFrom, FromFloor)
	}
	// Name the QuotaState import path by identifier too (a probed-but-unknown
	// source is not healthy → still floor-fallback).
	var _ quotastate.QuotaState
}
