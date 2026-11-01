package reportdoc

import (
	"reflect"
	"testing"
)

func TestSeverityCounts_ReadsEachShapeOfAReviewReport(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		report string
		want   SeverityCounts
	}{
		{"none", "## Findings\nNone.\n## Verdict\nPASS\n", SeverityCounts{}},
		{"tagged lines", "## Findings\n- [HIGH] a.go:1 — x — y\n1. [LOW] b.md:2 — z\n[CRITICAL] c.go:3 — w\n", SeverityCounts{Critical: 1, High: 1, Low: 1}},
		{"major and minor fold into high and low", "## Findings\n[MAJOR] a.go:1 — x\n[MINOR] a.go:2 — y\n[MEDIUM] a.go:3 — z\n", SeverityCounts{High: 1, Medium: 1, Low: 1}},
		{"identified headings", "## Issues\n### H1 (HIGH) — a gap\nbody\n", SeverityCounts{High: 1}},
		{"summary table", "### Severity Summary\n| Severity | Count |\n|---|---|\n| CRITICAL | 0 |\n| HIGH     | 2 |\n| MEDIUM | 1 |\n", SeverityCounts{High: 2, Medium: 1}},
		{"counts line", "docs-review: 1 HIGH, 2 MEDIUM, 0 LOW; ste-lint 0\n", SeverityCounts{High: 1, Medium: 2}},
		{"the larger of the listed and the declared count", "[HIGH] a.go:1 — x\n| HIGH | 0 |\ndocs-review: 3 HIGH, 0 LOW\n", SeverityCounts{High: 3}},
		{"an earlier larger declared count survives and declared major and minor fold", "| HIGH | 3 |\n| MAJOR | 2 |\ndocs-review: 0 HIGH, 1 MINOR\n", SeverityCounts{High: 3, Low: 1}},
		{"one count in prose is not a counts line", "The fix closes 1 HIGH of round 1.\n", SeverityCounts{}},
		{"a fenced template is not a finding", "```\n[CRITICAL|MAJOR|MINOR] file:line\n[HIGH] example\n```\n", SeverityCounts{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CountSeverities(tc.report); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("CountSeverities = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestSeverityCounts_BlockingCountsOnlyCriticalAndHigh(t *testing.T) {
	t.Parallel()
	if got := (SeverityCounts{Critical: 1, High: 2, Medium: 5, Low: 7}).Blocking(); got != 3 {
		t.Errorf("Blocking = %d, want 3", got)
	}
}
