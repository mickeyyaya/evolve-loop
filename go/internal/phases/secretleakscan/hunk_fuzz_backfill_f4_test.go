package secretleakscan

import "testing"

func FuzzHunkNewStart(f *testing.F) {
	for _, seed := range []string{"@@ -1,3 +10,4 @@", "@@ -1 +7 @@", "@@ -1 +abc @@", "@@ -1 +-5 @@", "", "@@", "@@ -0,0 +99999999999999999999 @@"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, line string) {
		if got := hunkNewStart(line); got < 0 {
			t.Fatalf("hunkNewStart(%q) = %d: a line number is never negative", line, got)
		}
	})
}
