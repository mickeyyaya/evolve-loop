package lifecycle

// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 06 §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (the failurelearning/
// signalcenter limits_test.go idiom; comments inside a function count, its doc
// comment does not). Promote (113 lines), the drain (114) and RecoverOrphans
// (52) were over the bar before the split.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/structure"
)

func TestLimits_FunctionsFilesAndNestingStayWithinTheBar(t *testing.T) {
	if err := structure.CheckLimits("."); err != nil {
		t.Fatal(err)
	}
}
