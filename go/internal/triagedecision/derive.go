// Package triagedecision is the one reader of triage's report as a decision.
// See docs/architecture/packages/internal-triagedecision.md.
package triagedecision

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
)

type card struct {
	ID     string   `json:"id"`
	Action string   `json:"action,omitempty"`
	Files  []string `json:"files,omitempty"`
}

type ref struct {
	ID string `json:"id"`
}

type decision struct {
	Cycle    int    `json:"cycle"`
	TopN     []card `json:"top_n"`
	Deferred []ref  `json:"deferred"`
	committedset.DroppedBucket
	Superseded []string        `json:"superseded"`
	PhaseSkip  json.RawMessage `json:"phase_skip"`
	Projected  bool            `json:"projected_by_orchestrator"`
}

func Derive(report []byte, cycle int, lanePin []string) ([]byte, error) {
	d, err := build(string(report), cycle, true)
	if err != nil {
		return nil, err
	}
	if missing := missingPins(lanePin, d.TopN); missing != "" {
		return nil, fmt.Errorf("triagedecision: pinned lane item %s is not in the report's top_n", missing)
	}
	return json.MarshalIndent(d, "", "  ")
}

func Project(report string, cycle int) ([]byte, error) {
	d, _ := build(report, cycle, false)
	return json.MarshalIndent(d, "", "  ")
}

func build(report string, cycle int, strict bool) (decision, error) {
	d := decision{Cycle: cycle, Projected: true, PhaseSkip: json.RawMessage("[]"),
		TopN: []card{}, Deferred: []ref{}, DroppedBucket: committedset.DroppedBucket{Dropped: []committedset.Dropped{}}, Superseded: []string{}}
	top, present, err := section(report, "top_n", strict, true)
	if err != nil {
		return d, err
	}
	if strict && !present {
		return d, errors.New("triagedecision: the report has no `## top_n` section")
	}
	for _, it := range top {
		d.TopN = append(d.TopN, card{ID: it.ID, Action: ActionOf(it.Rest), Files: FilesOf(it.Rest)})
	}
	deferred, _, err := section(report, "deferred", strict, false)
	if err != nil {
		return d, err
	}
	for _, it := range deferred {
		d.Deferred = append(d.Deferred, ref{ID: it.ID})
	}
	droppedCards, _, err := section(report, "dropped", strict, false)
	if err != nil {
		return d, err
	}
	for _, it := range droppedCards {
		d.Dropped = append(d.Dropped, committedset.Dropped{ID: it.ID, Reason: ReasonOf(it.Rest)})
	}
	superseded, _, err := section(report, "superseded", strict, false)
	if err != nil {
		return d, err
	}
	d.Superseded = append(d.Superseded, supersededIDs(superseded)...)
	skip := headerValue(report, "phase_skip:")
	switch {
	case skip == "":
	case json.Valid([]byte(skip)) && strings.HasPrefix(skip, "["):
		d.PhaseSkip = json.RawMessage(skip)
	case strict:
		return d, fmt.Errorf("triagedecision: phase_skip header is not a JSON array: %q", skip)
	}
	return d, nil
}

func section(report, name string, strict, required bool) (items []Item, present bool, err error) {
	body, ok := SectionBody(report, name)
	if !ok {
		return nil, false, nil
	}
	sec := ParseSection(body)
	if strict && (required || !sec.empty()) {
		if err := sectionError(name, sec); err != nil {
			return nil, true, err
		}
	}
	return sec.Items, true, nil
}

func (s Section) empty() bool {
	return !s.None && len(s.Items) == 0 && len(s.Rejected) == 0 && len(s.Prose) == 0
}

func sectionError(name string, sec Section) error {
	switch {
	case len(sec.Rejected) > 0:
		return fmt.Errorf("triagedecision: `## %s` bullet has no slug id: %q", name, sec.Rejected[0])
	case len(sec.Prose) > 0:
		return fmt.Errorf("triagedecision: `## %s` holds prose, not cards: %q", name, sec.Prose[0])
	case sec.None && len(sec.Items) > 0:
		return fmt.Errorf("triagedecision: `## %s` states both cards and none", name)
	case !sec.None && len(sec.Items) == 0:
		return fmt.Errorf("triagedecision: `## %s` states neither cards nor none", name)
	}
	return nil
}

func supersededIDs(items []Item) []string {
	seen := map[string]bool{}
	var ids []string
	for _, it := range items {
		if !seen[it.ID] {
			seen[it.ID] = true
			ids = append(ids, it.ID)
		}
	}
	return ids
}

func headerValue(report string, prefix string) string {
	for _, line := range bytes.Split([]byte(report), []byte("\n")) {
		if rest, ok := bytes.CutPrefix(bytes.TrimSpace(line), []byte(prefix)); ok {
			return string(bytes.TrimSpace(rest))
		}
	}
	return ""
}

func missingPins(pin []string, topN []card) string {
	committed := map[string]bool{}
	for _, c := range topN {
		committed[c.ID] = true
	}
	var missing []string
	for _, id := range pin {
		if !committed[id] {
			missing = append(missing, id)
		}
	}
	return strings.Join(missing, ", ")
}

func DeriveRouted(report []byte, cycle int, unroutedPin, routed []string) ([]byte, error) {
	if len(routed) > 0 && len(unroutedPin) == 0 && onlyNoneTopN(string(report)) {
		return Project(string(report), cycle)
	}
	return Derive(report, cycle, unroutedPin)
}

func onlyNoneTopN(report string) bool {
	body, ok := SectionBody(report, "top_n")
	if !ok {
		return false
	}
	sec := ParseSection(body)
	if len(sec.Items)+len(sec.Rejected) != 0 {
		return false
	}
	for _, line := range sec.Prose {
		if !strings.EqualFold(strings.TrimSpace(line), "none") {
			return false
		}
	}
	return true
}

type Route struct{ ID, Reason string }

type Edit struct {
	Raw       []byte
	Kept      int
	Removed   map[string]bool
	Escalated map[string]bool
}

func ConsoleRouteReason(reason string) string {
	return "console-routed: " + reason + " — operator-owned work is never built by a lane"
}

func ApplyRoutes(path string, ids []string, plan func(Edit) ([]Route, error)) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("decision: %w", err)
	}
	topN, err := rawArray(fields["top_n"])
	if err != nil {
		return fmt.Errorf("top_n: %w", err)
	}
	escalations, err := rawArray(fields["escalate_block"])
	if err != nil {
		return fmt.Errorf("escalate_block: %w", err)
	}
	kept, removed := removeCards(topN, ids)
	escalated := escalatedIDs(escalations)
	routes, err := plan(Edit{Raw: raw, Kept: len(kept), Removed: removed, Escalated: escalated})
	if err != nil {
		return err
	}
	grown := appendEscalations(escalations, escalated, routes)
	if len(removed) == 0 && len(grown) == len(escalations) {
		return nil
	}
	fields["top_n"], _ = json.Marshal(kept)
	fields["escalate_block"], _ = json.Marshal(grown)
	return atomicwrite.JSON(path, fields)
}

func removeCards(topN []json.RawMessage, ids []string) ([]json.RawMessage, map[string]bool) {
	routed := map[string]bool{}
	for _, id := range ids {
		routed[id] = true
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

func appendEscalations(escalations []json.RawMessage, escalated map[string]bool, routes []Route) []json.RawMessage {
	for _, r := range routes {
		if escalated[r.ID] {
			continue
		}
		entry, _ := json.Marshal(struct {
			TaskID string `json:"task_id"`
			Reason string `json:"reason"`
		}{r.ID, r.Reason})
		escalations = append(escalations, entry)
		escalated[r.ID] = true
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
