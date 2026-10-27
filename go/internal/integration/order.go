package integration

import (
	"slices"
)

func OrderDeliverables(batch []Deliverable) []Deliverable {
	if len(batch) == 0 {
		return nil
	}
	items := make([]Deliverable, len(batch))
	for i, d := range batch {
		d.Deps = slices.Clone(d.Deps)
		items[i] = d
	}
	slices.SortFunc(items, compareDeliverables)

	unresolved := initUnresolved(items)
	emitted := make([]bool, len(items))
	result := make([]Deliverable, 0, len(items))

	for len(result) < len(items) {
		chosen := -1
		for i := 0; i < len(items); i++ {
			if !emitted[i] && len(unresolved[i]) == 0 {
				chosen = i
				break
			}
		}
		if chosen == -1 {
			for i := 0; i < len(items); i++ {
				if !emitted[i] {
					chosen = i
					break
				}
			}
		}
		emitted[chosen] = true
		c := items[chosen]
		c.Deps = slices.Clone(c.Deps)
		result = append(result, c)
		for i := 0; i < len(items); i++ {
			if !emitted[i] {
				delete(unresolved[i], c.ID)
			}
		}
	}

	return result
}

func initUnresolved(items []Deliverable) []map[string]struct{} {
	present := make(map[string]bool, len(items))
	for _, d := range items {
		present[d.ID] = true
	}
	unresolved := make([]map[string]struct{}, len(items))
	for i, d := range items {
		deps := make(map[string]struct{})
		for _, dep := range d.Deps {
			if dep != d.ID && present[dep] {
				deps[dep] = struct{}{}
			}
		}
		unresolved[i] = deps
	}
	return unresolved
}

func compareDeliverables(a, b Deliverable) int {
	if a.InboxRank != b.InboxRank {
		if a.InboxRank < b.InboxRank {
			return -1
		}
		return 1
	}
	if a.UnblocksCount != b.UnblocksCount {
		if a.UnblocksCount > b.UnblocksCount {
			return -1
		}
		return 1
	}
	if a.Risky != b.Risky {
		if !a.Risky {
			return -1
		}
		return 1
	}
	if a.VerificationCost != b.VerificationCost {
		if a.VerificationCost < b.VerificationCost {
			return -1
		}
		return 1
	}
	if !a.DeliveredAt.Equal(b.DeliveredAt) {
		if a.DeliveredAt.Before(b.DeliveredAt) {
			return -1
		}
		return 1
	}
	if a.ID < b.ID {
		return -1
	}
	if a.ID > b.ID {
		return 1
	}
	return 0
}
