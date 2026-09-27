package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// publisherOrchestrator builds an orchestrator with the built-in spine only — no
// minted phase in catalog/order/runners — plus a fake minter and a catalog
// publisher that records the last published catalog into *sink.
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

// stale models the pre-fix binding (a method value over the pre-mint catalog
// value) and must still miss, proving the publisher — not aliasing — carries
// the new catalog across; live models the post-fix binding and must hit.
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

	// 1. The publisher fired at all, with a catalog that knows the minted phase.
	if _, ok := published.Get(minted); !ok {
		t.Fatalf("registerMintedPhases did not publish a catalog containing %q — the live contract resolver stays bound to the pre-mint snapshot for the rest of the cycle (cycle-1424: 600s artifact-timeout on a naked dispatch)", minted)
	}

	// 2. The live resolver now RESOLVES the minted phase's deliverable contract,
	//    same cycle, no disk refresh, no cycle-start wait.
	c, ok := live.Resolve(minted)
	if !ok {
		t.Fatalf("live CatalogResolver still MISSES %q immediately after the mint — the contract-injection path falls back to the unresolved-agent branch (adapters/bridge/bridge.go injectContract)", minted)
	}
	// The contract must be the spec-derived one for this phase:
	// PhaseSpec.AgentName() = "evolve-"+Name when the spec declares no explicit
	// agent, which every persona lookup depends on.
	if c.Phase != minted {
		t.Errorf("resolved contract Phase=%q, want %q (spec-derived contract must name the minted phase)", c.Phase, minted)
	}
	if want := "evolve-" + minted; c.AgentName != want {
		t.Errorf("resolved contract AgentName=%q, want %q (phasespec.AgentName convention)", c.AgentName, want)
	}
	if want := minted + "-report.md"; c.ArtifactName != want {
		t.Errorf("resolved contract ArtifactName=%q, want %q (phasecontract.artifactNameFromSpec convention fallback)", c.ArtifactName, want)
	}

	// 3. The stale binding must still miss. If this ever HITS, the two resolvers
	//    are aliasing the same map and the assertion above is vacuous.
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

	o := mintOrchestrator(t, fakeMinter{}) // no WithCatalogPublisher
	o.registerMintedPhases(mintPlan("minted-reviewer"))
	if _, ok := o.runners[Phase("minted-reviewer")]; !ok {
		t.Error("mint must still register the runner when no catalog publisher is wired")
	}
	if o.CatalogPublisherWired() {
		t.Error("CatalogPublisherWired must report false when no publisher was injected")
	}
}
