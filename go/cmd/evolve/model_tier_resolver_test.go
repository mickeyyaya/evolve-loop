package main

// model_tier_resolver_test.go — the model-resolvability gate and its wiring.
//
// router.ClampPlanModelRouting clears an advisor-proposed {cli,tier} that
// cannot resolve to a model, fed by an injected lookup so router stays a leaf
// (ADR-0069's import-cycle lesson). core.WithModelCatalogLookup is that seam:
// defined, documented, unit-tested — and never called from the composition
// root, so o.modelCatalogLookup was nil in every real cycle and the gate was
// dead. TestWireOrchestrator_ModelCatalogLookupWired is the regression guard,
// asserted through the REAL composition root (mirroring
// TestWireOrchestrator_CompositionFastPathWired) rather than by source
// inspection: an AST scan cannot tell WithModelCatalogLookup(resolver) from
// WithModelCatalogLookup(nil), and nil is precisely the dead-gate state.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

// TestResolveModelTier_ResolvesEveryCanonicalTierForEveryFamily pins that the
// gate cannot OVER-clamp: every shipped family resolves every canonical tier,
// so no legitimate advisor proposal is cleared to the profile default.
//
// This is why the resolver goes through bridge.LoadManifest rather than
// modelcatalog.Catalog.Lookup — the cached catalog reports ok=false for any CLI
// stuck on source:"detect" (codex today), which would silently strip codex from
// every advisor plan even though dispatch resolves it fine.
func TestResolveModelTier_ResolvesEveryCanonicalTierForEveryFamily(t *testing.T) {
	for _, cli := range []string{"claude", "codex", "agy", "ollama"} {
		for _, tier := range modelcatalog.CanonicalTiers {
			t.Run(cli+"/"+tier, func(t *testing.T) {
				model, ok := resolveModelTier(cli, tier)
				if !ok || model == "" {
					t.Errorf("resolveModelTier(%q, %q) = (%q, %v), want a concrete model — the routing clamp treats an "+
						"unresolvable pairing as a breach and clears the advisor's {cli,tier} to the profile default, so a "+
						"false negative here silently disables model routing for that family", cli, tier, model, ok)
				}
			})
		}
	}
}

// TestResolveModelTier_RejectsUnresolvablePairings (negative / anti-degenerate):
// a resolver that returned ok=true unconditionally would pass the test above
// while leaving the gate just as dead.
func TestResolveModelTier_RejectsUnresolvablePairings(t *testing.T) {
	for _, tc := range []struct {
		name, cli, tier string
	}{
		{"unknown-cli", "nosuchcli", "deep"},
		{"unknown-tier", "claude", "nosuchtier"},
		{"empty-cli", "", "deep"},
		{"empty-tier", "claude", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if model, ok := resolveModelTier(tc.cli, tc.tier); ok {
				t.Errorf("resolveModelTier(%q, %q) = (%q, true), want ok=false — an unresolvable pairing must be caught, "+
					"otherwise the gate accepts anything and the advisor can route a phase to a model that does not exist",
					tc.cli, tc.tier, model)
			}
		})
	}
}

// TestWireOrchestrator_ModelCatalogLookupWired is the WIRING proof: the option
// must reach a real Orchestrator built by the production composition root, with
// a non-nil resolver. A gate that isn't wired isn't a gate.
func TestWireOrchestrator_ModelCatalogLookupWired(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	d := wireOrchestratorDeps(root, evolveDir)
	if !d.Orchestrator.ModelCatalogLookupWired() {
		t.Fatal("production composition root (wireOrchestratorDeps) does not wire core.WithModelCatalogLookup — " +
			"router.ClampPlanModelRouting short-circuits on a nil lookup, so the catalog-resolvability gate is a " +
			"silent no-op in every real cycle and the advisor can route a phase to a (cli,tier) that resolves to nothing")
	}
}
