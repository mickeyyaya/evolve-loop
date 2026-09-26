// Package evalgate holds the structural inter-phase gates behind the orchestrator's
// DeliverableReviewer seam. They block only on certain violations and fail open on
// ambiguity. See docs/architecture/packages/internal-evalgate.md.
package evalgate

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

// SelectedSlugs returns the sorted, distinct kebab-case slugs a scout report
// selects; nil means the report claims none.
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

type decisionTraceEntry struct {
	Slug          string `json:"slug"`
	FinalDecision string `json:"finalDecision"`
}

type decisionTraceDoc struct {
	DecisionTrace []decisionTraceEntry `json:"decisionTrace"`
}

var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

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

var nextH2RE = regexp.MustCompile(`(?m)^## `)

// slugLineRE matches a "- **Slug:** <kebab>" bullet (bullet * or -). The slug may
// be backticked, the form real reports also emit. That widening is a deliberate
// capability increase: measured 2026-09-15, it changes the SelectedSlugs union on
// 6 of 84 cycle-16xx reports, and 2 become newly blockable at Gate A because their
// eval is absent: cycle-1664 (settle-wait-stability-shortcircuit) and cycle-1669
// (verdict-tool-call-claudep). TestSlugLineWideningIsNotBlockingNeutral pins it.
var slugLineRE = regexp.MustCompile(`(?m)^[*\-]\s*\*\*Slug:\*\*\s*` + "`" + `?([a-z0-9][a-z0-9-]*)`)

// htmlCommentRE lets a "<!-- none selected -->" placeholder read as an empty section.
var htmlCommentRE = regexp.MustCompile(`(?s)<!--.*?-->`)

// selectedTasksBody is the one "## Selected Tasks" bound that the parse and the
// parse-miss signal share, so a miss is judged over exactly the span parsed.
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

// SelectedTasksParseMiss reports a "## Selected Tasks" section that has content
// but yields no slug: format drift, not convergence.
func SelectedTasksParseMiss(report string) bool {
	body, ok := selectedTasksBody(report)
	if !ok {
		return false
	}
	if strings.TrimSpace(htmlCommentRE.ReplaceAllString(body, "")) == "" {
		return false
	}
	return len(slugBullets(body)) == 0
}

func fencedAfterHeading(report, heading string) (string, bool) {
	hi := strings.Index(report, heading)
	if hi < 0 {
		return "", false
	}
	rest := report[hi+len(heading):]
	// Anchored at a line start so an inline triple-backtick span in prose is not a fence.
	open := strings.Index(rest, "\n```")
	if open < 0 {
		return "", false
	}
	after := rest[open+len("\n```"):]
	// Skip the opening fence's language tag.
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
