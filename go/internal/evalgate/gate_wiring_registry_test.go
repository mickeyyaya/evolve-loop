// gate_wiring_registry_test.go — cycle-996 root-cause guard for the
// gate-wiring-binding-tests inbox item.
//
// The per-gate wiring pins (TestQualityGate_WiredIntoReviewer,
// TestFloorBindingGate_WiredIntoReviewer) bind two named gates into
// NewReviewer's composed slice (reviewer.go:39), but nothing FORCES a NEW gate
// added to that slice to carry such a pin — the exact class the item targets
// ("nothing forces cross-package/wiring binding tests for gate enforce paths").
// materializationGate ("evals-materialized") is currently composed yet has no
// wiring pin at all, proving the gap is live.
//
// These two default-suite meta-tests turn the per-gate discipline into a
// forcing function: every gate name returned by newGatesForTest() (the REAL
// production slice) must be registered in pinnedGateWirings, and the registry
// may not carry an entry for a gate no longer composed. Adding a gate to the
// production slice without registering its wiring pin fails the build loudly,
// naming the offender.
package evalgate

import (
	"sort"
	"strings"
	"testing"
)

// pinnedGateWirings maps each production reviewer gate name (gate.name()) to
// the name of the default-suite test that pins it into NewReviewer's composed
// slice. Every gate in newGatesForTest() MUST have an entry here; adding a new
// gate obliges its author to add a wiring pin and register it below.
var pinnedGateWirings = map[string]string{
	"predicate-quality":  "TestQualityGate_WiredIntoReviewer",
	"floor-binding":      "TestFloorBindingGate_WiredIntoReviewer",
	"evals-materialized": "TestMaterializationGate_WiredIntoReviewer",
}

// TestAllReviewerGates_HaveWiringPin is the forcing function: it iterates the
// REAL production slice (newGatesForTest → NewReviewer(...).(*reviewer).gates)
// and asserts every composed gate name is registered in pinnedGateWirings.
// Binding the production slice (not a hardcoded literal list) is what makes the
// guard non-tautological: adding a gate to reviewer.go without a wiring pin
// trips this test naming the unpinned gate.
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

// TestAllReviewerGates_NoStaleWiringPin is the reverse-direction twin: a name
// listed in pinnedGateWirings must still map to a gate present in the
// production slice, so a deleted gate cannot leave a dangling "pinned" claim
// (which would let a real wiring regression hide behind a stale registry row).
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
