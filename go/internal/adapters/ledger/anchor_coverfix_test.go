package ledger

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestSealChainsFromPrev(t *testing.T) {
	t.Parallel()
	const prevSHA = "abc123"
	cases := []struct {
		name    string
		hasPrev bool
		prevSHA string
		prev    string
		want    bool
	}{
		{"no prev_hash key never chains", false, prevSHA, prevSHA, false},
		{"first line chains only from the zero seed", true, "", ZeroSeed, true},
		{"first line with a non-zero prev does not chain", true, "", prevSHA, false},
		{"later line chains from its physical predecessor", true, prevSHA, prevSHA, true},
		{"later line with a zero seed does not chain", true, prevSHA, ZeroSeed, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := core.LedgerEntry{PrevHash: tc.prev}
			if got := sealChainsFromPrev(tc.hasPrev, e, tc.prevSHA); got != tc.want {
				t.Fatalf("sealChainsFromPrev = %v, want %v", got, tc.want)
			}
		})
	}
}
