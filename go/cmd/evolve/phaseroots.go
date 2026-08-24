package main

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
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
// phasespec/clamp.go) + the persona-resolvability demotion (cycle-1551; see
// demotePersonalessSpecs). Every catalog admission of a discovered spec goes
// through here, so writes_source eligibility can never bypass the
// sandboxed-profile verification and an undispatchable phase can never reach
// the SELECT menu — the wiring lives in the composed path, not just a unit.
func discoverUserSpecsClamped(projectRoot string, prm *prompts.Loader) ([]phasespec.PhaseSpec, []string) {
	specs, _, warns := phasespec.DiscoverUserSpecsFromRoots(phaseRoots(projectRoot))
	clamped, clampWarns := phasespec.ClampDiscoveredSpecs(specs,
		phasespec.SandboxedProfilePredicate(filepath.Join(projectRoot, ".evolve", "profiles")))
	demoted, demoteWarns := demotePersonalessSpecs(clamped, prm)
	return demoted, append(append(warns, clampWarns...), demoteWarns...)
}

// demotePersonalessSpecs returns a copy of specs with every phase whose
// persona doc cannot be loaded demoted to catalog:"on-demand" IN MEMORY, plus
// a warning per demotion. The SELECT menu must never offer a phase the runner
// cannot dispatch: cycle-1551 (soak-20260824a) died rc=4 when the advisor
// inserted defect-disposition-preflight, whose derived persona exists nowhere.
// This is the runtime seam counterpart of registerBuiltinSpecRunners' persona
// check for builtin spec phases — it covers TRACKED and UNTRACKED specs on any
// host, where the repo-catalog guard test only sees tracked ones. Demotion,
// not removal: the phase stays requestable on demand, and the dispatch-time
// fail-soft (core.ErrAgentDocMissing → optionalInfraSkip) bounds the blast
// radius if it is dispatched anyway. Control-role and non-llm phases are
// exempt (they do not dispatch through prompts.Loader.Agent).
func demotePersonalessSpecs(specs []phasespec.PhaseSpec, prm *prompts.Loader) ([]phasespec.PhaseSpec, []string) {
	var warns []string
	out := make([]phasespec.PhaseSpec, len(specs))
	copy(out, specs)
	for i, s := range out {
		if s.Catalog == phasespec.CatalogOnDemand || s.KindOrDefault() != "llm" || s.RoleOrDefault() == phasespec.RoleControl {
			continue
		}
		if _, err := prm.Agent(s.AgentName()); err != nil {
			warns = append(warns, "phase "+s.Name+": persona "+s.AgentName()+".md unresolvable ("+err.Error()+") — demoted to catalog:\"on-demand\" for this run; write agents/"+s.AgentName()+".md to restore it to the SELECT menu")
			out[i].Catalog = phasespec.CatalogOnDemand
		}
	}
	return out, warns
}
