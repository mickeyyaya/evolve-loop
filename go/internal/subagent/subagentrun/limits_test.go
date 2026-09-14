package subagentrun

// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 16 §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (signalcenter/limits_test.go
// idiom; comments inside a function count, its doc comment does not).

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/structure"
)

func TestLimits_FunctionsFilesAndNestingStayWithinTheBar(t *testing.T) {
	if err := structure.CheckLimits("."); err != nil {
		t.Fatal(err)
	}
}
