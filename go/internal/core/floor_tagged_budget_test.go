package core

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

func TestTaggedTestArgs_RunAPredicatePackageUnderTheCIParityBudget(t *testing.T) {
	args := taggedTestArgs("./acs/cycle1763", []string{"acs"})

	i := slices.Index(args, "-timeout")
	if i < 0 || i+1 >= len(args) || args[i+1] != ciparity.ACSDurableTimeout.String() {
		t.Fatalf("args = %v, want -timeout %s: the floor must not kill a predicate the CI-parity gate budgets that long (cycle 1763 edited the floor's 120s to 480s to survive)", args, ciparity.ACSDurableTimeout)
	}
	if !slices.Equal(args[len(args)-3:], []string{"-tags", "acs", "./acs/cycle1763"}) {
		t.Errorf("args = %v, want the tags then the package last", args)
	}
}
