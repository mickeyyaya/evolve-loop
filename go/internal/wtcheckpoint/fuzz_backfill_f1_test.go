package wtcheckpoint

import (
	"strings"
	"testing"
)

func FuzzSumNumstatBackfillF1(f *testing.F) {
	for _, seed := range []string{"", "1\t2\ta.go", "-\t-\tbin", "x\t1\ta", "1\ty\ta", "1\t2", "3\t4\ta\tb\n5\t6\tc"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, numstat string) {
		files, added, deleted, err := sumNumstat(numstat)
		if err != nil {
			if files != 0 || added != 0 || deleted != 0 {
				t.Fatalf("an error must come with zero totals, got (%d,%d,%d) for %q", files, added, deleted, numstat)
			}
			return
		}
		wantFiles := 0
		for _, line := range strings.Split(numstat, "\n") {
			if len(strings.SplitN(line, "\t", 3)) == 3 {
				wantFiles++
			}
		}
		if files != wantFiles {
			t.Fatalf("files = %d, want one per three-field line (%d) for %q", files, wantFiles, numstat)
		}
	})
}
