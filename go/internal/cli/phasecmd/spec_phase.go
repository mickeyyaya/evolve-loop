package phasecmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/specrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

func ResolveRunner(name string, req core.PhaseRequest) (core.PhaseRunner, bool, error) {
	name = strings.ToLower(name)
	if factory, ok := registry.For(name); ok {
		return factory(req), true, nil
	}
	root := projectRootOr(req.ProjectRoot)
	spec, ok, err := runnableSpec(name, root)
	if err != nil || !ok {
		return nil, false, err
	}
	return specrunner.New(spec, specrunner.Config{
		Bridge:  bridge.NewDefault(root, nil),
		Prompts: prompts.NewForProject(root),
	}), true, nil
}

func CatalogPhaseNames(projectRoot string) ([]string, []string, error) {
	builtins := registry.Names()
	cat, err := loadCatalog(projectRootOr(projectRoot))
	if err != nil {
		return builtins, nil, err
	}
	specs := make([]string, 0)
	for _, s := range cat.All() {
		if specRunnable(s) {
			specs = append(specs, s.Name)
		}
	}
	sort.Strings(specs)
	return builtins, specs, nil
}

func FormatUnknownPhaseError(command, name, projectRoot string) string {
	builtins, specs, err := CatalogPhaseNames(projectRoot)
	known := strings.Join(builtins, ", ")
	switch {
	case err != nil:
		return fmt.Sprintf("%s: unknown phase %q (known: %s; spec catalog unavailable: %v)", command, name, known, err)
	case len(specs) == 0:
		return fmt.Sprintf("%s: unknown phase %q (known: %s)", command, name, known)
	}
	return fmt.Sprintf("%s: unknown phase %q (known built-in: %s; known spec: %s)", command, name, known, strings.Join(specs, ", "))
}

func IsKnownPhase(name, projectRoot string) bool {
	name = strings.ToLower(name)
	if _, ok := registry.For(name); ok {
		return true
	}
	_, ok, err := runnableSpec(name, projectRootOr(projectRoot))
	return err == nil && ok
}

func runnableSpec(name, root string) (phasespec.PhaseSpec, bool, error) {
	cat, err := loadCatalog(root)
	if err != nil {
		return phasespec.PhaseSpec{}, false, err
	}
	spec, ok := cat.Get(name)
	if !ok || !specRunnable(spec) {
		return phasespec.PhaseSpec{}, false, nil
	}
	return spec, true, nil
}

func loadCatalog(root string) (phasespec.Catalog, error) {
	cat, _, _, err := phasespec.MergedCatalog(root)
	if err != nil {
		return phasespec.Catalog{}, fmt.Errorf("load phase catalog: %w", err)
	}
	return cat, nil
}

func specRunnable(s phasespec.PhaseSpec) bool {
	if _, builtin := registry.For(s.Name); builtin {
		return false
	}
	return s.KindOrDefault() == "llm" && s.RoleOrDefault() != phasespec.RoleControl
}

func projectRootOr(root string) string {
	if root != "" {
		return root
	}
	return cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")
}
