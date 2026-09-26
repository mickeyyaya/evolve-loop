package phasespec

import (
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const defaultRoot = ".evolve/phases"

const registryRelPath = "docs/architecture/phase-registry.json"

// RootsWithPolicy splits cfg.PhaseRoots on ":" (default .evolve/phases), joining relative entries to projectRoot.
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

// Roots returns the discovery roots configured in the project's .evolve/policy.json.
func Roots(projectRoot string) []string {
	pol, _ := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	return RootsWithPolicy(projectRoot, pol.PathsConfig())
}

// MergedCatalog is every consumer's one loader: registry plus clamped overlays; sources maps each user phase to its root.
func MergedCatalog(projectRoot string) (Catalog, map[string]string, []string, error) {
	builtin, err := Load(filepath.Join(projectRoot, filepath.FromSlash(registryRelPath)))
	if err != nil {
		return Catalog{}, nil, nil, err
	}
	user, sources, warns := DiscoverUserSpecsFromRoots(Roots(projectRoot))
	// Every admission seam clamps, so listing and lint report the writes_source that dispatch enforces.
	user, clampWarns := ClampDiscoveredSpecs(user,
		SandboxedProfilePredicate(filepath.Join(projectRoot, ".evolve", "profiles")))
	warns = append(warns, clampWarns...)
	merged, mWarns := builtin.Merge(user)
	return merged, sources, append(warns, mWarns...), nil
}
