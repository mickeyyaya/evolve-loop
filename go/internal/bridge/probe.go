package bridge

import (
	"context"
	"runtime"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// resolveTier computes a CLI's capability tier from its manifest and a
// binary-presence predicate — the Go port of probe_resolve_tier:
//
//	stub        → none
//	binary absent → none
//	declared tier deps all present → declared (full/hybrid), else degraded
//
// An empty/"none" default_tier is treated as "full" (matching the bash).
func resolveTier(m Manifest, hasBinary func(string) bool) string {
	if m.Stub {
		return "none"
	}
	if !hasBinary(m.Binary) {
		return "none"
	}
	declared := m.DefaultTier
	if declared == "" || declared == "none" {
		declared = "full"
	}
	for _, dep := range m.TierDependencies[declared] {
		if !hasBinary(dep) {
			return "degraded"
		}
	}
	return declared
}

// Probe satisfies core.Bridge: enumerates the embedded CLI manifests and
// reports each CLI's tier via the LookPath seam. The Go port of
// `bridge probe` (probe_all) reduced to the {os, cli→tier} shape the
// core.BridgeProbe contract carries.
func (e *Engine) Probe(_ context.Context) (core.BridgeProbe, error) {
	hasBinary := func(bin string) bool {
		if bin == "" {
			return false
		}
		_, err := e.deps.LookPath(bin)
		return err == nil
	}
	out := core.BridgeProbe{
		Version: runtime.GOOS,
		CLIs:    map[string]string{},
	}
	for _, cli := range ManifestNames() {
		m, err := LoadManifest(cli)
		if err != nil {
			out.CLIs[cli] = "none"
			continue
		}
		out.CLIs[cli] = resolveTier(m, hasBinary)
	}
	return out, nil
}
