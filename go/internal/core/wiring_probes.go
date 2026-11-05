package core

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func (o *Orchestrator) RoutingStrategy() router.RoutingStrategy { return o.strategy }

func (o *Orchestrator) RoutingConfig() config.RoutingConfig { return o.cfg }

func (o *Orchestrator) ShipFloor() []string { return o.resolvedShipFloor() }

func (o *Orchestrator) ContractGateReviewer() DeliverableReviewer {
	var gate DeliverableReviewer
	reviewerChainHas(o.reviewer, func(r DeliverableReviewer) bool {
		if g, ok := r.(declaredDeliverablesGate); ok && g.VerifiesDeclaredDeliverables() {
			gate = r
		}
		return gate != nil
	})
	return gate
}
