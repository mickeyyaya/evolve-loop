//go:build acs

// Package cycle2561 holds the predicates of the agy routing document contract.
package cycle2561

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

func TestC2561_001_TheRoutingBulletsOfAgentsMdStateTheChainsOfThePolicy(t *testing.T) {
	requireCliroute(t, "TestAgentsMdRoutingMatchesPolicy")
}

func TestC2561_002_TheTierTableAndBulletsOfModelRoutingStateTheChainsOfThePolicy(t *testing.T) {
	requireCliroute(t, "TestModelRoutingDocMatchesPolicy")
}

func TestC2561_003_ADR0150AmendsADR0136InBothRecords(t *testing.T) {
	requireCliroute(t, "TestADR0150AmendsADR0136")
}

func TestC2561_004_TheDriftChecksRejectAStaleChainAnEmptyTableAnUnbuiltKeyAndAnUnknownModel(t *testing.T) {
	requireCliroute(t, "TestModelRoutingDocDriftIsDetected", "TestAgentsMdRoutingDriftIsDetected")
}
