package gc

import "testing"

func TestLeafCycleNumber(t *testing.T) {
	cases := []struct {
		leaf string
		want int
		ok   bool
	}{
		{"cycle-aaa1111-570", 570, true},
		{"cycle-legacyB-8-integration", 8, true},
		{"cycle-legacyC-9-w0", 9, true},
		{"cycle-100", 100, true},
		{"cycle-lane-", 0, false},
		{"cycle-lane-abc", 0, false},
		{"nodash", 0, false},
	}
	for _, c := range cases {
		got, ok := LeafCycleNumber(c.leaf)
		if got != c.want || ok != c.ok {
			t.Errorf("LeafCycleNumber(%q) = (%d, %t), want (%d, %t)", c.leaf, got, ok, c.want, c.ok)
		}
	}
}
