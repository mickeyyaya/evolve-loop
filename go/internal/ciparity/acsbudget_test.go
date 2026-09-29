package ciparity_test

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/audit/ciparitygate"
)

func TestACSDurableTimeout_IsTheBudgetTheCIParityGateGivesPredicates(t *testing.T) {
	if got := ciparitygate.DefaultTimeouts().ACSDurable; got != ciparity.ACSDurableTimeout {
		t.Errorf("ciparitygate ACSDurable = %v, want ciparity.ACSDurableTimeout (%v): one budget for an ACS predicate run", got, ciparity.ACSDurableTimeout)
	}
	if ciparity.ACSDurableTimeout < 5*time.Minute {
		t.Errorf("ACSDurableTimeout = %v: a predicate that runs its own -count=50 -race bound needs minutes", ciparity.ACSDurableTimeout)
	}
}
