package evalgate

import (
	"strings"
	"testing"
)

func TestLintMonotonicBinaryTarget(t *testing.T) {
	cases := []struct {
		name     string
		class    string
		criteria []string
		want     int
	}{
		{"operator absolute target fires", "task-contract-design", []string{"prune the inbox backlog to <=25 items"}, 1},
		{"prose absolute target fires", "task-contract-design", []string{"reduce the backlog to at most 25 items"}, 1},
		{"monotonic-tagged class fires", "backlog-monotonic", []string{"prune the inbox backlog to under 25 items"}, 1},
		{"one of two criteria fires", "task-contract-design", []string{"prune the inbox backlog to <=25 items", "the ship gate stays enforce"}, 1},
		{"direction+floor phrasing is compliant", "task-contract-design", []string{"reduce the inbox backlog by >=50 items, requeueing the remainder with the delta"}, 0},
		{"non-monotonic class exempt", "pipeline-architecture", []string{"prune the inbox backlog to <=25 items"}, 0},
		{"no class exempt", "", []string{"prune the inbox backlog to <=25 items"}, 0},
		{"empty criteria", "task-contract-design", nil, 0},
		{"non-count criteria", "task-contract-design", []string{"the ship gate stays enforce", "docs/operations/ gains a rationale section"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := LintMonotonicBinaryTarget(tc.class, tc.criteria)
			if len(got) != tc.want {
				t.Fatalf("LintMonotonicBinaryTarget(%q, %v) = %v (%d findings), want %d", tc.class, tc.criteria, got, len(got), tc.want)
			}
			for _, f := range got {
				if !strings.Contains(f, "direction+floor") {
					t.Errorf("finding %q does not name the direction+floor remedy", f)
				}
			}
		})
	}
}
