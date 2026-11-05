package deliverable

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagedecision"
)

const recordedRouteReason = "the host recorded this route earlier in the cycle"

func routeThenClaim(h *HostEffects, c phasecontract.Contract, roots phasecontract.Roots) error {
	return errors.Join(routeCommitted(h, c, roots), claimCommitted(h, roots))
}

func routeCommitted(h *HostEffects, c phasecontract.Contract, roots phasecontract.Roots) error {
	ws := roots.Workspace
	pin := committedset.LanePin(ws)
	topN, _ := committedset.DecisionTopN(ws)
	if len(pin)+len(topN)+len(committedset.RoutedIDs(ws)) == 0 {
		return nil
	}
	if roots.Cycle < 1 || !filepath.IsAbs(roots.EvolveDir) {
		return errors.New("no cycle or project root to route into")
	}
	routed := inboxbatch.RoutedResolver(inboxDir(roots), h.forbidden)
	if err := recordRouted(ws, withoutDeferred(ws, pin), routed); err != nil {
		return err
	}
	h.derive(c, roots)
	if _, decided := committedset.DecisionTopN(ws); !decided {
		return nil
	}
	topN, _ = committedset.DecisionTopN(ws)
	if err := recordRouted(ws, withoutDeferred(ws, topN), routed); err != nil {
		return err
	}
	return editDecision(ws, routed)
}

func withoutDeferred(workspace string, ids []string) []string {
	deferred := committedset.Deferred(workspace)
	var out []string
	for _, id := range ids {
		if !slices.Contains(deferred, id) {
			out = append(out, id)
		}
	}
	return out
}

func recordRouted(workspace string, candidates []string, routed func(string) (bool, string)) error {
	for _, id := range candidates {
		isRouted, reason := routed(id)
		if !isRouted {
			continue
		}
		if err := committedset.RecordWorkspaceRoutes(workspace, []string{id}); err != nil {
			return fmt.Errorf("route %q (%s): %w", id, reason, err)
		}
	}
	return nil
}

func editDecision(workspace string, routed func(string) (bool, string)) error {
	ids := committedset.RoutedIDs(workspace)
	routes := make([]triagedecision.Route, len(ids))
	for i, id := range ids {
		reason := recordedRouteReason
		if isRouted, why := routed(id); isRouted {
			reason = why
		}
		routes[i] = triagedecision.Route{ID: id, Reason: triagedecision.ConsoleRouteReason(reason)}
	}
	path := filepath.Join(workspace, committedset.DecisionFile)
	if err := triagedecision.ApplyRoutes(path, ids, func(triagedecision.Edit) ([]triagedecision.Route, error) { return routes, nil }); err != nil {
		return fmt.Errorf("route cards: %w", err)
	}
	return nil
}

func flagRoutedInTopN(res *Result, roots phasecontract.Roots) {
	topN, _ := committedset.DecisionTopN(roots.Workspace)
	for _, id := range committedset.RoutedIDs(roots.Workspace) {
		if slices.Contains(topN, id) {
			res.add(CodeMissingEffect, fmt.Sprintf("review gate: console-routed item %q is still in top_n of %s. The host removes it from top_n and lists it under escalate_block. Next: %s", id, committedset.DecisionFile, hostFaultNext(roots.Cycle)))
		}
	}
}

func checkRouteThenClaim(res *Result, roots phasecontract.Roots) error {
	flagRoutedInTopN(res, roots)
	return checkInboxClaim(res, roots)
}

func hostFaultNext(cycle int) string {
	return fmt.Sprintf("the console reads the error with evolve signals tail --cycle %d --code RUNNER_HOST_EFFECT_FAILED,ORCHESTRATOR_HOST_EFFECT_FAILED.", cycle)
}
