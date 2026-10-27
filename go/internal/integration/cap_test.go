package integration

import "testing"

func TestCapBoundsAndDefaults(t *testing.T) {
	if DefaultCap != 4 || MinCap != 1 || MaxCap != 4 {
		t.Errorf("cap bounds = default %d min %d max %d, want 4, 1, 4", DefaultCap, MinCap, MaxCap)
	}
	if DefaultMaxAgeMinutes != 45 {
		t.Errorf("DefaultMaxAgeMinutes = %d, want 45", DefaultMaxAgeMinutes)
	}
	policy := DefaultFlushPolicy()
	if policy.Cap != 4 || policy.MaxAgeMinutes != 45 {
		t.Errorf("DefaultFlushPolicy() = %+v, want cap 4 and max age 45", policy)
	}
}

func TestHalveCapOnConfirmedRed(t *testing.T) {
	cases := []struct{ from, want int }{{4, 2}, {3, 1}, {2, 1}, {1, 1}}
	for _, c := range cases {
		if got := HalveCap(c.from); got != c.want {
			t.Errorf("HalveCap(%d) = %d, want %d", c.from, got, c.want)
		}
	}
}

func TestGrowCapOnGreen(t *testing.T) {
	cases := []struct{ from, want int }{{1, 2}, {2, 3}, {3, 4}, {4, 4}}
	for _, c := range cases {
		if got := GrowCap(c.from); got != c.want {
			t.Errorf("GrowCap(%d) = %d, want %d", c.from, got, c.want)
		}
	}
}

func TestCapClampsOutOfRangeInput(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"halve above the max", HalveCap(100), MaxCap},
		{"halve zero", HalveCap(0), MinCap},
		{"halve negative", HalveCap(-6), MinCap},
		{"grow above the max", GrowCap(9), MaxCap},
		{"grow zero", GrowCap(0), MinCap},
		{"grow negative", GrowCap(-3), MinCap},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestCapSequenceFromDefaultStaysInsideBounds(t *testing.T) {
	capNow := DefaultCap
	for i, want := range []int{2, 1, 1} {
		capNow = HalveCap(capNow)
		if capNow != want {
			t.Fatalf("red %d: cap = %d, want %d", i+1, capNow, want)
		}
	}
	for i, want := range []int{2, 3, 4, 4, 4} {
		capNow = GrowCap(capNow)
		if capNow != want {
			t.Fatalf("green %d: cap = %d, want %d", i+1, capNow, want)
		}
	}
}
