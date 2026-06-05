package main

import (
	"os"
	"path/filepath"
	"strings"
)

// phaseRootsEnv is the single seam for phase-plugin distribution (ADR-0038):
// a colon-separated, ordered list of phase discovery roots. A plugin bundle
// that ships phases registers by appending its directory here — evolve never
// scans CLI-private plugin caches. Mirrors the EVOLVE_KB_SEARCH_PATHS idiom.
const phaseRootsEnv = "EVOLVE_PHASE_ROOTS"

// defaultPhaseRoot is the project-local drop-in root, conventionally first so
// the local project can shadow a plugin phase by re-declaring its name.
const defaultPhaseRoot = ".evolve/phases"

// phaseRoots returns the ordered phase discovery roots, relative entries
// resolved against projectRoot (absolute entries pass through; empties
// dropped). Unset/empty env → just the default root, byte-identical to the
// single-root behavior before ADR-0038.
func phaseRoots(projectRoot string) []string {
	raw := os.Getenv(phaseRootsEnv)
	if strings.TrimSpace(raw) == "" {
		raw = defaultPhaseRoot
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
