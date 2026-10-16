package phasespec

import (
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const defaultRoot = ".evolve/phases"

const registryRelPath = "docs/architecture/phase-registry.json"

func RootsWithPolicy(projectRoot string, cfg policy.PathsConfig) []string {
	raw := cfg.PhaseRoots
	if strings.TrimSpace(raw) == "" {
		raw = defaultRoot
	}
	var out []string
	for _, p := range strings.Split(raw, ":") {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		if filepath.IsAbs(p) {
			out = append(out, p)
			continue
		}
		out = append(out, filepath.Join(projectRoot, p))
	}
	return out
}

func Roots(projectRoot string) []string {
	roots, _ := RootsWithWarnings(projectRoot)
	return roots
}

func RootsWithWarnings(projectRoot string) ([]string, []string) {
	pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	roots := RootsWithPolicy(projectRoot, pol.PathsConfig())
	if err != nil {
		return roots, []string{"PHASE_ROOTS_POLICY_UNREADABLE: " + err.Error() + "; paths.phase_roots ignored, discovering only " + defaultRoot}
	}
	return roots, nil
}

func MergedCatalog(projectRoot string) (Catalog, map[string]string, []string, error) {
	builtin, err := Load(filepath.Join(projectRoot, filepath.FromSlash(registryRelPath)))
	if err != nil {
		return Catalog{}, nil, nil, err
	}
	roots, warns := RootsWithWarnings(projectRoot)
	user, sources, discWarns := DiscoverUserSpecsFromRoots(roots)
	warns = append(warns, discWarns...)
	user, clampWarns := ClampDiscoveredSpecs(user,
		SandboxedProfilePredicate(filepath.Join(projectRoot, ".evolve", "profiles")))
	warns = append(warns, clampWarns...)
	merged, mWarns := builtin.Merge(user)
	return merged, sources, append(warns, mWarns...), nil
}
