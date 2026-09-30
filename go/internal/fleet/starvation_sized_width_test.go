package fleet

import "testing"

func TestWaveObservation_StarvedComparesRealizedWithSizedWidth(t *testing.T) {
	cases := []struct {
		name          string
		sized, actual int
		want          bool
	}{
		{"short of the sized width is starved", 2, 1, true},
		{"zero of the sized width is starved", 2, 0, true},
		{"full sized width is not starved", 2, 2, false},
		{"a quota-shrunk wave that delivers its sized width is not starved", 1, 1, false},
		{"a quota-shrunk wave that misses its sized width is still starved", 2, 1, true},
		{"nothing sized is not starved", 0, 0, false},
	}
	for _, c := range cases {
		got := WaveObservation{SizedLanes: c.sized, RealizedLanes: c.actual}.Starved()
		if got != c.want {
			t.Errorf("%s: Starved()=%v want %v", c.name, got, c.want)
		}
	}
}

func TestStarvationTracker_FiresOnShortfallAgainstSizedWidth(t *testing.T) {
	var tr StarvationTracker
	short := WaveObservation{SizedLanes: 2, RealizedLanes: 0}
	if tr.Observe(short, 2) {
		t.Fatal("fired on the first short wave with K=2")
	}
	if !tr.Observe(short, 2) {
		t.Fatal("did not fire on the second consecutive short wave")
	}
	full := WaveObservation{SizedLanes: 1, RealizedLanes: 1}
	tr.Observe(short, 3)
	if tr.Observe(full, 3) || tr.Streak() != 0 {
		t.Fatalf("a wave delivering its sized width must reset the streak, got %d", tr.Streak())
	}
}
