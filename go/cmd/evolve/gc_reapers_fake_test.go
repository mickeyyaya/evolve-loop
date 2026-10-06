package main

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

func init() {
	gcInjectedReapers = &gcReapers{
		sessions: func(context.Context) swarm.OrphanReapReport { return swarm.OrphanReapReport{} },
		sockets:  func(context.Context) swarm.OrphanSocketReport { return swarm.OrphanSocketReport{} },
	}
}
