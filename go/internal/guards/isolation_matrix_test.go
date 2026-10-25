package guards

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
)

func TestIsolationMatrix_N11_GuardPhaseAllowsAnInCellTaskAndDeniesAnOutOfCellPhasePersonaTask(t *testing.T) {
	t.Parallel()
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 42, Phase: "build", ActiveAgent: "builder"})
	task := core.GuardInput{ToolName: "Task", ToolInput: map[string]any{"subagent_type": "evo:evolve-builder"}}
	var failures []error
	if dec := newPhaseWithEnv(s, false, inCellEnv).Decide(context.Background(), task); !dec.Allow {
		failures = append(failures, errors.New("guard phase denied a Task inside a phase cell: "+dec.Reason))
	}
	if dec := newPhaseWithEnv(s, false, noCellEnv).Decide(context.Background(), task); dec.Allow {
		failures = append(failures, errors.New("guard phase allowed a phase-persona Task outside a cell during cycle 42"))
	}
	note, err := isolation.Judge("N11", errors.Join(failures...))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(note)
}
