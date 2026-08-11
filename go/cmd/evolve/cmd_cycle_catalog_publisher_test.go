// cmd_cycle_catalog_publisher_test.go — cycle-1429 TDD contract, the WIRING
// half of task `mint-catalog-live-refresh`.
//
// A core seam whose only caller is a test is dead code. internal/core's
// mint_catalog_publisher_test.go proves the orchestrator PUBLISHES its catalog
// after a mid-cycle mint; this file proves the PRODUCTION composition root
// (wireOrchestratorDeps, cmd_cycle.go) actually subscribes that publication to
// the bridge's contract resolver. Without this half, cycle-1424 reproduces
// verbatim with a green core test.
//
// Pattern copied verbatim from TestWireOrchestrator_CompositionFastPathWired
// (cmd_cycle_composition_test.go, cycle-804): drive the real composition root
// against a real temp-dir project root, assert an exported orchestrator
// predicate — no fakes, no injected doubles.
//
// THE CONTRACT (what Builder must implement in cmd/evolve):
//
//  1. `contractResolverSink` — the narrow bridge-side seam:
//     `interface{ SetContractResolver(phasecontract.Resolver) }`.
//     *bridge.Adapter already satisfies it (bridge.go:174); the compile-time
//     assertion below is the reachability proof that the PRODUCTION type, not a
//     test double, is what the sink accepts.
//  2. `catalogPublisher(sink contractResolverSink) func(phasespec.Catalog)` —
//     returns the closure that re-binds a fresh
//     `phasecontract.NewCatalogResolver(c.Get)` onto the sink for each published
//     catalog. Named + extracted (not an inline literal) so it is testable.
//  3. wireOrchestratorDeps appends `core.WithCatalogPublisher(catalogPublisher(br))`
//     to opts, beside the existing core.WithRegistrar(...) mint wiring.
//
// RED today: none of the three exist, so this file does not compile.
//
// Reachability probe (cycle-644 obligation): package main already imports
// adapters/bridge, phasecontract, phasespec and core (cmd_cycle.go:21-56), so
// every pin below rides an existing import edge — no new edge, no cycle.
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// The production bridge adapter MUST be an acceptable sink. This is the caller
// proof: it fails to compile the moment catalogPublisher is given a signature
// only a test double can satisfy.
var _ contractResolverSink = (*bridge.Adapter)(nil)

// resolverSinkSpy captures what catalogPublisher hands the bridge.
type resolverSinkSpy struct {
	last  phasecontract.Resolver
	calls int
}

func (s *resolverSinkSpy) SetContractResolver(r phasecontract.Resolver) {
	s.last = r
	s.calls++
}

// TestWireOrchestrator_CatalogPublisherWired is the cycle-1429 composition-root
// assertion: the real wireOrchestratorDeps binds a catalog publisher, so a
// mid-cycle mint reaches the bridge's contract resolver.
func TestWireOrchestrator_CatalogPublisherWired(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	d := wireOrchestratorDeps(root, evolveDir)
	if !d.Orchestrator.CatalogPublisherWired() {
		t.Fatal("RED (cycle-1429): the production composition root (wireOrchestratorDeps, cmd_cycle.go) does not wire core.WithCatalogPublisher, so the bridge's contract resolver stays bound to the cycle-START catalog snapshot (cmd_cycle.go:482 `catalog.Get`). A phase minted mid-cycle is spliced into o.catalog but NEVER into the resolver — CatalogResolver.Resolve misses it for the rest of the cycle and every dispatch falls back to the unresolved-agent path (the cycle-1424 600s artifact-timeout halt)")
	}
}

// TestCatalogPublisher_RebindsResolverOnEachPublish proves the published closure
// does the real work: each published catalog produces a resolver that RESOLVES
// that catalog's phases. A closure that captured a snapshot (the original bug,
// relocated) fails the second publish.
func TestCatalogPublisher_RebindsResolverOnEachPublish(t *testing.T) {
	spy := &resolverSinkSpy{}
	pub := catalogPublisher(spy)
	if pub == nil {
		t.Fatal("catalogPublisher returned nil — nothing would ever reach the bridge")
	}

	first, _ := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{Name: "defect-disposition-ledger", Optional: true, Description: "d", WhenToUse: "w"},
	})
	pub(first)
	if spy.calls != 1 {
		t.Fatalf("publish #1: sink called %d time(s), want 1", spy.calls)
	}
	if _, ok := spy.last.Resolve("defect-disposition-ledger"); !ok {
		t.Error("publish #1: the resolver handed to the bridge does not resolve the published phase — the mint stays invisible to contract injection")
	}

	// A SECOND mint in the same cycle must be visible too (Steps 11/12 may mint
	// more than once); a resolver bound to the first publish would miss it.
	second, _ := first.Merge([]phasespec.PhaseSpec{
		{Name: "second-minted-phase", Optional: true, Description: "d", WhenToUse: "w"},
	})
	pub(second)
	if spy.calls != 2 {
		t.Fatalf("publish #2: sink called %d time(s), want 2", spy.calls)
	}
	if _, ok := spy.last.Resolve("second-minted-phase"); !ok {
		t.Error("publish #2: the resolver misses the second mint — catalogPublisher captured a snapshot instead of re-binding per publish")
	}
	if _, ok := spy.last.Resolve("defect-disposition-ledger"); !ok {
		t.Error("publish #2: the resolver lost the FIRST mint — each publish must carry the merged catalog, not replace it")
	}
	// Negative: still no contract for an agent nobody minted.
	if _, ok := spy.last.Resolve("never-minted-agent"); ok {
		t.Error("the published resolver resolves an unknown agent — over-broad widening, not a targeted catalog refresh")
	}
}
