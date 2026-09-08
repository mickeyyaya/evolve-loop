//go:build e2e

// Tier 2 — LIVE model-tier matrix. For each CLI, fire a single cheap bridge
// launch at EACH supported tier's concrete model and assert the call succeeds
// and writes an artifact. This catches model-deprecation / tier-resolution
// drift (e.g. a provider 400-rejecting a model the manifest still advertises —
// the cycle-142 codex ChatGPT-safe-model class of bug) that the cheapest-tier-
// only T0/T1 would miss. Gate: EVOLVE_E2E_LIVE_MATRIX=1.
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
		// codex: a PROJECTION of the family manifest, never a copy (this table sat
		// three model generations stale until 2026-09-09).
		"codex": codexTierModels(),
		"agy":   {"fast": "gemini-3.5-flash", "balanced": "gemini-3.5-flash", "deep": "gemini-3.5-flash"},
	}
}

// codexTierModels reads the codex family tier table through the headless
// manifest (codex.json points at codex-tmux.json via model_tier_map_from — the
// single source every codex reader resolves through). A load failure panics — loud, never a silent nil that
// would launch every case without -m.
func codexTierModels() map[string]string {
	m, err := bridge.LoadManifest("codex") // the headless manifest the matrix drives; its model_tier_map_from pointer yields the family table
	if err != nil {
		// e2e-only binary: an embedded manifest that fails to load is a broken
		// build, and a silent nil here would launch every codex case with no -m
		// (the CLI default) while the test still claimed to exercise a tier.
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
