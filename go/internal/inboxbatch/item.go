// Package inboxbatch loads .evolve/inbox items, groups them into batches one cycle can carry,
// and decides which items are console-routed. See docs/architecture/packages/internal-inboxbatch.md.
package inboxbatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

type Item struct {
	ID              string                     `json:"id"`
	Title           string                     `json:"title"`
	Weight          float64                    `json:"weight"`
	Kind            string                     `json:"kind"`
	Class           string                     `json:"class"`
	Priority        string                     `json:"priority"`
	PriorityClass   string                     `json:"priority_class"`
	Campaign        string                     `json:"campaign"`
	Files           []string                   `json:"files"`
	ConnectsTo      []string                   `json:"connects_to"`
	Deps            []string                   `json:"deps"`
	Route           string                     `json:"route"`
	InjectedBy      string                     `json:"injected_by"`
	Continuation    *continuation.Continuation `json:"continuation,omitempty"`
	Acceptance      []string                   `json:"acceptance,omitempty"`
	DeliverableKind string                     `json:"deliverable_kind,omitempty"`
	CreatedAt       string                     `json:"created_at,omitempty"`
	Path            string                     `json:"-"`
	IDFromFileName  bool                       `json:"-"`
	mentions        []string
}

type ItemKey struct{ ID, Path string }

func (it Item) Key() ItemKey { return ItemKey{ID: it.ID, Path: it.Path} }

func (it *Item) UnmarshalJSON(raw []byte) error {
	type record Item
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

func (it Item) DeclaredSurface() bool {
	return len(declaredTokens(it.Files)) > 0
}

func (it Item) DeclaredPaths() []string {
	return declaredTokens(it.Files)
}

var filedAtLayouts = []string{time.RFC3339, "2006-01-02"}

const FilenameStampLayout = "2006-01-02T15-04-05Z"

func (it Item) FiledAt() time.Time {
	created := strings.TrimSpace(it.CreatedAt)
	for _, layout := range filedAtLayouts {
		if t, err := time.Parse(layout, created); err == nil {
			return t
		}
	}
	if base := filepath.Base(it.Path); len(base) >= len(FilenameStampLayout) {
		if t, err := time.Parse(FilenameStampLayout, base[:len(FilenameStampLayout)]); err == nil {
			return t
		}
	}
	return time.Time{}
}

func declaredTokens(files []string) []string {
	var out []string
	for _, tok := range surfaceTokens(files) {
		if isPathShaped(tok) {
			out = append(out, tok)
		}
	}
	return out
}

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

var lineLocatorRE = regexp.MustCompile(`(?::\d+(?:[-,:]\d+)*|#L\d+(?:-L?\d+)?)$`)

var repoPathRE = regexp.MustCompile(`^[A-Za-z0-9_.@-]+(?:/[A-Za-z0-9_.@-]+)*/$|^[A-Za-z0-9_.@-]+(?:/[A-Za-z0-9_.@-]+)+$`)

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

var surfaceAndProvenanceFields = map[string]bool{
	"files": true, "injected_by": true,
}

func skipMention(k string) bool {
	return surfaceAndProvenanceFields[k] || RoleOf(k).IsStamp()
}

const maxMentions = 64

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

func LoadDir(dir string) (items []Item, warnings []string, err error) {
	scan, err := ScanDir(dir)
	if err != nil {
		return nil, nil, err
	}
	for _, w := range scan.Warnings {
		warnings = append(warnings, w.Text)
	}
	return scan.Items, warnings, nil
}

const maxFieldLen = 160

const maxAcceptanceLen = 600

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
		it.ID, it.IDFromFileName = strings.TrimSuffix(name, ".json"), true
	}
	it.Path = name
	var warnings []string
	if r := sanitizeItem(&it); len(r.changed) > 0 {
		warnings = append(warnings, name+": "+r.notice())
	}
	return it, warnings, nil
}

type fieldRewrites struct {
	changed, stripped, truncated []string
}

func (r *fieldRewrites) clean(field, s string, max int) string {
	mapped := StripControl(s)
	if mapped != s {
		r.stripped = appendOnce(r.stripped, field)
	}
	if len(mapped) > max {
		cut := max
		for cut > 0 && !utf8.RuneStart(mapped[cut]) {
			cut--
		}
		mapped = mapped[:cut]
		r.truncated = appendOnce(r.truncated, field)
	}
	if mapped != s {
		r.changed = appendOnce(r.changed, field)
	}
	return mapped
}

func (r fieldRewrites) notice() string {
	var parts []string
	if len(r.stripped) > 0 {
		parts = append(parts, "sanitized control characters in "+strings.Join(r.stripped, ", "))
	}
	if len(r.truncated) > 0 {
		parts = append(parts, "truncated overlength "+strings.Join(r.truncated, ", ")+" to bound (item loaded)")
	}
	return strings.Join(parts, "; ")
}

func appendOnce(fields []string, field string) []string {
	if slices.Contains(fields, field) {
		return fields
	}
	return append(fields, field)
}

func sanitizeItem(it *Item) fieldRewrites {
	var r fieldRewrites
	it.ID = r.clean("id", it.ID, maxFieldLen)
	it.Title = r.clean("title", it.Title, maxFieldLen)
	it.Campaign = r.clean("campaign", it.Campaign, maxFieldLen)
	it.Route = r.clean("route", it.Route, maxFieldLen)
	it.DeliverableKind = r.clean("deliverable_kind", it.DeliverableKind, maxFieldLen)
	for i := range it.Files {
		it.Files[i] = r.clean("files", it.Files[i], maxFieldLen)
	}
	for i := range it.Acceptance {
		it.Acceptance[i] = r.clean("acceptance", it.Acceptance[i], maxAcceptanceLen)
	}
	return r
}

func SanitizedFields(it Item) []string {
	probe := it
	probe.Files = slices.Clone(it.Files)
	probe.Acceptance = slices.Clone(it.Acceptance)
	return sanitizeItem(&probe).changed
}

func StripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
}
