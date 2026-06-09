package core

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/research"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// fakeKB is a test double for research.KB capturing the query it received.
type fakeKB struct {
	lessons  []research.Lesson
	err      error
	gotQuery research.Query
}

func (f *fakeKB) Lookup(_ context.Context, q research.Query) ([]research.Lesson, error) {
	f.gotQuery = q
	return f.lessons, f.err
}

func TestRecallForPlan(t *testing.T) {
	t.Parallel()
	t.Run("no KB wired returns empty", func(t *testing.T) {
		o := &Orchestrator{}
		reason, lessons := o.recallForPlan(context.Background(), []FailedRecord{{Summary: "x"}})
		if reason != "" || lessons != nil {
			t.Errorf("no KB must yield empty recall, got %q/%v", reason, lessons)
		}
	})

	t.Run("no history returns empty", func(t *testing.T) {
		o := &Orchestrator{kb: &fakeKB{}}
		reason, lessons := o.recallForPlan(context.Background(), nil)
		if reason != "" || lessons != nil {
			t.Errorf("no history must yield empty recall, got %q/%v", reason, lessons)
		}
	})

	t.Run("latest failure drives the query and returns lesson digests", func(t *testing.T) {
		kb := &fakeKB{lessons: []research.Lesson{
			{ID: "inst-L001", Pattern: "egps-red", PreventiveAction: "Run the suite first."},
		}}
		o := &Orchestrator{kb: kb}
		history := []FailedRecord{
			{Cycle: 1, Classification: "code-build-fail", Summary: "older"},
			{Cycle: 2, Classification: "code-audit-fail", Summary: "EGPS red_count=3"},
		}
		reason, lessons := o.recallForPlan(context.Background(), history)
		if reason != "EGPS red_count=3" {
			t.Errorf("reason = %q, want the LATEST failure summary", reason)
		}
		if kb.gotQuery.Consequence != "code-audit-fail" {
			t.Errorf("query consequence = %q, want latest classification", kb.gotQuery.Consequence)
		}
		if len(lessons) != 1 || !strings.Contains(lessons[0], "inst-L001") {
			t.Errorf("lessons = %v, want the digest of inst-L001", lessons)
		}
	})

	t.Run("KB error degrades to reason-without-lessons", func(t *testing.T) {
		o := &Orchestrator{kb: &fakeKB{err: context.DeadlineExceeded}}
		reason, lessons := o.recallForPlan(context.Background(), []FailedRecord{{Summary: "boom", Classification: "code-audit-fail"}})
		if reason != "boom" {
			t.Errorf("reason = %q, want the summary even on KB error", reason)
		}
		if lessons != nil {
			t.Errorf("lessons must be nil on KB error, got %v", lessons)
		}
	})
}

func TestWriteRecallMemory(t *testing.T) {
	t.Parallel()
	t.Run("empty recall renders nothing", func(t *testing.T) {
		var b strings.Builder
		writeRecallMemory(&b, router.RouteInput{})
		if b.Len() != 0 {
			t.Errorf("empty recall must render nothing, got %q", b.String())
		}
	})
	t.Run("reason + lessons render under a recall heading", func(t *testing.T) {
		var b strings.Builder
		writeRecallMemory(&b, router.RouteInput{
			LastReason: "EGPS red_count=3",
			Lessons:    []string{"inst-L001 (egps-red): Run the suite first"},
		})
		out := b.String()
		for _, want := range []string{"Recall memory", "EGPS red_count=3", "inst-L001"} {
			if !strings.Contains(out, want) {
				t.Errorf("recall output missing %q; got:\n%s", want, out)
			}
		}
	})
}
