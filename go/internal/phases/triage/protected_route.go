package triage

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagedecision"
)

type protectedCard struct {
	ID, Path, Via, Reason string
	Committed             bool
}

func protectedTopNCards(body string, forbidden func(string) bool) []protectedCard {
	if forbidden == nil {
		forbidden = guards.IsProtectedSurface
	}
	var cards []protectedCard
	for _, line := range strings.Split(body, "\n") {
		if !listItemRE.MatchString(line) {
			continue
		}
		if path, hit := forbiddenFileOf(line, forbidden); hit {
			cards = append(cards, protectedCard{ID: cardIDOf(line), Path: path})
		}
	}
	return cards
}

func forbiddenFileOf(line string, forbidden func(string) bool) (string, bool) {
	filesMatch := filesFieldRE.FindStringSubmatch(line)
	if filesMatch == nil {
		return "", false
	}
	filesRaw := filesMatch[1]
	if filesRaw == "" {
		filesRaw = filesMatch[2]
	}
	for _, f := range strings.Split(filesRaw, ";") {
		f = strings.TrimSpace(f)
		if idx := strings.Index(f, "("); idx >= 0 {
			f = strings.TrimSpace(f[:idx])
		}
		if f != "" && forbidden(f) {
			return f, true
		}
	}
	return "", false
}

func cardIDOf(line string) string {
	if idMatch := topNItemIDRE.FindStringSubmatch(line); idMatch != nil {
		return strings.TrimSpace(idMatch[1])
	}
	return ""
}

func consoleRouteReason(path string) string {
	return fmt.Sprintf("%s %s — control-plane changes go through the console route (operator-gated), not lane top_n", committedset.ProtectedSurfaceReasonPrefix, path)
}

func routeProtectedCards(decisionPath string, cards []protectedCard, bound []string) error {
	ids := make([]string, len(cards))
	for i, c := range cards {
		if c.ID == "" {
			return fmt.Errorf("route protected cards: the card naming %s has no id", c.Path)
		}
		ids[i] = c.ID
	}
	err := triagedecision.ApplyRoutes(decisionPath, ids, func(e triagedecision.Edit) ([]triagedecision.Route, error) {
		for _, c := range cards {
			if !e.Removed[c.ID] && !e.Escalated[c.ID] {
				return nil, fmt.Errorf("card %q is not in the decision's top_n", c.ID)
			}
		}
		routed := cards
		if e.Kept == 0 {
			routed = withBoundItem(cards, bound, e.Raw)
		}
		return routesOf(routed), nil
	})
	if err != nil {
		return fmt.Errorf("route protected cards: %w", err)
	}
	return nil
}

func routesOf(cards []protectedCard) []triagedecision.Route {
	routes := make([]triagedecision.Route, len(cards))
	for i, c := range cards {
		routes[i] = triagedecision.Route{ID: c.ID, Reason: c.routeReason()}
	}
	return routes
}

func withBoundItem(cards []protectedCard, bound []string, decision []byte) []protectedCard {
	if len(cards) == 0 || len(bound) != 1 {
		return cards
	}
	id := bound[0]
	if answersFor(decision, id) || slices.ContainsFunc(cards, func(c protectedCard) bool { return c.ID == id }) {
		return cards
	}
	return append(slices.Clip(cards), protectedCard{ID: id, Path: cards[0].Path, Via: cards[0].ID})
}

func (c protectedCard) routeReason() string {
	if c.Via == "" {
		return consoleRouteReason(c.Path)
	}
	return fmt.Sprintf("%s (the lane's item, answered for by routed card %q)", consoleRouteReason(c.Path), c.Via)
}

func answersFor(decision []byte, id string) bool {
	for _, d := range committedset.DispositionsFrom(decision) {
		if d.ID == id {
			return true
		}
	}
	return false
}

func routedCardDiagnostics(cards []protectedCard) []core.Diagnostic {
	out := make([]core.Diagnostic, 0, len(cards))
	for _, c := range cards {
		out = append(out, core.Diagnostic{
			Severity: cyclestate.SeverityWarning,
			Message:  c.routedText(),
			Code:     cyclestate.DiagCodeTriageProtectedSurface,
			Subject:  c.ID,
		})
	}
	return out
}

func (c protectedCard) routedText() string {
	if c.Committed {
		return fmt.Sprintf("committed item %q is console-routed (%s): routed to the console by the host (escalate_block), not committed. Next: the console builds it. This lane has no work for it.", c.ID, c.Reason)
	}
	return fmt.Sprintf("top_n card %q names protected surface %q — routed to the console by the host (escalate_block), not committed", c.ID, c.Path)
}
