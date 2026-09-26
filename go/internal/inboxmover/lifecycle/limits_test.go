package lifecycle

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/structure"
)

func TestLimits_FunctionsFilesAndNestingStayWithinTheBar(t *testing.T) {
	if err := structure.CheckLimits("."); err != nil {
		t.Fatal(err)
	}
}
