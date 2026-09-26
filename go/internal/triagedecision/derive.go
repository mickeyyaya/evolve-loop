// Package triagedecision is the one reader of triage's report as a decision (ADR-0106 H1). The report is the
// phase's primary deliverable and states the commitment in prose sections. Derive is the strict reading the
// host writes before any judge when the phase left the decision absent; Project is the lenient reading ship
// writes for the inbox lifecycle. Nothing the report does not state is written: floors, skipped items and an
// undeclared footprint are left to their readers' own fallbacks.
package triagedecision

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type card struct {
	ID     string   `json:"id"`
	Action string   `json:"action,omitempty"`
	Files  []string `json:"files,omitempty"`
}

type ref struct {
	ID string `json:"id"`
}

type dropped struct {
	ID     string `json:"id"`
	Reason string `json:"reason,omitempty"`
}

// decision omits floor declarations and skip lists on purpose; consumers treat each absence as safe.
type decision struct {
	Cycle      int             `json:"cycle"`
	TopN       []card          `json:"top_n"`
	Deferred   []ref           `json:"deferred"`
	Dropped    []dropped       `json:"dropped"`
	Superseded []string        `json:"superseded"`
	PhaseSkip  json.RawMessage `json:"phase_skip"`
	Projected  bool            `json:"projected_by_orchestrator"`
}

// Derive is the strict reading: `## top_n` must be present and state cards or none, every present bucket must
// be readable, and every pinned lane item must be committed. An absent deferred, dropped or superseded section
// is empty, which commits more, never less.
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

// Project is the lenient reading: a missing or unreadable section projects as [].
func Project(report string, cycle int) ([]byte, error) {
	d, _ := build(report, cycle, false)
	return json.MarshalIndent(d, "", "  ")
}

func build(report string, cycle int, strict bool) (decision, error) {
	d := decision{Cycle: cycle, Projected: true, PhaseSkip: json.RawMessage("[]"),
		TopN: []card{}, Deferred: []ref{}, Dropped: []dropped{}, Superseded: []string{}}
	top, present, err := section(report, "top_n", strict)
	if err != nil {
		return d, err
	}
	if strict && !present {
		return d, errors.New("triagedecision: the report has no `## top_n` section")
	}
	for _, it := range top {
		d.TopN = append(d.TopN, card{ID: it.ID, Action: ActionOf(it.Rest), Files: FilesOf(it.Rest)})
	}
	deferred, _, err := section(report, "deferred", strict)
	if err != nil {
		return d, err
	}
	for _, it := range deferred {
		d.Deferred = append(d.Deferred, ref{ID: it.ID})
	}
	droppedCards, _, err := section(report, "dropped", strict)
	if err != nil {
		return d, err
	}
	for _, it := range droppedCards {
		d.Dropped = append(d.Dropped, dropped{ID: it.ID, Reason: ReasonOf(it.Rest)})
	}
	superseded, _, err := section(report, "superseded", strict)
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

// section reads one bucket's cards; present is false when the section is absent. In strict mode a present
// bucket states cards or none and nothing else.
func section(report, name string, strict bool) (items []Item, present bool, err error) {
	body, ok := SectionBody(report, name)
	if !ok {
		return nil, false, nil
	}
	sec := ParseSection(body)
	if strict {
		if err := sectionError(name, sec); err != nil {
			return nil, true, err
		}
	}
	return sec.Items, true, nil
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

// supersededIDs lists each id once: ship retires them by id alone.
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
