// materialization_wiring_test.go — cycle-996 wiring pin for materializationGate
// ("evals-materialized") in NewReviewer's composed gate list (reviewer.go:39).
//
// materialization_test.go exercises materializationGate{}.check() DIRECTLY but
// never through NewReviewer(...).Review(...), so deleting materializationGate{}
// from the composition slice passes the direct-.check() tests while silently
// dropping the scout-phase evals-materialized enforcement. This DEFAULT-SUITE
// test closes that blind spot, mirroring TestQualityGate_WiredIntoReviewer and
// TestFloorBindingGate_WiredIntoReviewer, and is the pin registered for
// "evals-materialized" in pinnedGateWirings (gate_wiring_registry_test.go).

package evalgate

import "testing"

// TestMaterializationGate_WiredIntoReviewer pins materializationGate
// ("evals-materialized") into the production gate list. Without it, the
// direct-.check() tests in materialization_test.go are testing an orphan.
func TestMaterializationGate_WiredIntoReviewer(t *testing.T) {
	found := false
	for _, g := range newGatesForTest() {
		if g.name() == "evals-materialized" {
			found = true
		}
	}
	if !found {
		t.Fatal("materializationGate is not wired into NewReviewer's gate list")
	}
}
