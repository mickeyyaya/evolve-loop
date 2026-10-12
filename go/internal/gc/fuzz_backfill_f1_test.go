package gc

import "testing"

func FuzzParseCwdListingBackfillF1(f *testing.F) {
	for _, seed := range []string{"", "p1\nR0\nn/a\n", "p9\n\nn/x\np10\nn/y\nR9\n", "pX\nn/z\n", "n/orphan\n", "p5\nR5\n"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, listing string) {
		for _, p := range parseCwdListing(listing) {
			if p.Pid <= 0 || p.Cwd == "" {
				t.Fatalf("emitted a process without a pid or cwd: %+v from %q", p, listing)
			}
		}
	})
}
