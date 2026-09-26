package triage

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

// protectedCard is a top_n card whose files name a lane-forbidden path.
type protectedCard struct{ ID, Path string }

// protectedTopNCards returns every top_n card that names a lane-forbidden path, in report order; a nil
// forbidden judges by manifest membership.
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

// forbiddenFileOf returns the first path of a card's files= field that forbidden judges lane-forbidden.
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

// consoleRouteReason is the escalation reason of a card the host routes to the console.
func consoleRouteReason(path string) string {
	return fmt.Sprintf("protected-surface: %s — control-plane changes go through the console route (operator-gated), not lane top_n", path)
}

// routeProtectedCards moves each card out of the decision's top_n into escalate_block with the
// console-route reason, so the item is answered and never committed; every other key is kept.
func routeProtectedCards(decisionPath string, cards []protectedCard) error {
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
	fields["top_n"], _ = json.Marshal(kept)
	fields["escalate_block"], _ = json.Marshal(appendEscalations(escalations, escalated, cards))
	if err := atomicwrite.JSON(decisionPath, fields); err != nil {
		return fmt.Errorf("route protected cards: %w", err)
	}
	return nil
}

// removeRoutedCards drops every top_n card whose id is one of cards and reports the ids it dropped.
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

// escalatedIDs indexes the task ids already in escalate_block.
func escalatedIDs(escalations []json.RawMessage) map[string]bool {
	out := map[string]bool{}
	for _, e := range escalations {
		out[rawField(e, "task_id")] = true
	}
	return out
}

// appendEscalations adds a console-route entry for each card not already escalated.
func appendEscalations(escalations []json.RawMessage, escalated map[string]bool, cards []protectedCard) []json.RawMessage {
	for _, c := range cards {
		if escalated[c.ID] {
			continue
		}
		entry, _ := json.Marshal(struct {
			TaskID string `json:"task_id"`
			Reason string `json:"reason"`
		}{c.ID, consoleRouteReason(c.Path)})
		escalations = append(escalations, entry)
		escalated[c.ID] = true
	}
	return escalations
}

// rawArray decodes an absent or null field as an empty array.
func rawArray(raw json.RawMessage) ([]json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []json.RawMessage
	err := json.Unmarshal(raw, &out)
	return out, err
}

// rawField reads one string field of a raw JSON object; anything else reads as "".
func rawField(obj json.RawMessage, key string) string {
	var m map[string]json.RawMessage
	if json.Unmarshal(obj, &m) != nil {
		return ""
	}
	var s string
	_ = json.Unmarshal(m[key], &s)
	return s
}

// routedCardDiagnostics is one warning per routed card, naming the card and its route.
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
