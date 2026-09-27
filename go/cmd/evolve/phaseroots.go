package main

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// phaseRoots is the cmd-local alias for phasespec.Roots.
// See ADR-0038.
func phaseRoots(projectRoot string) []string {
	return phasespec.Roots(projectRoot)
}

func discoverUserSpecsClamped(projectRoot string, prm *prompts.Loader) ([]phasespec.PhaseSpec, []string) {
	specs, _, warns := phasespec.DiscoverUserSpecsFromRoots(phaseRoots(projectRoot))
	clamped, clampWarns := phasespec.ClampDiscoveredSpecs(specs,
		phasespec.SandboxedProfilePredicate(filepath.Join(projectRoot, ".evolve", "profiles")))
	demoted, demoteWarns := demotePersonalessSpecs(clamped, prm)
	return demoted, append(append(warns, clampWarns...), demoteWarns...)
}

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
