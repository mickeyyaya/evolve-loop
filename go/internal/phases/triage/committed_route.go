package triage

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func routedCommittedDiagnostics(req core.PhaseRequest, forbidden func(string) bool) []core.Diagnostic {
	if req.ProjectRoot == "" {
		return nil
	}
	ids := committedset.RoutedIDs(req.Workspace)
	if len(ids) == 0 {
		return nil
	}
	routed := inboxbatch.RoutedResolver(filepath.Join(paths.EvolveDirOf(req.ProjectRoot), "inbox"), forbidden)
	var cards []protectedCard
	for _, id := range ids {
		if isRouted, reason := routed(id); isRouted {
			cards = append(cards, protectedCard{ID: id, Reason: reason, Committed: true})
		}
	}
	return routedCardDiagnostics(cards)
}
