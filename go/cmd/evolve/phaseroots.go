package main

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// phaseRoots delegates to phasespec.Roots — the single home for the
// EVOLVE_PHASE_ROOTS discovery-root policy (ADR-0038), shared with the
// merged-catalog loader so cmd and library consumers can never diverge.
// Kept as a local name for the cmd call sites.
func phaseRoots(projectRoot string) []string {
	return phasespec.Roots(projectRoot)
}
