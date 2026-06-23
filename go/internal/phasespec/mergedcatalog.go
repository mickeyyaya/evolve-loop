package phasespec

import (
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/policy"
)

// mergedcatalog.go — the ONE merged-catalog loader (built-in registry +
// user phase overlays). Previously cmd/evolve-local, which left non-cmd
// consumers (the runner's reconcile-on-timeout default) resolving phases
// under a second, builtin-only policy. Every consumer — CLI subcommands,
// the agent self-check, the host contract gate, the salvage rung, and the
// runner default — now derives the catalog from here.

// defaultRoot is the project-local drop-in root, conventionally first.
const defaultRoot = ".evolve/phases"

// registryRelPath locates the built-in phase registry inside a project.
const registryRelPath = "docs/architecture/phase-registry.json"

// RootsWithPolicy returns the phase-spec discovery roots using the given PathsConfig.
// A colon-separated cfg.PhaseRoots overrides; relative entries resolve against
// projectRoot; absolute entries are kept verbatim. Empty cfg ⇒ defaultRoot.
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

// Roots returns the phase-spec discovery roots for a project, loading
// phase roots from policy.json (PathsConfig.PhaseRoots) or falling back
// to the default project-local .evolve/phases. Replaced EVOLVE_PHASE_ROOTS
// env read (cycle-17).
func Roots(projectRoot string) []string {
	pol, _ := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	return RootsWithPolicy(projectRoot, pol.PathsConfig())
}

// MergedCatalog loads the built-in registry and overlays user phases from
// every discovery root. sources maps user phase name → discovery root
// (provenance). A missing/unreadable registry errors loudly — the caller
// decides how to degrade (the CLI and VerifyCatalogAware fall back to
// built-in-only resolution).
func MergedCatalog(projectRoot string) (Catalog, map[string]string, []string, error) {
	builtin, err := Load(filepath.Join(projectRoot, filepath.FromSlash(registryRelPath)))
	if err != nil {
		return Catalog{}, nil, nil, err
	}
	user, sources, warns := DiscoverUserSpecsFromRoots(Roots(projectRoot))
	merged, mWarns := builtin.Merge(user)
	return merged, sources, append(warns, mWarns...), nil
}
