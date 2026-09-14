// Package evalgate implements the structural inter-phase gates that replace
// prose/trust contracts with verified checks, mounted at the orchestrator's
// existing per-phase DeliverableReviewer seam (core.WithReviewer):
//
//   - Gate A (materialization): after scout, every slug it SELECTED must have a
//     real .evolve/evals/<slug>.md file on disk (cycle-166: selected slugs with
//     no eval files → audit FAIL after build tokens were already spent).
//   - Gate B (predicate quality): after tdd, the selected slugs' eval predicates
//     must not be tautological no-ops (cycle-204), via evalqualitycheck.
//   - Gate C (floor binding): after tdd, EGPS floor predicates must bind only
//     floors triage COMMITTED this cycle (cycle-280), via triagecap.
//   - Gate D (flaky predicate shape): after tdd, the authored predicates are
//     linted for shapes that flake under fleet load (cycles 1173/1175/1178).
//     ADVISORY-ONLY — see flakyshape.go for why it can never block.
//
// The BLOCKING gates (A/B/C) gate ONLY on CERTAIN violations (a stat'd-missing
// file, a definite tautology, a proven deferred-floor binding) and fail OPEN on
// any ambiguity (parse failure, zero slugs, advisory WARN), so enforce-by-default
// never false-blocks a healthy cycle. Gate D never blocks at all.
package evalgate

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

// SelectedSlugs extracts the slugs scout SELECTED from a scout-report.md body,
// unioning two sources for robustness:
//
//   - the machine-authored "## Decision Trace" JSON (objects whose
//     finalDecision == "selected"), the most reliable source;
//   - the "## Selected Tasks" prose ("- **Slug:** <kebab-case>"), a fallback
//     for reports that omit or malform the trace.
//
// Slugs are restricted to kebab-case ([a-z0-9-]); the result is deduped and
// sorted. An unparseable report yields an empty slice — callers treat empty as
// "no claim" (fail-open), never as "zero work".
func SelectedSlugs(report string) []string {
	set := map[string]struct{}{}
	for _, s := range decisionTraceSelected(report) {
		set[s] = struct{}{}
	}
	for _, s := range selectedTaskSlugs(report) {
		set[s] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// decisionTraceEntry mirrors one element of the scout "## Decision Trace" JSON.
type decisionTraceEntry struct {
	Slug          string `json:"slug"`
	FinalDecision string `json:"finalDecision"`
}

type decisionTraceDoc struct {
	DecisionTrace []decisionTraceEntry `json:"decisionTrace"`
}

var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// decisionTraceSelected returns slugs whose finalDecision is "selected" from the
// first fenced code block following the "## Decision Trace" heading.
func decisionTraceSelected(report string) []string {
	block, ok := fencedAfterHeading(report, "## Decision Trace")
	if !ok {
		return nil
	}
	var doc decisionTraceDoc
	if err := json.Unmarshal([]byte(block), &doc); err != nil {
		return nil
	}
	var out []string
	for _, e := range doc.DecisionTrace {
		if e.FinalDecision == "selected" && slugRE.MatchString(e.Slug) {
			out = append(out, e.Slug)
		}
	}
	return out
}

// selectedTasksSection bounds the "## Selected Tasks" section: from its heading
// to the next "## " heading (or EOF).
var nextH2RE = regexp.MustCompile(`(?m)^## `)

// slugLineRE matches a "- **Slug:** <kebab>" bullet (bullet char * or -). The
// slug may be wrapped in backticks: the persona template at
// agents/evolve-scout-reference.md declares the bare form, but real reports emit
// the backticked one about as often (12 of the 84 cycle-16xx reports carrying a
// "## Selected Tasks" section, measured 2026-09-15). Accepting both is what
// keeps SelectedTasksParseMiss a signal instead of a WARN on one cycle in seven.
//
// The widening is not blocking-neutral, and saying so cost this cycle an audit
// round: the first attempt counted "## Decision Trace" HEADINGS and reported the
// result as union equality, which is a different claim. Re-measured 2026-09-15
// by running this parser over the same 84-report corpus and diffing against the
// pre-widening pattern: the union SelectedSlugs returns differs on 6 of the 84
// reports — cycle-1605, -1610, -1634, -1664, -1665 and -1669, each going from
// empty to non-empty. All six do carry a "## Decision Trace" heading, but each
// states its selection as a "selected_tasks" string array, a shape
// decisionTraceSelected does not read, so the trace contributes nothing and the
// bullet is the only source of the slug.
//
// On 2 of those 6 the newly parsed slug has no eval file at either resolution
// evalFilePath checks, so Gate A blocks at StageEnforce where it previously
// fail-opened: cycle-1664 (settle-wait-stability-shortcircuit) and cycle-1669
// (verdict-tool-call-claudep). That is a deliberate capability increase, not a
// regression — catching a selected slug whose eval was never written is this
// gate's cycle-166 job, and on both of those cycles the eval really is absent.
// TestSlugLineWideningIsNotBlockingNeutral pins the effect by running it, so the
// next reader re-derives this paragraph instead of trusting it.
var slugLineRE = regexp.MustCompile(`(?m)^[*\-]\s*\*\*Slug:\*\*\s*` + "`" + `?([a-z0-9][a-z0-9-]*)`)

// htmlCommentRE matches an HTML comment, including a multi-line one, so a
// "<!-- none selected this cycle -->" placeholder reads as an EMPTY section
// rather than as content the parser failed on.
var htmlCommentRE = regexp.MustCompile(`(?s)<!--.*?-->`)

// selectedTasksBody returns the bounded body of the "## Selected Tasks" section
// — from the end of its heading to the next "## " heading (or EOF) — and whether
// that heading is present at all.
//
// It is the SINGLE bound shared by selectedTaskSlugs and SelectedTasksParseMiss.
// Two copies of this arithmetic could drift apart, and a miss signal computed
// over a different span than the parse it reports on would accuse a section the
// parser never read — the same class of defect this file is fixing.
func selectedTasksBody(report string) (string, bool) {
	const heading = "## Selected Tasks"
	start := strings.Index(report, heading)
	if start < 0 {
		return "", false
	}
	body := report[start+len(heading):]
	if loc := nextH2RE.FindStringIndex(body); loc != nil {
		body = body[:loc[0]]
	}
	return body, true
}

// slugBullets returns every slug captured by slugLineRE in an already-bounded
// section body.
func slugBullets(body string) []string {
	var out []string
	for _, m := range slugLineRE.FindAllStringSubmatch(body, -1) {
		out = append(out, m[1])
	}
	return out
}

func selectedTaskSlugs(report string) []string {
	body, ok := selectedTasksBody(report)
	if !ok {
		return nil
	}
	return slugBullets(body)
}

// SelectedTasksParseMiss reports whether a scout report's "## Selected Tasks"
// section is present and carries real content from which ZERO slugs parsed —
// format drift the parser could not read, as distinct from genuine convergence.
//
// SelectedSlugs returns nil for both shapes, so its callers cannot tell "nothing
// was claimed" (where fail-open is correct) from "something was claimed in a
// form we failed to read" (where fail-open silently drops the claim). That is
// cycle-1570: a section naming config-gate-default-policy-authority in prose
// rather than in a "- **Slug:**" bullet parsed to nil, Gate A's fail-open path
// let it through, and the missing eval surfaced three phases later as an audit
// H1. This function is that missing distinction.
//
// It is deliberately ADVISORY. Blocking every zero-slug report would false-block
// every converged cycle, which is precisely what this package's documented
// fail-open-on-ambiguity contract exists to prevent.
//
// True requires all three: the heading is present, the bounded body still holds
// non-whitespace content once HTML comments are stripped, and no slug bullet
// parses out of that body. Content after the next "## " heading belongs to
// another section and never counts, and a malformed "## Decision Trace" is out
// of scope by construction — this reads the Selected Tasks body only.
func SelectedTasksParseMiss(report string) bool {
	body, ok := selectedTasksBody(report)
	if !ok {
		return false // no section: nothing was claimed here
	}
	if strings.TrimSpace(htmlCommentRE.ReplaceAllString(body, "")) == "" {
		return false // an empty or comment-only section still claims no work
	}
	return len(slugBullets(body)) == 0
}

// fencedAfterHeading returns the contents of the first ``` fenced code block
// that follows heading in report (language tag on the opening fence is
// ignored), and whether one was found.
func fencedAfterHeading(report, heading string) (string, bool) {
	hi := strings.Index(report, heading)
	if hi < 0 {
		return "", false
	}
	rest := report[hi+len(heading):]
	// Require the opening fence at the start of a line ("\n```") so an inline
	// triple-backtick code span in prose can't be mistaken for a code block.
	open := strings.Index(rest, "\n```")
	if open < 0 {
		return "", false
	}
	after := rest[open+len("\n```"):]
	// Skip the language tag up to the end of the opening fence line.
	if nl := strings.IndexByte(after, '\n'); nl >= 0 {
		after = after[nl+1:]
	} else {
		return "", false
	}
	end := strings.Index(after, "```")
	if end < 0 {
		return "", false
	}
	return after[:end], true
}
