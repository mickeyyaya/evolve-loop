package fleet

import (
	"strings"
	"testing"
)

func TestBuildStarvationItem_IsStabilityClassedAndValidateRequiresAClass(t *testing.T) {
	item := BuildStarvationItem(WaveObservation{DesiredLanes: 3, RealizedLanes: 1}, 3, 0.9, 1, "2026-10-06T00:00:00Z")
	if item.PriorityClass != "stability" {
		t.Errorf("a loop running below its committed width is a stability defect; priority_class = %q", item.PriorityClass)
	}
	unclassed := item
	unclassed.PriorityClass = ""
	if err := unclassed.Validate(); err == nil || !strings.Contains(err.Error(), "priority_class") {
		t.Errorf("Validate must refuse an unclassed item by name, got %v", err)
	}
}
