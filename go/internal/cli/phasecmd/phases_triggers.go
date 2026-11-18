package phasecmd

import (
	"io"
	"maps"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func unproducedTriggerRefusals(project string, subjects, userSpecs []phasespec.PhaseSpec) []string {
	registryPath := config.RegistryPath(project)
	builtin, err := phasespec.Load(registryPath)
	if err != nil {
		return nil
	}
	cfg, _ := router.ConfigForProject(project, io.Discard)
	phasespec.ApplyUserRouting(&cfg, userSpecs, builtin)
	subject := make(map[string]bool, len(subjects))
	for _, s := range subjects {
		subject[s.Name] = true
	}
	for name := range maps.Clone(cfg.Triggers) {
		if !subject[name] {
			delete(cfg.Triggers, name)
		}
	}
	return phasespec.UnproducedTriggerFields(cfg, append(builtin.All(), userSpecs...), router.HarnessProduces)
}

func createSpecRefusals(project string, spec phasespec.PhaseSpec) []string {
	if v := phasespec.ValidateUserSpec(spec); len(v) > 0 {
		return v
	}
	existing, _, _ := phasespec.DiscoverUserSpecsFromRoots(phasespec.Roots(project))
	return unproducedTriggerRefusals(project, []phasespec.PhaseSpec{spec}, append(existing, spec))
}
