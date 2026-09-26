package triagecap

import (
	"encoding/json"
	"regexp"
	"strings"
)

// idSlugRE rejects non-slug ids: promotion moves an id out of the inbox, so an id parsed from prose must never pass.
var idSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// These headings have no phasecontract section, so they anchor on the triage persona's literals.
// Unlike deferredHeadingRE, the projection keeps deferred and dropped distinct.
var (
	deferredSectionRE   = regexp.MustCompile(`(?m)^## deferred\b`)
	droppedSectionRE    = regexp.MustCompile(`(?m)^## dropped\b`)
	supersededSectionRE = regexp.MustCompile(`(?m)^## superseded\b`)
	dropReasonRE        = regexp.MustCompile(`reason=(.*)$`)
)

type projTopN struct {
	ID     string `json:"id"`
	Action string `json:"action,omitempty"`
	// Files is the declared files= footprint, the planner's only disjointness input. Never inferred from
	// prose: an absent footprint is honest, a guessed one merges or splits real lanes.
	Files []string `json:"files,omitempty"`
}

type projID struct {
	ID string `json:"id"`
}

type projDropped struct {
	ID     string `json:"id"`
	Reason string `json:"reason,omitempty"`
}

// projectedDecision omits floor declarations and skip lists on purpose; consumers treat each absence as safe.
type projectedDecision struct {
	Cycle      int           `json:"cycle"`
	TopN       []projTopN    `json:"top_n"`
	Deferred   []projID      `json:"deferred"`
	Dropped    []projDropped `json:"dropped"`
	Superseded []string      `json:"superseded"`
	Projected  bool          `json:"projected_by_orchestrator"`
}

// ProjectDecisionJSON derives triage-decision.json from a triage report; a missing section projects as [].
func ProjectDecisionJSON(artifact string, cycle int) ([]byte, error) {
	// Empty, not nil: consumers expect arrays, and disjoint packing can legitimately narrow top_n to zero.
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
	// Superseded ids shipped under another id; ship retires them by id alone, so each appears once.
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

// rawItem is one "- {id}: {rest}" list item with a valid slug id.
type rawItem struct {
	id   string
	rest string
}

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

// actionOf is the text before the em-dash metadata separator ("{action} — priority=…").
func actionOf(rest string) string {
	if i := strings.Index(rest, "—"); i >= 0 {
		return strings.TrimSpace(rest[:i])
	}
	return strings.TrimSpace(rest)
}

// splitDeclaredFiles returns every files= field's tokens and the item with those fields removed.
// A field spans to the next ", key=" or end of line, because agents separate paths with spaces and commas too.
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

func isFilesSeparator(r rune) bool {
	return r == ';' || r == ',' || r == ' ' || r == '\t'
}

// filesOf keeps only usable repo-relative paths, since the planner matches exactly.
// MissingCardFilesWarning reports a declaration that yields none.
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

// declaredFilePath trims agent punctuation and rejects placeholders, globs, absolute or ".." paths, and bare names.
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

func reasonOf(rest string) string {
	if m := dropReasonRE.FindStringSubmatch(rest); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// sectionBody returns the text from headingRE's match to the next "## " heading or EOF.
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
