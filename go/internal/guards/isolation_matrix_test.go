package guards

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
)

func TestIsolationMatrix_N11_GuardPhaseDeniesTaskDuringACycle(t *testing.T) {
	t.Parallel()
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 42, Phase: "build", ActiveAgent: "builder"})
	var failure error
	if dec := NewPhase(s, false).Decide(context.Background(), core.GuardInput{ToolName: "Task"}); dec.Allow {
		failure = errors.New("guard phase allowed Task during cycle 42; only the Agent tool name is denied")
	}
	note, err := isolation.Judge("N11", failure)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(note)
}
