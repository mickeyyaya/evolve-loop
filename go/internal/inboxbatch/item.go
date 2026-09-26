// Package inboxbatch loads .evolve/inbox items, groups them into batches one cycle can carry,
// and decides which items are console-routed. See docs/architecture/packages/internal-inboxbatch.md.
package inboxbatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

// Item is one .evolve/inbox/*.json record; absent fields zero-value rather than error.
type Item struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Weight float64 `json:"weight"`
	Kind   string  `json:"kind"`
	// Class is the declared archetype that IsOperatorState keys on.
	Class      string   `json:"class"`
	Priority   string   `json:"priority"`
	Campaign   string   `json:"campaign"`
	Files      []string `json:"files"`
	ConnectsTo []string `json:"connects_to"`
	Deps       []string `json:"deps"`
	// Route "console-*" makes the item operator-owned; "lane" overrides a heuristic derivation; empty derives.
	Route string `json:"route"`
	// InjectedBy is autofile provenance; a non-empty value clamps the route:"lane" override.
	InjectedBy string `json:"injected_by"`
	// Continuation binds a failed cycle's preserved work; machine-consumed, never rendered.
	Continuation *continuation.Continuation `json:"continuation,omitempty"`
	// Acceptance is the single source of the Task Contract block's criteria.
	Acceptance []string `json:"acceptance,omitempty"`
	// DeliverableKind is "code" (default) or "document".
	DeliverableKind string `json:"deliverable_kind,omitempty"`
	// CreatedAt is the filing timestamp as authored: RFC3339 or a bare date.
	CreatedAt string `json:"created_at,omitempty"`
	// Path is the file name inside the inbox dir, for display only.
	Path string `json:"-"`
	// mentions are the files the record's own text names, derived at decode.
	mentions []string
}

// UnmarshalJSON decodes a record and derives the files its author-written fields name.
// Deriving at decode gives every reader (wave seed, claim floor, LoadFile) the same surface.
func (it *Item) UnmarshalJSON(raw []byte) error {
	type record Item // drops this method, so decoding does not recurse
	var r record
	if err := json.Unmarshal(raw, &r); err != nil {
		return err
	}
	*it = Item(r)
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err == nil {
		it.mentions = mentionedFiles(doc)
	}
	return nil
}

// DeclaredSurface reports whether any files[] token is path-shaped; placeholders and bare file names declare nothing.
func (it Item) DeclaredSurface() bool {
	return len(declaredTokens(it.Files)) > 0
}

// DeclaredPaths returns the item's declared fix surface: its path-shaped files[] tokens.
func (it Item) DeclaredPaths() []string {
	return declaredTokens(it.Files)
}

// filedAtLayouts are the created_at shapes authors write, most specific first.
var filedAtLayouts = []string{time.RFC3339, "2006-01-02"}

// filenameStampLayout is the timestamp prefix of inbox file names; colons are not filename-safe.
const filenameStampLayout = "2006-01-02T15-04-05Z"

// FiledAt is when the item was filed: its created_at, else its file name's timestamp prefix.
// It is zero when neither parses; premise drift measures from it, so a date is never guessed.
func (it Item) FiledAt() time.Time {
	created := strings.TrimSpace(it.CreatedAt)
	for _, layout := range filedAtLayouts {
		if t, err := time.Parse(layout, created); err == nil {
			return t
		}
	}
	if base := filepath.Base(it.Path); len(base) >= len(filenameStampLayout) {
		if t, err := time.Parse(filenameStampLayout, base[:len(filenameStampLayout)]); err == nil {
			return t
		}
	}
	return time.Time{}
}

// declaredTokens is the path-shaped files[] tokens: the declared surface the console classifier judges.
func declaredTokens(files []string) []string {
	var out []string
	for _, tok := range surfaceTokens(files) {
		if isPathShaped(tok) {
			out = append(out, tok)
		}
	}
	return out
}

// surfaceTokens splits files[] entries into bare tokens ("a.go b.go", "(a.go)", "a.go;", "a.go:178").
func surfaceTokens(files []string) []string {
	var out []string
	for _, f := range files {
		for _, tok := range strings.Fields(f) {
			tok = lineLocatorRE.ReplaceAllString(strings.Trim(tok, "()[]{},;:'\""), "")
			if tok != "" {
				out = append(out, tok)
			}
		}
	}
	return out
}

// lineLocatorRE matches a trailing citation locator (":178", ":189,205,221", ":10-20", "#L10-L20"),
// which is not part of the path; a date-shaped suffix strips too.
var lineLocatorRE = regexp.MustCompile(`(?::\d+(?:[-,:]\d+)*|#L\d+(?:-L?\d+)?)$`)

// repoPathRE matches a whole token of slash-joined path segments, or one segment with a trailing slash ("go/").
var repoPathRE = regexp.MustCompile(`^[A-Za-z0-9_.@-]+(?:/[A-Za-z0-9_.@-]+)*/$|^[A-Za-z0-9_.@-]+(?:/[A-Za-z0-9_.@-]+)+$`)

// isPathShaped accepts "go/", "docs/x.md" and "skills/audit" but not "N/A", "w/o", "TBD" or a bare "role.go".
func isPathShaped(tok string) bool {
	if !repoPathRE.MatchString(tok) {
		return false
	}
	for _, seg := range strings.Split(strings.TrimSuffix(tok, "/"), "/") {
		if len(seg) > 1 {
			return true
		}
	}
	return false
}

var pathInProseRE = regexp.MustCompile(`[A-Za-z0-9_.@-]+(?:/[A-Za-z0-9_.@-]+)+/?`)

// mentionSkip holds the declared surface and the machine-written fields the mention walk never reads.
var mentionSkip = map[string]bool{
	"files": true, "continuation": true, "route": true, "injected_by": true,
}

// skipMention also skips every "routed_" field, the prefix the console router stamps on all it writes.
func skipMention(k string) bool {
	return mentionSkip[k] || strings.HasPrefix(k, "routed_")
}

// maxMentions bounds the walk so a runaway record cannot make routing costly.
const maxMentions = 64

// mentionedFiles returns the distinct file paths the fields name, in walk order; directory mentions are context.
func mentionedFiles(doc map[string]any) []string {
	var out []string
	seen := map[string]bool{}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			for _, p := range pathInProseRE.FindAllString(x, -1) {
				if len(out) < maxMentions && !seen[p] && isFileSpelling(p) {
					seen[p] = true
					out = append(out, p)
				}
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		case map[string]any:
			for _, k := range sortedKeys(x) {
				if !skipMention(k) {
					walk(x[k])
				}
			}
		}
	}
	walk(doc)
	return out
}

// isFileSpelling reports whether p's last segment has an extension after its first character ("runner.go", not ".evolve").
func isFileSpelling(p string) bool {
	last := p[strings.LastIndex(p, "/")+1:]
	return len(last) > 1 && strings.Contains(last[1:], ".")
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// LoadDir loads every *.json in dir sorted by ID; a missing dir is empty and a malformed file is a warning.
func LoadDir(dir string) (items []Item, warnings []string, err error) {
	entries, rerr := os.ReadDir(dir)
	if rerr != nil {
		if os.IsNotExist(rerr) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("inboxbatch: read dir: %w", rerr)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		it, ws, ferr := LoadFile(filepath.Join(dir, name))
		if ferr != nil {
			warnings = append(warnings, name+": "+ferr.Error())
			continue
		}
		warnings = append(warnings, ws...)
		items = append(items, it)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	// The resolver index keeps the last duplicate, so a collision is surfaced rather than dropped.
	for i := 1; i < len(items); i++ {
		if items[i].ID == items[i-1].ID {
			warnings = append(warnings, items[i].Path+": duplicate id "+items[i].ID+" (also "+items[i-1].Path+") — dep/connects references resolve ambiguously")
		}
	}
	return items, warnings, nil
}

// maxFieldLen fits every legitimate id yet stops a runaway field flooding the prompt.
const maxFieldLen = 160

// maxAcceptanceLen fits a real criterion yet stops an item smuggling in a page of instructions.
const maxAcceptanceLen = 600

// LoadFile loads one record with LoadDir's id fallback and sanitization; warnings are non-fatal.
func LoadFile(path string) (Item, []string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Item{}, nil, err
	}
	var it Item
	if err := json.Unmarshal(raw, &it); err != nil {
		return Item{}, nil, err
	}
	name := filepath.Base(path)
	if it.ID == "" {
		it.ID = strings.TrimSuffix(name, ".json")
	}
	it.Path = name
	var warnings []string
	if sanitizeItem(&it) {
		warnings = append(warnings, name+": sanitized control characters/overlength in rendered fields")
	}
	return it, warnings, nil
}

// sanitizeItem cleans the prompt-rendered fields and reports whether anything changed.
func sanitizeItem(it *Item) bool {
	changed := false
	clean := func(s string) string { return cleanBounded(s, maxFieldLen, &changed) }
	it.ID = clean(it.ID)
	it.Title = clean(it.Title)
	it.Campaign = clean(it.Campaign)
	it.Route = clean(it.Route)
	it.DeliverableKind = clean(it.DeliverableKind)
	for i := range it.Files {
		it.Files[i] = clean(it.Files[i])
	}
	for i := range it.Acceptance {
		it.Acceptance[i] = cleanBounded(it.Acceptance[i], maxAcceptanceLen, &changed)
	}
	return changed
}

// StripControl replaces control characters (C0 and DEL) with spaces.
// A newline in agent-authored text entering a prompt would forge a new context line.
func StripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
}

// cleanBounded strips control characters and truncates s to max bytes, flagging changed when either applied.
func cleanBounded(s string, max int, changed *bool) string {
	mapped := StripControl(s)
	if len(mapped) > max {
		mapped = mapped[:max]
	}
	if mapped != s {
		*changed = true
	}
	return mapped
}
