// Package evalgate implements two structural inter-phase gates that replace
// prose/trust contracts with verified checks, mounted at the orchestrator's
// existing per-phase DeliverableReviewer seam (core.WithReviewer):
//
//   - Gate A (materialization): after scout, every slug it SELECTED must have a
//     real .evolve/evals/<slug>.md file on disk (cycle-166: selected slugs with
//     no eval files → audit FAIL after build tokens were already spent).
//   - Gate B (predicate quality): after tdd, the selected slugs' eval predicates
//     must not be tautological no-ops (cycle-204), via evalqualitycheck.
//
// Both gate ONLY on CERTAIN violations (a stat'd-missing file, a definite
// tautology) and fail OPEN on any ambiguity (parse failure, zero slugs,
// advisory WARN), so enforce-by-default never false-blocks a healthy cycle.
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

// slugLineRE matches a "- **Slug:** <kebab>" bullet (bullet char * or -).
var slugLineRE = regexp.MustCompile(`(?m)^[*\-]\s*\*\*Slug:\*\*\s*([a-z0-9][a-z0-9-]*)`)

func selectedTaskSlugs(report string) []string {
	const heading = "## Selected Tasks"
	start := strings.Index(report, heading)
	if start < 0 {
		return nil
	}
	body := report[start+len(heading):]
	if loc := nextH2RE.FindStringIndex(body); loc != nil {
		body = body[:loc[0]]
	}
	var out []string
	for _, m := range slugLineRE.FindAllStringSubmatch(body, -1) {
		out = append(out, m[1])
	}
	return out
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
