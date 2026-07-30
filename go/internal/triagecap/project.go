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
	deferredSectionRE   = regexp.MustCompile(`(?m)^## deferred\b`)
	droppedSectionRE    = regexp.MustCompile(`(?m)^## dropped\b`)
	supersededSectionRE = regexp.MustCompile(`(?m)^## superseded\b`)
	dropReasonRE        = regexp.MustCompile(`reason=(.*)$`)
)

type projTopN struct {
	ID     string `json:"id"`
	Action string `json:"action,omitempty"`
	// Files is the card's DECLARED repo footprint, parsed from the item's
	// `files=a;b` metadata field. It is the only channel the fleet disjointness
	// planner can read (fleet.TodosFromTriage → Todo.Files → Partition/menus are
	// exact-file-overlap only): a card without it becomes an id island, so a
	// second lane can concurrently edit the same file it names in prose
	// (triage-cards-carry-files; live on cycles 1130/1133/1134). omitempty and
	// NEVER inferred from the action prose — an absent footprint is honest, a
	// guessed one silently merges or splits real lanes.
	Files []string `json:"files,omitempty"`
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
	Cycle      int           `json:"cycle"`
	TopN       []projTopN    `json:"top_n"`
	Deferred   []projID      `json:"deferred"`
	Dropped    []projDropped `json:"dropped"`
	Superseded []string      `json:"superseded"`
	Projected  bool          `json:"projected_by_orchestrator"`
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
		Cycle:      cycle,
		Projected:  true,
		TopN:       []projTopN{},
		Deferred:   []projID{},
		Dropped:    []projDropped{},
		Superseded: []string{},
	}
	if body, ok := sectionBody(artifact, topNHeadingRE); ok {
		for _, it := range parseItems(body) {
			d.TopN = append(d.TopN, projTopN{ID: it.id, Action: actionOf(it.rest), Files: filesOf(it.rest)})
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
	// superseded[] names inbox ids whose work already shipped under a different
	// id — retired by id ALONE at ship (inboxmover.ReconcileSuperseded), the
	// durable close of the cycle-544..548 orphan gap. Deduped here so the ship
	// hook receives a clean list even if the report repeats an id.
	if body, ok := sectionBody(artifact, supersededSectionRE); ok {
		seen := map[string]struct{}{}
		for _, it := range parseItems(body) {
			if _, dup := seen[it.id]; dup {
				continue
			}
			seen[it.id] = struct{}{}
			d.Superseded = append(d.Superseded, it.id)
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

// splitDeclaredFiles separates an item's `files=` metadata field(s) from the rest
// of its text: it returns the raw declared tokens and the item with every files=
// field REMOVED.
//
// Field-span parsing, not `files=(\S+)`: agents write `files=a.go; b.go` and
// `files=a.go, b.go` at least as often as the contract's `a.go;b.go`, and a
// value regex that stops at the first space both loses paths (a silently partial
// footprint is still an overlap on the dropped file) and leaves the remainder in
// the item text, where the floor counters would read those paths as package
// mentions. The span therefore runs from `files=` to the next `, key=` metadata
// field (or end of line), and every separator — `;`, `,`, whitespace — splits.
// Repeated files= fields are all consumed and unioned (L1: a second field left
// behind would feed exactly the leakage this avoids).
func splitDeclaredFiles(rest string) (tokens []string, stripped string) {
	var kept strings.Builder
	remaining := rest
	for {
		start := filesFieldRE.FindStringIndex(remaining)
		if start == nil {
			kept.WriteString(remaining)
			break
		}
		value := remaining[start[1]:]
		end := len(value)
		if m := nextMetadataFieldRE.FindStringIndex(value); m != nil {
			end = m[0]
		}
		if nl := strings.IndexByte(value[:end], '\n'); nl >= 0 {
			end = nl
		}
		tokens = append(tokens, strings.FieldsFunc(value[:end], isFilesSeparator)...)
		kept.WriteString(remaining[:start[0]])
		kept.WriteString(" ") // keep token boundaries intact for later matchers
		remaining = value[end:]
	}
	return tokens, kept.String()
}

// isFilesSeparator reports whether r separates two declared paths. Whitespace is
// included deliberately: a path cannot contain a space, so treating one as a
// separator can only ever split a list the agent wrote loosely.
func isFilesSeparator(r rune) bool {
	return r == ';' || r == ',' || r == ' ' || r == '\t'
}

// filesOf extracts the item's declared repo footprint. Only repo-relative paths
// survive (declaredFilePath) — the planner compares footprints by EXACT
// repo-relative path, so a mangled token could never match another card's and is
// dropped rather than handed over as a footprint that only looks like one (the
// drop is not silent: MissingCardFilesWarning reports a declaration that yields
// nothing usable). Returns nil when nothing is declared: the prose is NEVER mined
// for paths (see projTopN.Files).
func filesOf(rest string) []string {
	tokens, _ := splitDeclaredFiles(rest)
	var files []string
	seen := map[string]bool{}
	for _, tok := range tokens {
		p, ok := declaredFilePath(tok)
		if !ok || seen[p] {
			continue
		}
		seen[p] = true
		files = append(files, p)
	}
	return files
}

// declaredFilePath normalizes one declared token and reports whether it is usable
// as a footprint entry. The surrounding punctuation of the shapes agents actually
// write — `["a.go", "b.go"]`, backticked paths, a trailing sentence comma — is
// trimmed; what CANNOT be repaired is rejected: an unsubstituted template
// placeholder or a glob (no exact match is possible), an absolute path or a `..`
// escape (not repo-relative — the planner's whole comparison basis), and a bare
// filename (unlocatable).
func declaredFilePath(tok string) (string, bool) {
	p := strings.Trim(tok, "[]()\"'`,;:. \t")
	if p == "" || strings.ContainsAny(p, "{}*?<>") {
		return "", false
	}
	if strings.HasPrefix(p, "/") || strings.Contains(p, "..") || !strings.Contains(p, "/") {
		return "", false
	}
	return p, true
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
