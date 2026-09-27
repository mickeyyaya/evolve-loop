package bridge

import (
	"context"
	"runtime"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

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

// Probe satisfies core.Bridge: it enumerates the embedded CLI manifests and reports each CLI's tier via the LookPath seam.
func (e *Engine) Probe(_ context.Context) (core.BridgeProbe, error) {
	hasBinary := func(bin string) bool {
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
