package triage

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

type protectedCard struct{ ID, Path, Via string }

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
	return fmt.Sprintf("protected-surface: %s — control-plane changes go through the console route (operator-gated), not lane top_n", path)
}

func routeProtectedCards(decisionPath string, cards []protectedCard, bound []string) error {
	for _, c := range cards {
		if c.ID == "" {
			return fmt.Errorf("route protected cards: the card naming %s has no id", c.Path)
		}
	}
	raw, err := os.ReadFile(decisionPath)
	if err != nil {
		return fmt.Errorf("route protected cards: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("route protected cards: decision: %w", err)
	}
	topN, err := rawArray(fields["top_n"])
	if err != nil {
		return fmt.Errorf("route protected cards: top_n: %w", err)
	}
	escalations, err := rawArray(fields["escalate_block"])
	if err != nil {
		return fmt.Errorf("route protected cards: escalate_block: %w", err)
	}
	kept, removed := removeRoutedCards(topN, cards)
	escalated := escalatedIDs(escalations)
	for _, c := range cards {
		if !removed[c.ID] && !escalated[c.ID] {
			return fmt.Errorf("route protected cards: card %q is not in the decision's top_n", c.ID)
		}
	}
	routed := cards
	if len(kept) == 0 {
		routed = withBoundItem(cards, bound, raw)
	}
	fields["top_n"], _ = json.Marshal(kept)
	fields["escalate_block"], _ = json.Marshal(appendEscalations(escalations, escalated, routed))
	if err := atomicwrite.JSON(decisionPath, fields); err != nil {
		return fmt.Errorf("route protected cards: %w", err)
	}
	return nil
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

func removeRoutedCards(topN []json.RawMessage, cards []protectedCard) ([]json.RawMessage, map[string]bool) {
	routed := map[string]bool{}
	for _, c := range cards {
		routed[c.ID] = true
	}
	kept := make([]json.RawMessage, 0, len(topN))
	removed := map[string]bool{}
	for _, card := range topN {
		id := rawField(card, "id")
		if routed[id] {
			removed[id] = true
			continue
		}
		kept = append(kept, card)
	}
	return kept, removed
}

func escalatedIDs(escalations []json.RawMessage) map[string]bool {
	out := map[string]bool{}
	for _, e := range escalations {
		out[rawField(e, "task_id")] = true
	}
	return out
}

func appendEscalations(escalations []json.RawMessage, escalated map[string]bool, cards []protectedCard) []json.RawMessage {
	for _, c := range cards {
		if escalated[c.ID] {
			continue
		}
		entry, _ := json.Marshal(struct {
			TaskID string `json:"task_id"`
			Reason string `json:"reason"`
		}{c.ID, c.routeReason()})
		escalations = append(escalations, entry)
		escalated[c.ID] = true
	}
	return escalations
}

func rawArray(raw json.RawMessage) ([]json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []json.RawMessage
	err := json.Unmarshal(raw, &out)
	return out, err
}

func rawField(obj json.RawMessage, key string) string {
	var m map[string]json.RawMessage
	if json.Unmarshal(obj, &m) != nil {
		return ""
	}
	var s string
	_ = json.Unmarshal(m[key], &s)
	return s
}

func routedCardDiagnostics(cards []protectedCard) []core.Diagnostic {
	out := make([]core.Diagnostic, 0, len(cards))
	for _, c := range cards {
		out = append(out, core.Diagnostic{
			Severity: cyclestate.SeverityWarning,
			Message:  fmt.Sprintf("top_n card %q names protected surface %q — routed to the console by the host (escalate_block), not committed", c.ID, c.Path),
			Code:     cyclestate.DiagCodeTriageProtectedSurface,
			Subject:  c.ID,
		})
	}
	return out
}
