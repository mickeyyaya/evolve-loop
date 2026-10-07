//go:build acs

package cycle536

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
	"testing"
)

func TestC536_007_WaveSeedPicksDisjointRepsNotRawTopWeight(t *testing.T) {
	evolveDir := t.TempDir()
	writeInboxTodo(t, evolveDir, "alpha", 0.9, []string{"x.go"})
	writeInboxTodo(t, evolveDir, "beta", 0.8, []string{"x.go"})
	writeInboxTodo(t, evolveDir, "gamma", 0.7, []string{"y.go"})

	reps := triagecap.SelectWaveSeedTopN(evolveDir, 2)
	if len(reps) != 2 {
		t.Fatalf("expected 2 disjoint lane reps for count=2; got %d (%v)", len(reps), repIDs(reps))
	}

	got := repIDs(reps)
	if got[0] != "alpha" {
		t.Errorf("highest-weight candidate must lead the seed; got order %v", got)
	}
	for _, id := range got {
		if id == "beta" {
			t.Errorf("beta shares x.go with alpha and must be rejected as a concurrent lane; got %v", got)
		}
	}
	if got[1] != "gamma" {
		t.Errorf("disjoint runner-up gamma must be the second lane; got %v", got)
	}

	owner := map[string]string{}
	for _, r := range reps {
		for _, f := range r.Files {
			if prev, dup := owner[f]; dup {
				t.Errorf("reps %q and %q both own file %q — lanes are not file-disjoint", prev, r.ID, f)
			}
			owner[f] = r.ID
		}
	}
}

func TestC536_008_WaveSeedCountBelowTwoIsLegacySingleFocus(t *testing.T) {
	evolveDir := t.TempDir()
	writeInboxTodo(t, evolveDir, "alpha", 0.9, []string{"x.go"})
	writeInboxTodo(t, evolveDir, "beta", 0.8, []string{"x.go"})

	reps := triagecap.SelectWaveSeedTopN(evolveDir, 1)
	if len(reps) != 1 {
		t.Fatalf("count<2 must return exactly one candidate; got %d (%v)", len(reps), repIDs(reps))
	}
	if reps[0].ID != "alpha" {
		t.Errorf("count<2 must return the single highest-weight candidate; got %q", reps[0].ID)
	}
}
