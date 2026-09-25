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
	"regexp"
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
	// DeliverableKind is what the item wants built — "code" (default) or
	// "document" (ADR-0099: a solutions/<id>/ deliverable with candidate options
	// and a recommendation). Projected into the Task Contract block; the cycle's
	// authoritative kind is what triage declares in its report header.
	DeliverableKind string `json:"deliverable_kind,omitempty"`
	// Path is the source file (relative name inside the inbox dir) — operator
	// affordance for `evolve inbox batches` output; not part of grouping.
	Path string `json:"-"`
	// mentions are the FILES the record's own text names, derived at decode
	// (UnmarshalJSON). With no declared surface they are the item's surface for
	// the console classifier (F29) — the surface triage's breaker would
	// otherwise derive only after a lane has paid for scout and triage.
	mentions []string
}

// UnmarshalJSON decodes an inbox record and derives the files its own text
// names. Deriving at DECODE, not in one loader, is deliberate: the wave seed,
// the claim floor and LoadFile each unmarshal records themselves, and the
// console classifier must see the same surface from every one of them. The
// walk reads DECODED strings (so an escaped "\/" is still a slash) from every
// author-written field — authors spread paths across summary, fix, notes,
// root_cause, problem, details and the rest — skipping the declared surface
// and the machine-written fields (mentionSkip).
func (it *Item) UnmarshalJSON(raw []byte) error {
	type record Item // the same fields without this method: default decoding
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

// DeclaredSurface reports whether the item DECLARES its fix surface: at least
// one files[] token shaped like a repo path (a slash-separated path). It is the
// ONE home of that belief — the console classifier's "a declared surface wins"
// rule and the seed's admissibility tie-break both read it (F29) — and a
// placeholder ("TBD", "N/A", "()") or a bare file name ("role.go", which the
// triage LLM would resolve into the tree) declares nothing.
func (it Item) DeclaredSurface() bool {
	return len(declaredTokens(it.Files)) > 0
}

// declaredTokens is the ONE token set a declared surface consists of: the
// path-shaped files[] tokens. The console classifier judges exactly these in
// scope and DeclaredSurface asks whether any exist, so an annotation word
// ("(go test)" yields "go") is never read as a directory (F29 review).
func declaredTokens(files []string) []string {
	var out []string
	for _, tok := range surfaceTokens(files) {
		if isPathShaped(tok) {
			out = append(out, tok)
		}
	}
	return out
}

// surfaceTokens splits files[] entries into bare tokens (the loose shapes
// authors write: "a.go b.go", "(a.go)", "a.go;", "a.go:178").
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

// lineLocatorRE matches the source locator authors append when citing a file
// ("a.go:178", "a.go:178:5", "a.go:189,205,221", "a.go:10-20", "a.go#L10-L20")
// — any trailing ":<digits>" run joined by "-", "," or ":", so a date-shaped suffix
// ("x.md:2026-09-26") strips too, equally not part of the path. It is how the
// file is cited, not part of its path — kept, the ":" failed the path shape
// and the file went unjudged while triage's breaker still matched it (F35).
var lineLocatorRE = regexp.MustCompile(`(?::\d+(?:[-,:]\d+)*|#L\d+(?:-L?\d+)?)$`)

// repoPathRE matches a WHOLE slash-bearing token: segments of path characters
// joined by slashes, or one segment with a trailing slash ("go/", "skills/").
var repoPathRE = regexp.MustCompile(`^[A-Za-z0-9_.@-]+(?:/[A-Za-z0-9_.@-]+)*/$|^[A-Za-z0-9_.@-]+(?:/[A-Za-z0-9_.@-]+)+$`)

// isPathShaped reports whether a files[] token names a repo path rather than a
// placeholder: wholly slash-separated path characters, not every segment a
// single character — "go/internal/core", "docs/x.md", "skills/audit", "go/" and
// "go/internal/x/y.go" declare a surface while "N/A", "w/o", "I/O", "TBD" and a
// bare "role.go" do not.
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

// pathInProseRE finds slash-separated path tokens inside prose.
var pathInProseRE = regexp.MustCompile(`[A-Za-z0-9_.@-]+(?:/[A-Za-z0-9_.@-]+)+/?`)

// mentionSkip are the fields the mention walk never reads: the declared surface
// (judged on its own) and machine-written provenance/routing state. Every field
// the console router stamps starts with "routed_" (inboxmover/lifecycle
// route.go), so skipMention matches that prefix rather than copying its names.
var mentionSkip = map[string]bool{
	"files": true, "continuation": true, "route": true, "injected_by": true,
}

// skipMention reports whether the walk skips field k.
func skipMention(k string) bool {
	return mentionSkip[k] || strings.HasPrefix(k, "routed_")
}

// maxMentions bounds the walk so a runaway record cannot make routing costly.
const maxMentions = 64

// mentionedFiles returns the distinct FILE paths (a last segment with an
// extension) the record's author-written fields name, in walk order, at most
// maxMentions. Directory mentions are context, not surface: "the stall shows
// in go/internal/core" names no file a lane would change.
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

// isFileSpelling reports whether p's last segment carries an extension after
// its first character ("runner.go" yes; "core", ".evolve", "evolve/" no).
func isFileSpelling(p string) bool {
	last := p[strings.LastIndex(p, "/")+1:]
	return len(last) > 1 && strings.Contains(last[1:], ".")
}

// sortedKeys keeps the walk deterministic.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
	it.DeliverableKind = clean(it.DeliverableKind)
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
