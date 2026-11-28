package incidentkb

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	CoverageIndexRel   = RelDir + "/REGRESSION-COVERAGE-INDEX.md"
	coverageSummary    = "coverage-summary"
	coverageMapHeading = "## Coverage map"
	coverageCells      = 5
)

var (
	ErrCoverageRow    = errors.New("coverage map row")
	errConflictMarker = errors.New("a conflict marker in the coverage map")
	incidentLinkRE    = regexp.MustCompile(`^\[([^\]]+)\]\([^)]+\)`)
	conflictMarkerRE  = regexp.MustCompile(`^(<{7}|={7}$|>{7})`)
)

type CoverageRowError struct {
	Line   int
	Reason error
}

func (e *CoverageRowError) Error() string {
	return fmt.Sprintf("%v: line %d: %v", ErrCoverageRow, e.Line, e.Reason)
}

func (e *CoverageRowError) Unwrap() error { return ErrCoverageRow }

type CoverageCounts struct {
	Incidents  int
	Rows       int
	Covered    int
	Partial    int
	None       int
	Untestable int
}

func CoverageCountsOf(doc []byte) (CoverageCounts, error) {
	lines := strings.Split(string(doc), "\n")
	start := coverageMapStart(lines)
	if start < 0 {
		return CoverageCounts{}, fmt.Errorf("%w: no %q section", ErrCoverageRow, coverageMapHeading)
	}
	var counts CoverageCounts
	names := map[string]bool{}
	tableLine := 0
	for i := start + 1; i < len(lines) && !strings.HasPrefix(lines[i], "## "); i++ {
		line := lines[i]
		if conflictMarkerRE.MatchString(line) {
			return CoverageCounts{}, &CoverageRowError{Line: i + 1, Reason: errConflictMarker}
		}
		if !strings.HasPrefix(line, "|") {
			continue
		}
		if tableLine++; tableLine <= 2 {
			continue
		}
		name, status, err := coverageRow(line)
		if err != nil {
			return CoverageCounts{}, &CoverageRowError{Line: i + 1, Reason: err}
		}
		counts.add(status)
		names[name] = true
	}
	counts.Incidents = len(names)
	return counts, nil
}

func coverageMapStart(lines []string) int {
	for i, line := range lines {
		if strings.TrimSpace(line) == coverageMapHeading {
			return i
		}
	}
	return -1
}

type coverageStatus struct {
	glyph string
	label string
	tally func(*CoverageCounts) *int
}

var coverageStatuses = []coverageStatus{
	{"✅", "covered (a test fails if the bug comes back)", func(c *CoverageCounts) *int { return &c.Covered }},
	{"🟡", "partial (related test, doesn't pin the exact mode)", func(c *CoverageCounts) *int { return &c.Partial }},
	{"❌", "none (no regression test)", func(c *CoverageCounts) *int { return &c.None }},
	{"⛔", "untestable (external infra / live CLI)", func(c *CoverageCounts) *int { return &c.Untestable }},
}

func coverageRow(line string) (string, coverageStatus, error) {
	cells := splitCells(line)
	if len(cells) != coverageCells {
		return "", coverageStatus{}, fmt.Errorf("%d cells, want %d (escape a pipe inside a cell as \\|)", len(cells), coverageCells)
	}
	m := incidentLinkRE.FindStringSubmatch(cells[0])
	if m == nil {
		return "", coverageStatus{}, fmt.Errorf("the incident cell %q does not start with a link [name](record.md)", cells[0])
	}
	glyphs := make([]string, 0, len(coverageStatuses))
	for _, s := range coverageStatuses {
		if cells[3] == s.glyph {
			return m[1], s, nil
		}
		glyphs = append(glyphs, s.glyph)
	}
	return "", coverageStatus{}, fmt.Errorf("the coverage cell %q is not one of %s", cells[3], strings.Join(glyphs, " "))
}

func splitCells(line string) []string {
	inner := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "|"), "|")
	var cells []string
	var cell strings.Builder
	for i := 0; i < len(inner); i++ {
		switch {
		case inner[i] == '\\' && i+1 < len(inner) && inner[i+1] == '|':
			cell.WriteString(`\|`)
			i++
		case inner[i] == '|':
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
		default:
			cell.WriteByte(inner[i])
		}
	}
	return append(cells, strings.TrimSpace(cell.String()))
}

func (c *CoverageCounts) add(status coverageStatus) {
	c.Rows++
	*status.tally(c)++
}

func RenderCoverageSummary(doc []byte) ([]byte, CoverageCounts, error) {
	counts, err := CoverageCountsOf(doc)
	if err != nil {
		return nil, CoverageCounts{}, err
	}
	text := string(doc)
	span, err := findMarkerSpan(text, coverageSummary)
	if err != nil {
		return nil, CoverageCounts{}, err
	}
	return []byte(text[:span.contentS] + "\n" + counts.table() + "\n" + text[span.contentE:]), counts, nil
}

func (c CoverageCounts) table() string {
	var b strings.Builder
	fmt.Fprintf(&b, "| | Count |\n|---|---|\n| Incidents mapped | %d |\n| Distinct failure modes (coverage-map rows) | %d |", c.Incidents, c.Rows)
	for _, s := range coverageStatuses {
		fmt.Fprintf(&b, "\n| %s %s | %d |", s.glyph, s.label, *s.tally(&c))
	}
	return b.String()
}
