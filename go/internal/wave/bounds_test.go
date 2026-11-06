package wave

import "testing"

func TestBoundsOf_ACeilingIsTheNextFloorWheneverTheNextRecordHasOne(t *testing.T) {
	t.Parallel()
	records := []Record{{Number: 1, CycleFloor: 10}, {Number: 2, CycleFloor: 30}, {Number: 3, CycleFloor: 30}, {Number: 4, CycleFloor: 0}}
	for i, want := range []Bounds{{Floor: 10, Ceiling: 30}, {Floor: 30, Ceiling: 30}, {Floor: 30}, {}} {
		if got := BoundsOf(records, i); got != want {
			t.Errorf("BoundsOf(%d) = %+v, want %+v", i, got, want)
		}
	}
}

func TestBounds_HoldsTheCyclesAboveTheFloorUpToTheCeiling(t *testing.T) {
	t.Parallel()
	closed, open := Bounds{Floor: 10, Ceiling: 30}, Bounds{Floor: 10}
	for _, tc := range []struct {
		b    Bounds
		id   int
		want bool
	}{{closed, 10, false}, {closed, 11, true}, {closed, 30, true}, {closed, 31, false}, {open, 10, false}, {open, 5000, true}} {
		if got := tc.b.Holds(tc.id); got != tc.want {
			t.Errorf("%+v.Holds(%d) = %v, want %v", tc.b, tc.id, got, tc.want)
		}
	}
}
