package incidentkb

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrMissingMarker      = errors.New("missing generated marker")
	ErrOverlappingMarkers = errors.New("overlapping generated markers")
)

type markerSpan struct {
	part     string
	contentS int
	contentE int
}

func markerTokens(part string) (string, string) {
	return "<!-- GENERATED:" + part + " BEGIN -->", "<!-- GENERATED:" + part + " END -->"
}

func findMarkerSpan(doc, part string) (markerSpan, error) {
	beginTok, endTok := markerTokens(part)
	bIdx := strings.Index(doc, beginTok)
	if bIdx < 0 {
		return markerSpan{}, fmt.Errorf("%w: %s begin", ErrMissingMarker, part)
	}
	contentS := bIdx + len(beginTok)
	eRel := strings.Index(doc[contentS:], endTok)
	if eRel < 0 {
		return markerSpan{}, fmt.Errorf("%w: %s end", ErrMissingMarker, part)
	}
	return markerSpan{part: part, contentS: contentS, contentE: contentS + eRel}, nil
}

func Render(doc []byte) ([]byte, error) {
	rec, err := Parse(doc)
	if err != nil {
		return nil, err
	}
	if problems := validateFrontmatter(rec, rec.ID); len(problems) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	text := string(doc)
	spans := make([]markerSpan, 0, len(generatedParts))
	for _, part := range generatedParts {
		sp, err := findMarkerSpan(text, part)
		if err != nil {
			return nil, err
		}
		spans = append(spans, sp)
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].contentS < spans[j].contentS })
	var b strings.Builder
	cursor := 0
	for _, sp := range spans {
		if sp.contentS < cursor {
			return nil, fmt.Errorf("%w: %s", ErrOverlappingMarkers, sp.part)
		}
		b.WriteString(text[cursor:sp.contentS])
		b.WriteByte('\n')
		b.WriteString(renderPart(rec, sp.part))
		b.WriteByte('\n')
		cursor = sp.contentE
	}
	b.WriteString(text[cursor:])
	return []byte(b.String()), nil
}

func renderPart(rec Record, part string) string {
	switch part {
	case "status":
		return renderStatus(rec)
	case "actions":
		return renderActions(rec.Actions)
	default:
		return renderEffectiveness(rec.Effectiveness)
	}
}

func renderStatus(rec Record) string {
	if rec.VerifiedAt != "" {
		return fmt.Sprintf("- **Status:** %s (verified_at: %s)", rec.Status, rec.VerifiedAt)
	}
	return fmt.Sprintf("- **Status:** %s", rec.Status)
}

func renderActions(actions []Action) string {
	if len(actions) == 0 {
		return "_No actions._"
	}
	var b strings.Builder
	b.WriteString("| ID | Type | Owner | Item | PR | Done | State |\n")
	b.WriteString("|---|---|---|---|---|---|---|")
	for _, a := range actions {
		pr := "-"
		if a.PR > 0 {
			pr = fmt.Sprintf("#%d", a.PR)
		}
		fmt.Fprintf(&b, "\n| %s | %s | %s | %s | %s | %s | %s |",
			escapeCell(a.ID),
			escapeCell(a.Type),
			escapeCell(a.Owner),
			escapeCell(a.Item),
			pr,
			escapeCell(a.Done),
			escapeCell(a.State),
		)
	}
	return b.String()
}

func renderEffectiveness(eff Effectiveness) string {
	var b strings.Builder
	code := eff.Metric.Code
	if code == "" {
		code = "-"
	}
	unit := eff.Metric.Unit
	if unit == "" {
		unit = "-"
	}
	since := eff.Metric.Since
	if since == "" {
		since = "-"
	}
	fmt.Fprintf(&b, "- **Metric:** `%s` (unit: `%s`, since: `%s`, filters: %s)\n", code, unit, since, formatFilters(eff.Metric.Filters))
	fmt.Fprintf(&b, "- **Baseline:** %d over %d %s(s) | **Target:** %d | **Checks:** %d\n", eff.Baseline.Value, eff.Baseline.Units, unit, eff.Target, eff.Checks)
	if len(eff.Results) == 0 {
		b.WriteString("- **Results:** none")
		return b.String()
	}
	b.WriteString("| Check | Observed | Verdict | At |\n")
	b.WriteString("|---|---|---|---|")
	for _, r := range eff.Results {
		fmt.Fprintf(&b, "\n| %d | %d | %s | %s |", r.Check, r.Observed, escapeCell(r.Verdict), escapeCell(r.At))
	}
	return b.String()
}

func formatFilters(filters []Filter) string {
	if len(filters) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(filters))
	for _, f := range filters {
		parts = append(parts, fmt.Sprintf("`%s %s %s`", f.Field, f.Op, f.Value))
	}
	return strings.Join(parts, ", ")
}

func escapeCell(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "|", `\|`)
}
