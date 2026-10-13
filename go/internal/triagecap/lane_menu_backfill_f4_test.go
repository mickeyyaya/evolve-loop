package triagecap

import (
	"reflect"
	"testing"
)

func TestExpandWithClusterMates_BelowTwoPerLaneEveryLaneKeepsOnlyItsRepresentative(t *testing.T) {
	selection := []FleetCandidate{{ID: "a", Files: []string{"x.go"}}, {ID: "b", Files: []string{"y.go"}}}
	backlog := []FleetCandidate{{ID: "mate", Files: []string{"x.go"}}}

	for _, perLane := range []int{-1, 0, 1} {
		got := ExpandWithClusterMates(selection, backlog, perLane)
		want := [][]FleetCandidate{{selection[0]}, {selection[1]}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("perLane=%d: menus = %+v, want %+v", perLane, got, want)
		}
	}
	got := ExpandWithClusterMates(selection, backlog, 2)
	if len(got[0]) != 2 || got[0][1].ID != "mate" || len(got[1]) != 1 {
		t.Errorf("at perLane=2 the mate that overlaps only lane 0 joins it: %+v", got)
	}
}
