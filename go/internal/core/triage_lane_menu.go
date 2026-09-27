package core

import "github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"

type LaneMenuFn func(inboxRoot string, items []inboxbatch.Item) []inboxbatch.Item

func WithLaneMenu(fn LaneMenuFn) Option {
	return func(o *Orchestrator) { o.laneMenu = fn }
}

func (o *Orchestrator) LaneMenuWired() bool { return o.laneMenu != nil }

func (o *Orchestrator) laneMenuOrRoutingOnly() LaneMenuFn {
	if o.laneMenu == nil {
		return routingOnlyLaneMenu
	}
	return o.laneMenu
}

func routingOnlyLaneMenu(_ string, items []inboxbatch.Item) []inboxbatch.Item {
	laneRoutable, _, _ := inboxbatch.PartitionConsole(items, nil)
	return laneRoutable
}
