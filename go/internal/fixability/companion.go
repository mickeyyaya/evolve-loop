package fixability

import (
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const ReasonConsoleCompanionPending = "console_companion_pending"

type Item struct {
	ID         string
	Deps       []string
	ConnectsTo []string
}

func Companions(laneID string, open []Item) []string {
	prefixes := []string{laneID + "-"}
	if stem, ok := strings.CutSuffix(laneID, "-lane"); ok {
		prefixes = append(prefixes, stem+"-")
	}
	var out []string
	for _, it := range open {
		if it.ID != laneID && isCompanion(it, laneID, prefixes) {
			out = append(out, it.ID)
		}
	}
	return out
}

func isCompanion(it Item, laneID string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(it.ID, p) {
			return true
		}
	}
	return slices.Contains(it.Deps, laneID) || slices.Contains(it.ConnectsTo, laneID)
}

func OpenConsole(items []inboxbatch.Item, protected func(string) bool) []Item {
	var out []Item
	for _, it := range items {
		if routed, _ := inboxbatch.ConsoleRouted(it, protected); routed {
			out = append(out, Item{ID: it.ID, Deps: it.Deps, ConnectsTo: it.ConnectsTo})
		}
	}
	return out
}
