package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestApicoverNamed_CodeLadderRedesign(t *testing.T) {
	t.Parallel()
	if CodeLadderRedesign != redesignCode {
		t.Fatalf("CodeLadderRedesign = %q, want %q", CodeLadderRedesign, redesignCode)
	}
	if m, ok := signalcenter.IsRegistered(CodeLadderRedesign); !ok || m != signalcenter.ModuleLadder {
		t.Fatalf("CodeLadderRedesign registered=%v module=%q, want the ladder module", ok, m)
	}
}
