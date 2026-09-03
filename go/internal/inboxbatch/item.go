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

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

// Item is the structured view of one .evolve/inbox/*.json entry. Fields are
// tolerant-by-default: real items are a mix of hand-authored and
// agent-autofiled JSON, so anything absent zero-values rather than erroring.
type Item struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Weight float64 `json:"weight"`
	Kind   string  `json:"kind"`
	// Class is the item's declared archetype ("pipeline-architecture",
	// "task-contract-design", …). Authors have been writing it into inbox JSON
	// for a while; it was silently dropped at load until cycle-1190. It is the
	// routing signal downstream archetype detectors key off (IsOperatorState).
	Class      string   `json:"class"`
	Priority   string   `json:"priority"`
	Campaign   string   `json:"campaign"`
	Files      []string `json:"files"`
	ConnectsTo []string `json:"connects_to"`
	Deps       []string `json:"deps"`
	// Route is the ADR-0074 dispatch-authority field: "console-*" values mark
	// the item operator-owned (never lane-dispatchable), "lane" is the explicit
	// override for protected-files false positives. Empty = derive (see
	// ConsoleRouted).
	Route string `json:"route"`
	// InjectedBy carries autofile provenance (retrofile, chronicle-escalation,
	// …). ADR-0074 clamp: an agent-autofiled item may NOT lane-override a
	// protected-surface derivation — agent-authored fields cannot widen agent
	// authority (ADR-0073 clamp-parity vocabulary).
	InjectedBy string `json:"injected_by"`
	// Continuation (ADR-0076 slice C) binds a FAILed cycle's preserved,
	// snapshot-committed work to this item so the next attempt resumes instead
	// of restarting cold. Machine-consumed only (never rendered into the triage
	// prompt); validated at adoption time, tolerant here. Nil = fresh start.
	Continuation *continuation.Continuation `json:"continuation,omitempty"`
	// Acceptance is the item's verbatim acceptance criteria. It is the SINGLE
	// source the harness projects into the tdd, build and audit prompts' Task Contract block
	// (ADR-0098) — never re-typed by an agent, so the builder and the auditor
	// grade against the same words.
	Acceptance []string `json:"acceptance,omitempty"`
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
		it, ws, ferr := LoadFile(filepath.Join(dir, name))
		if ferr != nil {
			warnings = append(warnings, name+": "+ferr.Error())
			continue
		}
		warnings = append(warnings, ws...)
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

// maxAcceptanceLen bounds one acceptance criterion as rendered into a prompt —
// wide enough for a real criterion (the filed items run 150–400 characters),
// narrow enough that an item cannot smuggle a page of instructions.
const maxAcceptanceLen = 600

// LoadFile reads ONE inbox item record (the shape the lane-scope resolver
// hands back per task id) with the same identity fallback and prompt-surface
// sanitisation LoadDir applies. Warnings are non-fatal sanitisation notes.
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
		// Filename stem is the stable fallback identity (some autofiled
		// items omit id; the filename is unique by construction).
		it.ID = strings.TrimSuffix(name, ".json")
	}
	it.Path = name
	// Prompt-injection surface: id/campaign/files/acceptance render into LLM
	// prompts (RenderMarkdown / Edge reasons / the Task Contract block). Strip
	// control characters and bound each field.
	var warnings []string
	if sanitizeItem(&it) {
		warnings = append(warnings, name+": sanitized control characters/overlength in rendered fields")
	}
	return it, warnings, nil
}

// sanitizeItem cleans the fields that reach the triage prompt, reporting
// whether anything changed. Control characters collapse to a single space
// (never a newline — one batch, one line) and overlength truncates.
func sanitizeItem(it *Item) bool {
	changed := false
	clean := func(s string) string { return cleanBounded(s, maxFieldLen, &changed) }
	it.ID = clean(it.ID)
	it.Title = clean(it.Title) // renders as the Task Contract heading
	it.Campaign = clean(it.Campaign)
	it.Route = clean(it.Route)
	for i := range it.Files {
		it.Files[i] = clean(it.Files[i])
	}
	for i := range it.Acceptance {
		it.Acceptance[i] = cleanBounded(it.Acceptance[i], maxAcceptanceLen, &changed)
	}
	return changed
}

// cleanBounded strips control characters and bounds s to max bytes, flagging
// changed when either applied.
func cleanBounded(s string, max int, changed *bool) string {
	mapped := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
	if len(mapped) > max {
		mapped = mapped[:max]
	}
	if mapped != s {
		*changed = true
	}
	return mapped
}
