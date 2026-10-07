package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

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

// TestWireOrchestrator_ModelCatalogLookupWired asserts against the real
// composition root rather than by source inspection: an AST scan cannot
// tell WithModelCatalogLookup(resolver) from WithModelCatalogLookup(nil),
// and nil is precisely the dead-gate state.
func TestWireOrchestrator_ModelCatalogLookupWired(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	d := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{})
	if !d.Orchestrator.ModelCatalogLookupWired() {
		t.Fatal("production composition root (wireOrchestratorDeps) does not wire core.WithModelCatalogLookup — " +
			"router.ClampPlanModelRouting short-circuits on a nil lookup, so the catalog-resolvability gate is a " +
			"silent no-op in every real cycle and the advisor can route a phase to a (cli,tier) that resolves to nothing")
	}
}
