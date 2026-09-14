package core

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"
	"github.com/mickeyyaya/evolve-loop/go/internal/textcap"
)

// ADR-0103 unit 04: truncateGoal and maxGoalTextChars are CONSUMERS of the
// advisor's cap — the judge's goal section and the task-recall digest render
// the advisor's bound with textcap's rule, so neither core facade can silently
// re-implement either.
func TestTruncateGoal_ProjectsTheAdvisorCap(t *testing.T) {
	long := strings.Repeat("g", maxGoalTextChars+7)
	if got, want := truncateGoal("  "+long+"  "), advisor.TruncateGoal(long); got != want {
		t.Fatalf("truncateGoal must be the advisor's cap:\n got %q\nwant %q", got[len(got)-20:], want[len(want)-20:])
	}
	if got, want := truncateGoal(long), textcap.TruncateRunes(long, advisor.MaxGoalTextRunes); got != want {
		t.Fatal("the advisor's cap is textcap's rule at the advisor's bound")
	}
	if maxGoalTextChars != advisor.MaxGoalTextRunes {
		t.Fatalf("maxGoalTextChars = %d, want the advisor's %d", maxGoalTextChars, advisor.MaxGoalTextRunes)
	}
	if advisor.MaxGoalTextRunes != 4000 {
		t.Fatalf("the advisor's bound = %d, want 4000", advisor.MaxGoalTextRunes)
	}
	// task_recall.go's exact call: the lesson digest capped at the advisor's bound.
	if got := truncateRunes(strings.Repeat("g", maxGoalTextChars+7), maxGoalTextChars); !strings.HasSuffix(got, " …[truncated]") || len([]rune(got)) != 4000+len([]rune(" …[truncated]")) {
		t.Fatalf("the recall digest cap renders the marker after 4000 runes: %d runes", len([]rune(got)))
	}
}
