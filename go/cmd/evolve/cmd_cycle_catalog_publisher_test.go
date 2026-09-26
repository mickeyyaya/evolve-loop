package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// A compile-time caller proof: the production bridge adapter is a valid sink.
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

func TestWireOrchestrator_CatalogPublisherWired(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	d := wireOrchestratorDeps(root, evolveDir, io.Discard)
	if !d.Orchestrator.CatalogPublisherWired() {
		t.Fatal("RED (cycle-1429): the production composition root (wireOrchestratorDeps, cmd_cycle.go) does not wire core.WithCatalogPublisher, so the bridge's contract resolver stays bound to the cycle-START catalog snapshot (cmd_cycle.go:482 `catalog.Get`). A phase minted mid-cycle is spliced into o.catalog but NEVER into the resolver — CatalogResolver.Resolve misses it for the rest of the cycle and every dispatch falls back to the unresolved-agent path (the cycle-1424 600s artifact-timeout halt)")
	}
}

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
	if _, ok := spy.last.Resolve("never-minted-agent"); ok {
		t.Error("the published resolver resolves an unknown agent — over-broad widening, not a targeted catalog refresh")
	}
}
