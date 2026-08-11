package main

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// contractResolverSink is the narrow bridge-side seam the catalog publisher
// writes to — *bridge.Adapter satisfies it (bridge.go:174). Declared as an
// interface (not the concrete adapter) so the publisher is testable without a
// live bridge, and narrow so it cannot grow into a second bridge handle.
type contractResolverSink interface {
	SetContractResolver(phasecontract.Resolver)
}

// catalogPublisher returns the closure wireOrchestratorDeps hands to
// core.WithCatalogPublisher: for EVERY published catalog it binds a FRESH
// phasecontract.NewCatalogResolver over that catalog and installs it on the
// bridge.
//
// Re-binding per publish (rather than capturing one catalog) is the whole point:
// Catalog.Merge returns a new value over a new map, so a resolver bound at cycle
// start — or at the first mint — cannot see a later one. A second mint in the
// same cycle must be resolvable too.
func catalogPublisher(sink contractResolverSink) func(phasespec.Catalog) {
	if sink == nil {
		return nil
	}
	return func(c phasespec.Catalog) {
		sink.SetContractResolver(phasecontract.NewCatalogResolver(c.Get))
	}
}
