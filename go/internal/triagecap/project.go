package triagecap

import (
	"encoding/json"
	"regexp"
	"strings"
)

// project.go — deterministic projection of triage-report.md into the
// triage-decision.json companion the inbox-lifecycle hook (ship.postship
// promoteInbox) consumes. The triage AGENT is instructed to emit the companion
// but in practice almost never does (cycles 308/316/320-322 all missing it), so
// promote-to-processed never ran and claimed items ping-ponged back to inbox
// every cycle. This is the robust fallback: parse the markdown the agent DID
// write — single source (the report), guaranteed present — instead of trusting
// the LLM to also hand-author a parallel JSON that can drift from it.
//
// SUBSET BY DESIGN — each omitted field has absent-safe consumer semantics, so a
// projected companion is behaviourally identical to the no-companion baseline
// for every gate while still being PRESENT for promotion:
//   - committed_floors / deferred_floors are OMITTED: ReadDeclaredFloors /
//     ReadDeferredFloors treat an absent field as "fall back to the prose
//     counter", so the cap + eval gates keep counting the markdown directly.
//   - skip_shipped / skip_rejected / escalate_block require Step-0a's git-log
//     idempotency judgment, which the markdown does not carry; they are left
//     absent, so extractIDs walks top_n only — the documented inbox-lifecycle
//     behaviour (a shipped cycle is deemed to have addressed its top_n).

// idSlugRE validates an inbox task id (kebab-case slug). Items whose leading
// token is not a slug are SKIPPED: promotion moves an id out of the inbox, so a
// bogus id parsed from free-form prose would silently delete a non-existent
// item. Matching a closed structural shape — never an open substring — is the
// standing defence against agent content that resembles control structure.
var idSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Section headings are part of triage's required report structure
// (agents/evolve-triage.md Step 4). topNHeadingRE (floors.go) already anchors
// ## top_n via the phasecontract canonical; deferred/dropped have no separate
// contract section, so they are anchored to the documented literals here.
// Separate per-section locators (the floor counter's deferredHeadingRE in
// deferred.go deliberately matches deferred-OR-dropped as one set; the
// projection must keep the two sections distinct).
var (
	deferredSectionRE = regexp.MustCompile(`(?m)^## deferred\b`)
	droppedSectionRE  = regexp.MustCompile(`(?m)^## dropped\b`)
	dropReasonRE      = regexp.MustCompile(`reason=(.*)$`)
)

type projTopN struct {
	ID     string `json:"id"`
	Action string `json:"action,omitempty"`
}

type projID struct {
	ID string `json:"id"`
}

type projDropped struct {
	ID     string `json:"id"`
	Reason string `json:"reason,omitempty"`
}

// projectedDecision is the minimal-but-honest companion shape. projected_by_
// orchestrator marks it as a deterministic projection (vs an agent-authored
// companion) for forensics; unknown to the consumers' structs, so harmless.
type projectedDecision struct {
	Cycle     int           `json:"cycle"`
	TopN      []projTopN    `json:"top_n"`
	Deferred  []projID      `json:"deferred"`
	Dropped   []projDropped `json:"dropped"`
	Projected bool          `json:"projected_by_orchestrator"`
}

// ProjectDecisionJSON parses a triage-report.md body and returns the projected
// triage-decision.json bytes. Never errors on a malformed report — a missing
// section yields an empty slice (the loop is no worse off than with no
// companion at all).
func ProjectDecisionJSON(artifact string, cycle int) ([]byte, error) {
	// Initialize the three slice fields to empty (not nil) so an artifact with
	// empty/absent sections marshals each as [] rather than null. The consumer
	// (ship/postship.go) and any JSON reader expect arrays; a null top_n is a
	// live regression once disjoint packing can legitimately narrow it to zero.
	d := projectedDecision{
		Cycle:     cycle,
		Projected: true,
		TopN:      []projTopN{},
		Deferred:  []projID{},
		Dropped:   []projDropped{},
	}
	if body, ok := sectionBody(artifact, topNHeadingRE); ok {
		for _, it := range parseItems(body) {
			d.TopN = append(d.TopN, projTopN{ID: it.id, Action: actionOf(it.rest)})
		}
	}
	if body, ok := sectionBody(artifact, deferredSectionRE); ok {
		for _, it := range parseItems(body) {
			d.Deferred = append(d.Deferred, projID{ID: it.id})
		}
	}
	if body, ok := sectionBody(artifact, droppedSectionRE); ok {
		for _, it := range parseItems(body) {
			d.Dropped = append(d.Dropped, projDropped{ID: it.id, Reason: reasonOf(it.rest)})
		}
	}
	return json.MarshalIndent(d, "", "  ")
}

// rawItem is one parsed "- {id}: {rest}" list item with a valid slug id.
type rawItem struct {
	id   string
	rest string
}

// parseItems extracts the slug-id list items from one section body, skipping
// any line without a valid leading slug id (free-form prose, malformed rows).
func parseItems(body string) []rawItem {
	var items []rawItem
	for _, m := range listItemRE.FindAllStringSubmatch(body, -1) {
		id, rest, ok := splitID(m[1])
		if !ok {
			continue
		}
		items = append(items, rawItem{id: id, rest: rest})
	}
	return items
}

// splitID splits "id: rest" on the FIRST colon and validates the id as a slug.
func splitID(text string) (id, rest string, ok bool) {
	i := strings.IndexByte(text, ':')
	if i < 0 {
		return "", "", false
	}
	id = strings.TrimSpace(text[:i])
	if !idSlugRE.MatchString(id) {
		return "", "", false
	}
	return id, strings.TrimSpace(text[i+1:]), true
}

// actionOf is the rest text up to the em-dash metadata separator
// ("{action} — priority=…"), or the whole rest when there is no separator.
func actionOf(rest string) string {
	if i := strings.Index(rest, "—"); i >= 0 {
		return strings.TrimSpace(rest[:i])
	}
	return strings.TrimSpace(rest)
}

// reasonOf extracts the dropped item's "reason=…" tail (to end of line).
func reasonOf(rest string) string {
	if m := dropReasonRE.FindStringSubmatch(rest); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// sectionBody extracts a "## heading" section body (heading to the next "## "
// or EOF). Single home for section extraction — topNSection delegates here.
func sectionBody(artifact string, headingRE *regexp.Regexp) (string, bool) {
	loc := headingRE.FindStringIndex(artifact)
	if loc == nil {
		return "", false
	}
	body := artifact[loc[1]:]
	if next := nextHeadingRE.FindStringIndex(body); next != nil {
		body = body[:next[0]]
	}
	return body, true
}
