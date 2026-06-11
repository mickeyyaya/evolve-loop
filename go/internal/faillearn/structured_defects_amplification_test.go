package faillearn

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestStructuredDefectsOmitSingleSummaryDuplicate(t *testing.T) {
	ev := FailureEvent{
		Cycle:       282,
		FailedPhase: "audit",
		Scope:       ScopePhase,
		Summary:     "bridge launch stalled",
		Defects:     []string{"bridge launch stalled"},
		Now:         time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
	}

	if got := structuredDefects(ev); got != nil {
		t.Fatalf("structuredDefects() = %#v, want nil for single duplicate defect", got)
	}

	_, body := RenderLessonYAML(ev)
	if bytes.Contains(body, []byte("defects:")) {
		t.Fatalf("RenderLessonYAML emitted duplicate-only defects:\n%s", string(body))
	}
}

func TestStructuredDefectsRetainIndependentDefects(t *testing.T) {
	ev := FailureEvent{
		Summary: "pipeline failed",
		Defects: []string{
			"bridge launch stalled",
			"artifact verifier rejected malformed deliverable",
		},
	}

	got := structuredDefects(ev)
	if len(got) != 2 {
		t.Fatalf("structuredDefects() length = %d, want 2: %#v", len(got), got)
	}
	joined := strings.Join(got, "\n")
	for _, want := range ev.Defects {
		if !strings.Contains(joined, want) {
			t.Fatalf("structuredDefects() missing %q in %#v", want, got)
		}
	}
}
