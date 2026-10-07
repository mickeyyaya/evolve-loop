package inboxrank

import (
	"fmt"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type Inputs struct {
	Config     policy.InboxPriorityConfig
	Now        time.Time
	Recurrence map[string]int
}

func (in Inputs) Order(items, queue []inboxbatch.Item) []Ranked {
	counts := func(it inboxbatch.Item) int { return in.Recurrence[it.ID] }
	return Order(items, in.Config, Context{Now: in.Now, Queue: queue, Recurrence: counts})
}

func (b Breakdown) topFactor() Factor {
	top := b.Terms[0]
	for _, term := range b.Terms[1:] {
		if term.Contribution > top.Contribution {
			top = term
		}
	}
	return top.Factor
}

func Sequence(ranked []Ranked) func([]inboxbatch.Item) []inboxbatch.Item {
	position := make(map[inboxbatch.ItemKey]int, len(ranked))
	for i, r := range ranked {
		if _, seen := position[r.Item.Key()]; !seen {
			position[r.Item.Key()] = i
		}
	}
	return func(items []inboxbatch.Item) []inboxbatch.Item {
		at := func(it inboxbatch.Item) int {
			if p, ok := position[it.Key()]; ok {
				return p
			}
			return len(ranked)
		}
		ordered := slices.Clone(items)
		slices.SortStableFunc(ordered, func(a, b inboxbatch.Item) int { return at(a) - at(b) })
		return ordered
	}
}

func Labels(ranked []Ranked) func(inboxbatch.Item) string {
	labels := make(map[inboxbatch.ItemKey]string, len(ranked))
	for _, r := range ranked {
		labels[r.Item.Key()] = fmt.Sprintf("score %.4f, top factor %s", r.Breakdown.Score, r.Breakdown.topFactor())
	}
	return func(it inboxbatch.Item) string {
		if label, ok := labels[it.Key()]; ok {
			return label
		}
		return "not ranked"
	}
}
