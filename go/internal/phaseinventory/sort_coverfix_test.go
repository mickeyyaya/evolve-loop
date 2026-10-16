package phaseinventory

import (
	"reflect"
	"testing"
)

func TestSortByName_OrdersPhasesAndCategoriesByName(t *testing.T) {
	t.Parallel()
	inv := &Inventory{
		Phases: []PhaseEntry{{Name: "ship"}, {Name: "audit"}, {Name: "build"}},
		CategoryIndex: map[string][]string{
			"delivery": {"ship", "audit"},
		},
	}
	sortByName(inv)
	gotNames := []string{inv.Phases[0].Name, inv.Phases[1].Name, inv.Phases[2].Name}
	if want := []string{"audit", "build", "ship"}; !reflect.DeepEqual(gotNames, want) {
		t.Errorf("phase order = %v, want %v", gotNames, want)
	}
	if want := []string{"audit", "ship"}; !reflect.DeepEqual(inv.CategoryIndex["delivery"], want) {
		t.Errorf("category order = %v, want %v", inv.CategoryIndex["delivery"], want)
	}
}
