package policy

import (
	"os"
	"strings"
	"testing"
)

func TestFloorPhases_AbsentMeansUseDefault(t *testing.T) {
	p := Policy{}
	floor, overridden := p.FloorPhases()
	if overridden {
		t.Errorf("absent ship_floor must report overridden=false (orchestrator uses router default)")
	}
	if floor != nil {
		t.Errorf("absent ship_floor must return nil floor, got %v", floor)
	}
}

func TestFloorPhases_AuditOnlyOptIn(t *testing.T) {
	p := Policy{ShipFloor: []string{"audit"}}
	floor, overridden := p.FloorPhases()
	if !overridden {
		t.Fatal("explicit ship_floor must report overridden=true")
	}
	if len(floor) != 1 || floor[0] != "audit" {
		t.Errorf("floor = %v, want [audit]", floor)
	}
}

func TestFloorPhases_AuditIsNonRemovable(t *testing.T) {
	// A user floor that omits audit must still get audit appended — the one
	// gate that can never be dropped, even by typo.
	p := Policy{ShipFloor: []string{"build"}}
	floor, overridden := p.FloorPhases()
	if !overridden {
		t.Fatal("explicit ship_floor must report overridden=true")
	}
	if !contains(floor, "audit") {
		t.Errorf("floor %v must contain non-removable 'audit'", floor)
	}
	if !contains(floor, "build") {
		t.Errorf("floor %v must preserve the user's 'build'", floor)
	}
}

func TestFloorPhases_PreservesUserOrderAuditLast(t *testing.T) {
	p := Policy{ShipFloor: []string{"tdd", "build"}}
	floor, _ := p.FloorPhases()
	want := "tdd,build,audit"
	if got := strings.Join(floor, ","); got != want {
		t.Errorf("floor = %q, want %q (audit appended last)", got, want)
	}
}

func TestLoadParsesShipFloor(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/policy.json"
	if err := os.WriteFile(path, []byte(`{"ship_floor":["audit"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ShipFloor) != 1 || p.ShipFloor[0] != "audit" {
		t.Errorf("ShipFloor = %v, want [audit]", p.ShipFloor)
	}
}
