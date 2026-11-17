package inboxcurator

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func HistoryFloor(open []inboxbatch.Item) (time.Time, bool) {
	var floor time.Time
	for _, it := range open {
		if at := it.FiledAt(); !at.IsZero() && (floor.IsZero() || at.Before(floor)) {
			floor = at
		}
	}
	return floor, !floor.IsZero()
}

func ClosedOnMain(history []Change, open []inboxbatch.Item) []Proposal {
	var out []Proposal
	for _, it := range open {
		if it.FiledAt().IsZero() {
			continue
		}
		for _, c := range history {
			if closedBy(c, it) {
				out = append(out, Proposal{
					Kind: ProposeConsume, ID: it.ID, Path: it.Path, Trigger: TriggerHistory, Deterministic: true,
					Reason: "main holds Closes-Inbox for this item (" + c.Commit + ")", Evidence: []string{"commit " + c.Commit},
				})
				break
			}
		}
	}
	return out
}
