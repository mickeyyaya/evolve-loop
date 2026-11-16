package core

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestPrescreenFleetRebase_ALaneWithNoWorktreeFallsThroughToTheRebase(t *testing.T) {
	t.Parallel()
	var errOut bytes.Buffer
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, nil, WithStderr(&errOut))

	conflict, landed := o.prescreenFleetRebase(context.Background(), 7, CycleState{})

	if conflict || landed || errOut.Len() != 0 {
		t.Errorf("prescreen of a lane with no worktree = (%v, %v) with %q, want (false, false) and no output", conflict, landed, errOut.String())
	}
}

func TestPrescreenFleetRebase_APreScreenErrorFallsThroughWithoutAPredictedConflict(t *testing.T) {
	t.Parallel()
	var errOut bytes.Buffer
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, nil, WithStderr(&errOut))

	conflict, landed := o.prescreenFleetRebase(context.Background(), 7, CycleState{ActiveWorktree: t.TempDir()})

	if conflict || landed || !strings.Contains(errOut.String(), "pre-screen error") {
		t.Errorf("prescreen of a worktree git cannot read = (%v, %v) with %q, want (false, false) and a pre-screen error line", conflict, landed, errOut.String())
	}
}
