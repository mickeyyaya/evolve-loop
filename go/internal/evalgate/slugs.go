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

var slugLineRE = regexp.MustCompile(`(?m)^[*\-]\s*\*\*Slug:\*\*\s*` + "`" + `?([a-z0-9][a-z0-9-]*)`)

var htmlCommentRE = regexp.MustCompile(`(?s)<!--.*?-->`)

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
	open := strings.Index(rest, "\n```")
	if open < 0 {
		return "", false
	}
	after := rest[open+len("\n```"):]
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
