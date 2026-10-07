package failurelearning

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
)

func TestRemediationItems_AreCorrectnessClassed(t *testing.T) {
	items := New(floorClock, carryover.New()).remediationItems(Failure{Cycle: 9}, []string{"a defect in the work"}, 0.75)
	if len(items) != 1 || items[0].PriorityClass != "correctness" {
		t.Errorf("a defect a failed phase reported is a correctness item: %+v", items)
	}
}
