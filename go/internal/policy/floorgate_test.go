package policy

import (
	"encoding/json"
	"testing"
)

func TestFloorGate_ParsedFromFloorKey(t *testing.T) {
	const js = `{
	  "version": 1,
	  "floor": [
	    {"id": "dossier-closeout", "description": "every completed cycle writes a dossier", "enforced_since_cycle": 2}
	  ]
	}`
	var p Policy
	if err := json.Unmarshal([]byte(js), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(p.Floor) != 1 {
		t.Fatalf("want 1 floor gate parsed, got %d", len(p.Floor))
	}
	var g FloorGate = p.Floor[0] // names the type for apicover
	if g.ID != "dossier-closeout" {
		t.Errorf("ID = %q, want dossier-closeout", g.ID)
	}
	if g.EnforcedSinceCycle != 2 {
		t.Errorf("EnforcedSinceCycle = %d, want 2", g.EnforcedSinceCycle)
	}
	if g.Description == "" {
		t.Error("Description should round-trip from the floor entry")
	}
}

func TestPolicy_FloorEnrolls(t *testing.T) {
	p := Policy{Floor: []FloorGate{{ID: "dossier-closeout"}}}
	if !p.FloorEnrolls("dossier-closeout") {
		t.Error("FloorEnrolls(dossier-closeout) = false, want true")
	}
	if p.FloorEnrolls("not-a-gate") {
		t.Error("FloorEnrolls(not-a-gate) = true, want false")
	}
	if (Policy{}).FloorEnrolls("dossier-closeout") {
		t.Error("empty policy must enroll nothing")
	}
}
