package semvercheck

import "testing"

func TestIsSemver(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"1.2.3", true},
		{"11.7.2", true},
		{"0.0.0", true},
		{"v1.2.3", false},
		{"1.2", false},
		{"1.2.3.4", false},
		{"1.2.3-alpha", false},
		{"", false},
		{"abc", false},
	}
	for _, tc := range cases {
		if got := IsSemver(tc.in); got != tc.want {
			t.Errorf("IsSemver(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
