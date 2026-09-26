package evalgate

import "testing"

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
