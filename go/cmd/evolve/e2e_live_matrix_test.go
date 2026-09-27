//go:build e2e

package main

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"testing"
	"time"
)

// liveTierModels maps each CLI to the concrete model per abstract tier. A
// function, not a package var: the codex row loads a manifest, and init-time
// I/O (or its panic) must not fire before liveGate's SKIP. agy
// pins all tiers to one model (per its manifest); ollama is omitted (host model
// varies — exercise it via EVOLVE_E2E_LIVE_MODEL_OLLAMA + T0).
func liveTierModels() map[string]map[string]string {
	return map[string]map[string]string{
		"claude-p": {"fast": "haiku", "balanced": "sonnet", "deep": "opus"},
		"codex":    codexTierModels(), // a projection of the family manifest, never a copy
		"agy":      {"fast": "gemini-3.5-flash", "balanced": "gemini-3.5-flash", "deep": "gemini-3.5-flash"},
	}
}

// codexTierModels reads the codex family tier table through the headless
// manifest (codex.json points at codex-tmux.json via model_tier_map_from — the
// single source every codex reader resolves through). A load failure panics — loud, never a silent nil that
// would launch every case without -m.
func codexTierModels() map[string]string {
	m, err := bridge.LoadManifest("codex")
	if err != nil {
		panic("e2e: codex family manifest unavailable: " + err.Error())
	}
	if len(m.ModelTierMap) == 0 {
		panic("e2e: codex family manifest declares no model_tier_map")
	}
	return m.ModelTierMap
}

func TestE2ELiveModelTierMatrix(t *testing.T) {
	liveGate(t, "EVOLVE_E2E_LIVE_MATRIX")
	repoRoot := mustRepoRoot(t)
	evolveBin := buildBinary(t, t.TempDir(), "evolve", "./cmd/evolve", repoRoot)

	for _, cli := range liveHeadlessCLIs {
		cli := cli
		tiers := liveTierModels()[cli.Driver]
		t.Run(cli.Driver, func(t *testing.T) {
			if ok, why := liveCLIAvailable(cli); !ok {
				t.Skip(why)
			}
			seen := map[string]bool{} // skip duplicate models (agy maps all tiers to one)
			for _, tier := range []string{"fast", "balanced", "deep"} {
				model := tiers[tier]
				if model == "" || seen[model] {
					continue
				}
				seen[model] = true
				t.Run(tier+"_"+model, func(t *testing.T) {
					if ok, _ := liveBudgetRemaining(); !ok {
						t.Skip("live budget exhausted")
					}
					size, out, err := liveBridgeLaunch(t, evolveBin, cli.Driver, model,
						envDurationSeconds("EVOLVE_E2E_LIVE_TIMEOUT_S", 3*time.Minute))
					if err != nil {
						if isTransient(out, err) {
							t.Skipf("%s/%s transient (quarantined):\nerr=%v\n%s", cli.Driver, model, err, lastN(out, 600))
						}
						t.Fatalf("%s tier %s model %s REJECTED (deprecation/contract?):\nerr=%v\n%s",
							cli.Driver, tier, model, err, lastN(out, 1200))
					}
					if size <= 0 {
						t.Errorf("%s/%s: no artifact written", cli.Driver, model)
					}
					t.Logf("[live-matrix] %s tier=%s model=%s OK (%d bytes)", cli.Driver, tier, model, size)
				})
			}
		})
	}
}
