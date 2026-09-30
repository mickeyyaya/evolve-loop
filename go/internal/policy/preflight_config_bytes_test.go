package policy

import "testing"

func TestPreflightConfig_MinFreeBytesConvertsGiB(t *testing.T) {
	cases := []struct {
		gib  float64
		want uint64
	}{
		{1, 1 << 30},
		{0.25, 256 << 20},
		{5, 5 << 30},
	}
	for _, c := range cases {
		if got := (PreflightConfig{MinFreeGiB: c.gib}).MinFreeBytes(); got != c.want {
			t.Errorf("MinFreeBytes(%v GiB) = %d, want %d", c.gib, got, c.want)
		}
	}
	if got := (Policy{}).PreflightConfig().MinFreeBytes(); got != 1<<30 {
		t.Errorf("default floor = %d bytes, want 1 GiB", got)
	}
}
