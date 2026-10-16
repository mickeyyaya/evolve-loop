package fleetbudget

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

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
	var _ quotastate.QuotaState
}
