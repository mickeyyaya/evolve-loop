package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func publisherOrchestrator(t *testing.T, sink *phasespec.Catalog) *Orchestrator {
	t.Helper()
	cfg := config.RoutingConfig{
		Stage: config.StageEnforce,
		Order: []string{"scout", "build", "audit", "ship"},
	}
	return NewOrchestrator(nil, nil, map[Phase]PhaseRunner{
		PhaseBuild: &fakeRunner{name: "build", verdict: VerdictPASS},
	},
		WithCatalog(phasespec.Catalog{}),
		WithRouting(cfg, nil),
		WithRegistrar(fakeMinter{}),
		WithCatalogPublisher(func(c phasespec.Catalog) { *sink = c }),
	)
}

func TestRegisterMintedPhases_PublishesCatalogToLiveResolver(t *testing.T) {
	t.Parallel()

	preMint := phasespec.Catalog{}
	var published phasespec.Catalog
	o := publisherOrchestrator(t, &published)

	stale := phasecontract.NewCatalogResolver(preMint.Get)
	live := phasecontract.NewCatalogResolver(func(name string) (phasespec.PhaseSpec, bool) {
		return published.Get(name)
	})

	const minted = "defect-disposition-ledger"
	if _, ok := live.Resolve(minted); ok {
		t.Fatalf("pre-mint: resolver must MISS %q before it is minted (the test would prove nothing otherwise)", minted)
	}

	o.registerMintedPhases(mintPlan(minted))

	if _, ok := published.Get(minted); !ok {
		t.Fatalf("registerMintedPhases did not publish a catalog containing %q — the live contract resolver stays bound to the pre-mint snapshot for the rest of the cycle (cycle-1424: 600s artifact-timeout on a naked dispatch)", minted)
	}

	c, ok := live.Resolve(minted)
	if !ok {
		t.Fatalf("live CatalogResolver still MISSES %q immediately after the mint — the contract-injection path falls back to the unresolved-agent branch (adapters/bridge/bridge.go injectContract)", minted)
	}
	if c.Phase != minted {
		t.Errorf("resolved contract Phase=%q, want %q (spec-derived contract must name the minted phase)", c.Phase, minted)
	}
	if want := "evolve-" + minted; c.AgentName != want {
		t.Errorf("resolved contract AgentName=%q, want %q (phasespec.AgentName convention)", c.AgentName, want)
	}
	if want := minted + "-report.md"; c.ArtifactName != want {
		t.Errorf("resolved contract ArtifactName=%q, want %q (phasecontract.artifactNameFromSpec convention fallback)", c.ArtifactName, want)
	}

	if _, ok := stale.Resolve(minted); ok {
		t.Errorf("the pre-mint snapshot resolver unexpectedly resolves %q — Catalog.Merge no longer copies its map, and this test's proof is vacuous; re-derive the contract before touching the fix", minted)
	}
}

func TestRegisterMintedPhases_PublishedCatalogStillMissesUnknownAgent(t *testing.T) {
	t.Parallel()

	var published phasespec.Catalog
	o := publisherOrchestrator(t, &published)
	o.registerMintedPhases(mintPlan("minted-reviewer"))

	live := phasecontract.NewCatalogResolver(func(name string) (phasespec.PhaseSpec, bool) {
		return published.Get(name)
	})
	if _, ok := live.Resolve("minted-reviewer"); !ok {
		t.Fatal("setup: the minted phase must resolve, else the negative below proves nothing")
	}
	for _, unknown := range []string{"never-minted-agent", "", "simplifier"} {
		if c, ok := live.Resolve(unknown); ok {
			t.Errorf("resolver must MISS unknown agent %q after a mint, got contract %+v — over-broad catalog widening", unknown, c)
		}
	}
}

func TestRegisterMintedPhases_RejectedMintPublishesNothing(t *testing.T) {
	t.Parallel()

	cfg := config.RoutingConfig{Stage: config.StageEnforce, Order: []string{"scout", "build", "audit", "ship"}}
	published := 0
	o := NewOrchestrator(nil, nil, map[Phase]PhaseRunner{
		PhaseBuild: &fakeRunner{name: "build", verdict: VerdictPASS},
	},
		WithCatalog(phasespec.Catalog{}),
		WithRouting(cfg, nil),
		WithRegistrar(fakeMinter{reject: map[string]bool{"bad-phase": true}}),
		WithCatalogPublisher(func(phasespec.Catalog) { published++ }),
	)
	o.registerMintedPhases(mintPlan("bad-phase"))
	if published != 0 {
		t.Errorf("catalog published %d time(s) for a REJECTED mint, want 0 — a rejected phase must never reach the live contract resolver", published)
	}
}

func TestRegisterMintedPhases_NoPublisherIsNoop(t *testing.T) {
	t.Parallel()

	o := mintOrchestrator(t, fakeMinter{})
	o.registerMintedPhases(mintPlan("minted-reviewer"))
	if _, ok := o.runners[Phase("minted-reviewer")]; !ok {
		t.Error("mint must still register the runner when no catalog publisher is wired")
	}
	if o.CatalogPublisherWired() {
		t.Error("CatalogPublisherWired must report false when no publisher was injected")
	}
}
