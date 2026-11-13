package mainstop

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
)

func TestProofReds_AsksForEachKeyWithTheTagsOfTheFloor(t *testing.T) {
	t.Parallel()
	keys := []baseattr.Key{{Package: "example.com/m/a", Test: "TestA"}, {Package: "example.com/m/b", Test: "TestB"}}

	reds := ProofReds(keys)

	if !slices.Equal(reds.Keys, keys) {
		t.Errorf("keys = %v, want %v", reds.Keys, keys)
	}
	if !slices.Equal(reds.Flags, []string{"-tags", "integration"}) {
		t.Errorf("flags = %v, want the integration tag, because a key of the stop can be an integration test", reds.Flags)
	}
}
