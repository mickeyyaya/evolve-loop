package evalgate

import (
	"sort"
	"strings"
	"testing"
)

// pinnedGateWirings maps each gate in NewReviewer's slice to the test that pins it there.
var pinnedGateWirings = map[string]string{
	"predicate-quality":     "TestQualityGate_WiredIntoReviewer",
	"floor-binding":         "TestFloorBindingGate_WiredIntoReviewer",
	"evals-materialized":    "TestMaterializationGate_WiredIntoReviewer",
	"flaky-predicate-shape": "TestFlakyShapeGate_WiredIntoReviewer",
}

func TestAllReviewerGates_HaveWiringPin(t *testing.T) {
	var unpinned []string
	for _, g := range newGatesForTest() {
		if _, ok := pinnedGateWirings[g.name()]; !ok {
			unpinned = append(unpinned, g.name())
		}
	}
	if len(unpinned) > 0 {
		sort.Strings(unpinned)
		t.Fatalf("reviewer gate(s) have no wiring pin registered in pinnedGateWirings: %s — every gate composed into NewReviewer must carry a wiring binding test (see TestQualityGate_WiredIntoReviewer)", strings.Join(unpinned, ", "))
	}
}

func TestAllReviewerGates_NoStaleWiringPin(t *testing.T) {
	live := make(map[string]bool)
	for _, g := range newGatesForTest() {
		live[g.name()] = true
	}
	var stale []string
	for name := range pinnedGateWirings {
		if !live[name] {
			stale = append(stale, name)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Fatalf("pinnedGateWirings lists gate(s) not present in NewReviewer's slice: %s — a removed gate left a dangling wiring pin claim; delete the stale registry entry", strings.Join(stale, ", "))
	}
}
