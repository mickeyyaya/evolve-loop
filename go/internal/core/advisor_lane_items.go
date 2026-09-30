package core

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func (o *Orchestrator) laneItemsForAdvisor(projectRoot, workspace string) []router.LaneItem {
	ids := LaneScopeIDs(workspace)
	items := make([]router.LaneItem, 0, len(ids))
	for _, id := range ids {
		items = append(items, o.laneItem(projectRoot, id))
	}
	return items
}

func (o *Orchestrator) laneItem(projectRoot, id string) router.LaneItem {
	path := ""
	if o.scopePathFor != nil {
		path = o.scopePathFor(projectRoot, id)
	}
	if path == "" {
		return router.LaneItem{ID: id, Unresolved: "inbox record not resolved"}
	}
	item, _, err := inboxbatch.LoadFile(path)
	if err != nil {
		return router.LaneItem{ID: id, Unresolved: fmt.Sprintf("inbox record unreadable at %s: %v", path, err)}
	}
	return router.LaneItem{
		ID:              id,
		Kind:            item.Kind,
		DeliverableKind: resolveDeliverableKind(item.DeliverableKind, projectDomainDefault(projectRoot)),
		Acceptance:      item.Acceptance,
	}
}
