package signalcenter

// limits_test.go — the clean-code limits the design promises for this package
// (ADR-0101 decision 9; design §11), enforced by a test rather than by review:
// every function < 50 lines, nesting depth ≤ 4, every file < 800 lines.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/structure"
)

func TestLimits_FunctionsFilesAndNestingStayWithinTheBar(t *testing.T) {
	t.Parallel()
	if err := structure.CheckLimits("."); err != nil {
		t.Fatal(err)
	}
}
