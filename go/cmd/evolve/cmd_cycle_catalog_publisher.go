package main

import (
	"fmt"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
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
func catalogPublisher(sink contractResolverSink, router *cliroute.Router) func(phasespec.Catalog) {
	if sink == nil {
		return nil
	}
	return func(c phasespec.Catalog) {
		sink.SetContractResolver(phasecontract.NewCatalogResolver(c.Get))
		recompileRouter(router, c, os.Stderr)
	}
}

func recompileRouter(router *cliroute.Router, c phasespec.Catalog, warn io.Writer) {
	if router == nil {
		return
	}
	if err := router.Recompile(c); err != nil {
		fmt.Fprintf(warn, "[cli-routing] WARN a minted phase left the routing table unchanged: %v\n", err)
	}
}
