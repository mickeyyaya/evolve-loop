package specrunner

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestNew_ThreadsRetractOnFailFromTheSpec(t *testing.T) {
	t.Parallel()
	for _, declared := range []bool{true, false} {
		p := New(phasespec.PhaseSpec{Name: "test-amplification", RetractOnFail: declared}, Config{})
		if got := p.RetractsOnFail(); got != declared {
			t.Errorf("spec retract_on_fail=%t, runner RetractsOnFail()=%t", declared, got)
		}
	}
}
