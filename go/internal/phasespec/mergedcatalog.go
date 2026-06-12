package phasespec

import (
	"os"
	"path/filepath"
	"strings"
)

// mergedcatalog.go — the ONE merged-catalog loader (built-in registry +
// user phase overlays). Previously cmd/evolve-local, which left non-cmd
// consumers (the runner's reconcile-on-timeout default) resolving phases
// under a second, builtin-only policy. Every consumer — CLI subcommands,
// the agent self-check, the host contract gate, the salvage rung, and the
// runner default — now derives the catalog from here.

// rootsEnv is the single seam for phase-plugin distribution (ADR-0038):
// a colon-separated list of discovery roots overlaid in order.
const rootsEnv = "EVOLVE_PHASE_ROOTS"

// defaultRoot is the project-local drop-in root, conventionally first.
const defaultRoot = ".evolve/phases"

// registryRelPath locates the built-in phase registry inside a project.
const registryRelPath = "docs/architecture/phase-registry.json"

// Roots returns the phase-spec discovery roots for a project: EVOLVE_PHASE_ROOTS
// (colon-separated; relative entries resolve against projectRoot) or the
// default project-local .evolve/phases.
func Roots(projectRoot string) []string {
	raw := os.Getenv(rootsEnv)
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
