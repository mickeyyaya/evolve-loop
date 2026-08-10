package main

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// phaseRoots delegates to phasespec.Roots — the single home for the
// EVOLVE_PHASE_ROOTS discovery-root policy (ADR-0038), shared with the
// merged-catalog loader so cmd and library consumers can never diverge.
// Kept as a local name for the cmd call sites.
func phaseRoots(projectRoot string) []string {
	return phasespec.Roots(projectRoot)
}

// discoverUserSpecsClamped is the COMPOSED discovery path: discovery-root
// load + the registrar-parity clamp (ADR-0073 on-disk-spec trust gap; see
// phasespec/clamp.go). Every catalog admission of a discovered spec goes
// through here, so writes_source eligibility can never bypass the
// sandboxed-profile verification — the wiring the security review demanded
// live in the composed path, not just a unit.
func discoverUserSpecsClamped(projectRoot string) ([]phasespec.PhaseSpec, []string) {
	specs, _, warns := phasespec.DiscoverUserSpecsFromRoots(phaseRoots(projectRoot))
	clamped, clampWarns := phasespec.ClampDiscoveredSpecs(specs,
		phasespec.SandboxedProfilePredicate(filepath.Join(projectRoot, ".evolve", "profiles")))
	return clamped, append(warns, clampWarns...)
}
