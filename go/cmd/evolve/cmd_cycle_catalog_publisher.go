package main

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// contractResolverSink is the narrow bridge seam the catalog publisher writes
// to; *bridge.Adapter satisfies it.
type contractResolverSink interface {
	SetContractResolver(phasecontract.Resolver)
}

// catalogPublisher installs a fresh contract resolver on the bridge for every
// published catalog: Catalog.Merge returns a new map, so a resolver bound
// earlier cannot see a later mint.
func catalogPublisher(sink contractResolverSink) func(phasespec.Catalog) {
	if sink == nil {
		return nil
	}
	return func(c phasespec.Catalog) {
		sink.SetContractResolver(phasecontract.NewCatalogResolver(c.Get))
	}
}
