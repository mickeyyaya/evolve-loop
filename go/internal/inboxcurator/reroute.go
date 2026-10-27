package inboxcurator

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const (
	RouteConsoleTarget = "console"
	RouteLaneTarget    = "lane"
)

var routingSources = []string{".evolve/policy.json", ".evolve/profiles", "go/internal/guards/integrity_surface.go"}

func TouchesRouting(paths []string) bool {
	for _, p := range paths {
		for _, src := range routingSources {
			if _, ok := samePlace(src, p); ok {
				return true
			}
		}
	}
	return false
}

func Reroutes(open []inboxbatch.Item, isProtected func(string) bool) []Proposal {
	var out []Proposal
	for _, it := range open {
		p := Proposal{Kind: ProposeReroute, ID: it.ID, Path: it.Path, Trigger: TriggerRouting}
		switch routed, why := inboxbatch.ConsoleRouted(it, isProtected); {
		case strings.EqualFold(strings.TrimSpace(it.Route), inboxbatch.RouteLaneValue) && routed:
			p.Target, p.Deterministic, p.Reason = RouteConsoleTarget, true, "the item names route lane, but "+why
			p.Evidence = []string{why}
		case UnderivedConsoleRoute(it, isProtected) && !recordsAReason(it.RoutedReason):
			p.Target, p.Reason = RouteLaneTarget, "route "+it.Route+" holds the item on the console, and no protected fix surface and no pipeline kind explain it"
			p.Evidence = []string{"route " + it.Route}
		default:
			continue
		}
		out = append(out, p)
	}
	return out
}

func recordsAReason(raw json.RawMessage) bool {
	var reason string
	return json.Unmarshal(raw, &reason) == nil && strings.TrimSpace(reason) != ""
}

func Duplicates(open []inboxbatch.Item, t Thresholds) []Proposal {
	sorted := slices.Clone(open)
	slices.SortStableFunc(sorted, func(a, b inboxbatch.Item) int {
		return cmp.Or(a.FiledAt().Compare(b.FiledAt()), cmp.Compare(a.ID, b.ID))
	})
	var out []Proposal
	for i, newer := range sorted {
		v := JudgeOverlap(newer, sorted[:i], Thresholds{Refuse: t.Refuse, Warn: t.Refuse})
		if len(v.Refused) == 0 {
			continue
		}
		top := v.Refused[0]
		out = append(out, Proposal{
			Kind: ProposeMerge, ID: newer.ID, Path: newer.Path, Target: top.ID, Trigger: TriggerSweep,
			Reason:   "the item overlaps the older open item " + top.ID + "; the operator or the orchestrator decides the merge",
			Evidence: []string{fmt.Sprintf("similarity %.2f on %s", top.Similarity, strings.Join(top.Surface, ", "))},
		})
	}
	return out
}
