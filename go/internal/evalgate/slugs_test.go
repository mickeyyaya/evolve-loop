package evalgate

import (
	"reflect"
	"testing"
)

func TestSelectedSlugs(t *testing.T) {
	cases := []struct {
		name   string
		report string
		want   []string
	}{
		{
			name: "decision-trace only",
			report: "intro\n\n## Decision Trace\n```json\n" +
				`{"decisionTrace":[{"slug":"add-cache","finalDecision":"selected"},{"slug":"skip-me","finalDecision":"deferred"}]}` +
				"\n```\n",
			want: []string{"add-cache"},
		},
		{
			name: "selected-tasks prose only",
			report: "## Selected Tasks\n\n### Task 1: Cache\n- **Slug:** add-cache\n- **Type:** feature\n\n" +
				"### Task 2: Limit\n- **Slug:** rate-limit\n\n## Deferred\n- something\n",
			want: []string{"add-cache", "rate-limit"},
		},
		{
			name: "both sources union + dedupe",
			report: "## Selected Tasks\n- **Slug:** add-cache\n- **Slug:** prose-only\n\n" +
				"## Decision Trace\n```json\n" +
				`{"decisionTrace":[{"slug":"add-cache","finalDecision":"selected"},{"slug":"trace-only","finalDecision":"selected"}]}` +
				"\n```\n",
			want: []string{"add-cache", "prose-only", "trace-only"},
		},
		{
			name:   "convergence / empty report",
			report: "## Gap Analysis\nNothing to do.\n",
			want:   nil,
		},
		{
			name: "non-kebab slug in trace rejected",
			report: "## Decision Trace\n```json\n" +
				`{"decisionTrace":[{"slug":"Goal_Umbrella","finalDecision":"selected"}]}` +
				"\n```\n",
			want: nil,
		},
		{
			name:   "malformed decision-trace json falls back to empty",
			report: "## Decision Trace\n```json\n{not valid json\n```\n",
			want:   nil,
		},
		{
			name:   "selected-tasks section bounded by next heading",
			report: "## Selected Tasks\n- **Slug:** in-section\n\n## Deferred\n- **Slug:** not-counted\n",
			want:   []string{"in-section"},
		},
		{
			name: "rejected decisions excluded",
			report: "## Decision Trace\n```json\n" +
				`{"decisionTrace":[{"slug":"keep","finalDecision":"selected"},{"slug":"drop","finalDecision":"rejected"}]}` +
				"\n```\n",
			want: []string{"keep"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SelectedSlugs(c.report)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("SelectedSlugs()=%v, want %v", got, c.want)
			}
		})
	}
}

func TestFencedAfterHeading_FenceWithoutTrailingNewline(t *testing.T) {
	got, ok := fencedAfterHeading("intro\n\n## Decision Trace\n```", "## Decision Trace")
	if ok || got != "" {
		t.Fatalf("fencedAfterHeading()=%q, %v; want empty string, false", got, ok)
	}
}

func TestFencedAfterHeading_NoFenceAfterHeading(t *testing.T) {
	got, ok := fencedAfterHeading("intro\n\n## Decision Trace\nno fenced block here", "## Decision Trace")
	if ok || got != "" {
		t.Fatalf("fencedAfterHeading()=%q, %v; want empty string, false", got, ok)
	}
}

func TestFencedAfterHeading_MissingClosingFence(t *testing.T) {
	got, ok := fencedAfterHeading("intro\n\n## Decision Trace\n```json\n{\"slug\":\"x\"}\n", "## Decision Trace")
	if ok || got != "" {
		t.Fatalf("fencedAfterHeading()=%q, %v; want empty string, false", got, ok)
	}
}
