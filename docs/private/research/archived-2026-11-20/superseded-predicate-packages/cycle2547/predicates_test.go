//go:build acs

// Package cycle2547 holds the predicates of the agy routing document contract.
package cycle2547

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func requireCliroute(t *testing.T, names ...string) {
	t.Helper()
	pattern := "^("
	for i, name := range names {
		if i > 0 {
			pattern += "|"
		}
		pattern += name
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./internal/cliroute",
		Pattern: pattern + ")$",
		Names:   names,
	})
}

func TestC2547_001_TheTierTableAndBulletsStateTheChainsOfThePolicy(t *testing.T) {
	requireCliroute(t, "TestModelRoutingDocMatchesPolicy")
}

func TestC2547_002_ADR0150AmendsADR0136InBothRecords(t *testing.T) {
	requireCliroute(t, "TestADR0150AmendsADR0136")
}

func TestC2547_003_TheDriftCheckRejectsAStaleChainAnEmptyTableAndAnUnbuiltKey(t *testing.T) {
	requireCliroute(t, "TestModelRoutingDocDriftIsDetected")
}
