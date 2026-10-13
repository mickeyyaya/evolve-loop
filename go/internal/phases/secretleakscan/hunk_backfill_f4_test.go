package secretleakscan

import "testing"

func TestHunkNewStart_OnlyAWellFormedNonNegativeNewRangeYieldsALine(t *testing.T) {
	cases := []struct {
		line string
		want int
	}{
		{"@@ -1,3 +10,4 @@", 10},
		{"@@ -1 +7 @@ func f()", 7},
		{"@@ -1 +0,0 @@", 0},
		{"@@ -1 +abc @@", 0},
		{"@@ -1 +-5 @@", 0},
		{"@@ -1 -3 @@", 0},
		{"@@ -1", 0},
		{"", 0},
	}
	for _, tc := range cases {
		if got := hunkNewStart(tc.line); got != tc.want {
			t.Errorf("hunkNewStart(%q) = %d, want %d", tc.line, got, tc.want)
		}
	}
}
