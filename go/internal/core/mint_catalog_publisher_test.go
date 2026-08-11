// mint_catalog_publisher_test.go — cycle-1429 TDD contract, task
// `mint-catalog-live-refresh` (inbox pipeline-defect-infra-systemic, P0).
//
// THE DEFECT (cycle-1424 root cause, still open after #428/#429).
// The composition root binds the bridge's deliverable-contract resolver ONCE, at
// cycle start, over a SNAPSHOT of the phase catalog:
//
//	cmd/evolve/cmd_cycle.go:482
//	  br.SetContractResolver(phasecontract.NewCatalogResolver(catalog.Get))
//
// `catalog.Get` is a method value bound to that catalog VALUE, whose `byName`
// map is the pre-mint map. When the advisor mints a phase mid-cycle,
// registerMintedPhases (routing_dispatch.go:158-162) does
// `merged, _ := o.catalog.Merge(...); o.catalog = merged` — and Catalog.Merge
// returns a NEW Catalog over a NEW map (phasespec/discover.go:99-104). The
// orchestrator's own catalog therefore knows the minted phase, while the
// resolver the bridge injects prompts through is still reading the stale,
// pre-mint map. CatalogResolver.Resolve misses for the freshly-minted phase for
// the REST OF THAT CYCLE, so every contract-dependent consumer falls back to the
// unresolved-agent path. That is the cycle-1424 halt: `defect-disposition-ledger`
// was dispatched, the engine polled 600s for an artifact, and the phase never
// resolved a contract. #429 made that fallback SAFE (the footer now discloses the
// polled path); this test closes the miss itself so the fallback stops being
// load-bearing.
//
// THE CONTRACT (what Builder must implement).
//  1. `core.WithCatalogPublisher(fn func(phasespec.Catalog)) Option` — injects a
//     sink the orchestrator notifies whenever its live catalog CHANGES.
//  2. `registerMintedPhases` calls that sink AFTER the `o.catalog = merged`
//     splice, with the merged catalog, for every successfully-minted phase.
//  3. `(*Orchestrator).CatalogPublisherWired() bool` — the composition-root
//     reachability predicate (same idiom as CompositionFastPathWired), asserted
//     from cmd/evolve by TestWireOrchestrator_CatalogPublisherWired.
//
// RED today: WithCatalogPublisher / CatalogPublisherWired do not exist, so this
// file does not compile — the correct RED for a missing seam.
//
// Reachability probe (cycle-644 obligation): internal/core ALREADY imports both
// phasecontract (build_removal_check.go:33) and phasespec, so the pins below add
// no new import edge and cannot introduce an import cycle.
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

// TestRegisterMintedPhases_PublishesCatalogToLiveResolver is the cycle-1429
// acceptance for mint-catalog-live-refresh, reproducing the cycle-1424 shape
// EXACTLY: a resolver bound over the pre-mint catalog snapshot misses the minted
// phase; after the mint publishes, a resolver bound over the published catalog
// hits and yields the spec-derived contract.
//
// The two resolvers are the point. `stale` models today's cmd_cycle.go:482
// binding (a method value over the pre-mint catalog value) and must STILL miss —
// proving the publisher is what carries the new catalog across, not some
// accidental aliasing. `live` models the post-fix binding and must HIT.
func TestRegisterMintedPhases_PublishesCatalogToLiveResolver(t *testing.T) {
	t.Parallel()

	preMint := phasespec.Catalog{}
	var published phasespec.Catalog
	o := publisherOrchestrator(t, &published)

	// The stale resolver: bound ONCE over the pre-mint catalog value, exactly as
	// the composition root binds it today.
	stale := phasecontract.NewCatalogResolver(preMint.Get)
	// The live resolver: bound over whatever the publisher last delivered.
	live := phasecontract.NewCatalogResolver(func(name string) (phasespec.PhaseSpec, bool) {
		return published.Get(name)
	})

	const minted = "defect-disposition-ledger" // the cycle-1424 phase, by name
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
	// The contract must be the SPEC-DERIVED one for this exact phase. The TDD
	// draft asserted AgentName==minted; the repo-wide convention is
	// PhaseSpec.AgentName() = "evolve-"+Name when the spec declares no explicit
	// agent (phasespec/phasespec.go:210-215), which every persona lookup depends
	// on. Corrected to the real convention and STRENGTHENED with the Phase
	// identity assertion, so the property ("names this phase, not a guess") is
	// pinned harder, not weakened.
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

// TestRegisterMintedPhases_PublishedCatalogStillMissesUnknownAgent is the
// NEGATIVE half (adversarial-testing SKILL §6): the fix must close the miss for
// the MINTED name only. A resolver that starts resolving arbitrary unknown agent
// names would synthesize a bogus contract for every non-phase bridge caller and
// re-open a worse class of the same bug.
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

// TestRegisterMintedPhases_RejectedMintPublishesNothing pins the failure edge: a
// registrar rejection (out-of-envelope, driverless profile, bad name) must not
// publish a catalog at all. Publishing on a rejected mint would hand the live
// resolver a phase that has no runner — a routable ghost.
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

// TestRegisterMintedPhases_NoPublisherIsNoop is the back-compat edge: every
// orchestrator built WITHOUT the option (every existing test, cmd_campaign, the
// subagent harness) must keep working — a nil publisher is a no-op, never a
// panic.
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
