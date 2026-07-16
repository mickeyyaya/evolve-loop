// Package inboxbatch groups .evolve/inbox items into batches a SINGLE cycle
// can consume coherently — the deterministic half of task selection (Core Rule
// 5: grouping is mechanical signal-following, so it lives in Go; CHOOSING a
// batch stays the triage LLM's judgment). One-item-per-cycle consumption pays
// the full pipeline overhead (scout→triage→tdd→build→audit→ship) per item;
// batching related items amortizes it across work that shares a campaign, a
// package area, or an explicit dependency/link edge.
//
// Design: Strategy — each grouping signal is a Rule emitting edges; a
// union-find clusters items over the union of all rules' edges; batches order
// dep-topologically and split at a configurable cap. Pure and deterministic
// end to end: same inbox in, same batches out.
package inboxbatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Item is the structured view of one .evolve/inbox/*.json entry. Fields are
// tolerant-by-default: real items are a mix of hand-authored and
// agent-autofiled JSON, so anything absent zero-values rather than erroring.
type Item struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Weight     float64  `json:"weight"`
	Kind       string   `json:"kind"`
	Priority   string   `json:"priority"`
	Campaign   string   `json:"campaign"`
	Files      []string `json:"files"`
	ConnectsTo []string `json:"connects_to"`
	Deps       []string `json:"deps"`
	// Path is the source file (relative name inside the inbox dir) — operator
	// affordance for `evolve inbox batches` output; not part of grouping.
	Path string `json:"-"`
}

// LoadDir parses every *.json under dir into Items, sorted by ID for
// deterministic downstream grouping. A missing dir is an empty inbox (nil,
// nil, nil) — the loop runs fine with no backlog. A malformed item is skipped
// LOUDLY via the warnings slice (fail-open: one broken file must not hide the
// rest of the backlog), never silently. Non-JSON files are ignored (the inbox
// hosts occasional notes/subdirs).
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
		raw, ferr := os.ReadFile(filepath.Join(dir, name))
		if ferr != nil {
			warnings = append(warnings, name+": "+ferr.Error())
			continue
		}
		var it Item
		if jerr := json.Unmarshal(raw, &it); jerr != nil {
			warnings = append(warnings, name+": "+jerr.Error())
			continue
		}
		if it.ID == "" {
			// Filename stem is the stable fallback identity (some autofiled
			// items omit id; the filename is unique by construction).
			it.ID = strings.TrimSuffix(name, ".json")
		}
		it.Path = name
		// Prompt-injection surface: id/campaign/files render into the triage
		// LLM prompt (RenderMarkdown / Edge reasons). Strip control characters
		// + cap length at ingestion, LOUDLY, so a garbled or malicious item
		// can never fabricate a prompt line.
		if sanitizeItem(&it) {
			warnings = append(warnings, name+": sanitized control characters/overlength in rendered fields")
		}
		items = append(items, it)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	// A duplicate id silently mis-wires dep/connects resolution (last wins in
	// the resolver index) — keep both items but surface the collision.
	for i := 1; i < len(items); i++ {
		if items[i].ID == items[i-1].ID {
			warnings = append(warnings, items[i].Path+": duplicate id "+items[i].ID+" (also "+items[i-1].Path+") — dep/connects references resolve ambiguously")
		}
	}
	return items, warnings, nil
}

// maxFieldLen caps rendered fields — long enough for every legitimate id in
// the backlog, short enough that a runaway field cannot flood the prompt.
const maxFieldLen = 160

// sanitizeItem cleans the fields that reach the triage prompt, reporting
// whether anything changed. Control characters collapse to a single space
// (never a newline — one batch, one line) and overlength truncates.
func sanitizeItem(it *Item) bool {
	changed := false
	clean := func(s string) string {
		mapped := strings.Map(func(r rune) rune {
			if r < 0x20 || r == 0x7f {
				return ' '
			}
			return r
		}, s)
		if len(mapped) > maxFieldLen {
			mapped = mapped[:maxFieldLen]
		}
		if mapped != s {
			changed = true
		}
		return mapped
	}
	it.ID = clean(it.ID)
	it.Campaign = clean(it.Campaign)
	for i := range it.Files {
		it.Files[i] = clean(it.Files[i])
	}
	return changed
}
