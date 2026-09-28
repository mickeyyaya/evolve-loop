package core

import (
	"context"
	"reflect"
	"testing"
)

func TestWorktreeWritablePaths_OnlyTheDebuggerOfAFleetRebaseMayWriteItsConflicts(t *testing.T) {
	conflicts := []string{"go/.apicover-enforce"}
	needed, reclassified := string(CodeGitFleetRebaseNeeded), string(CodeGitFleetRebaseConflict)
	for name, tc := range map[string]struct {
		next Phase
		cs   CycleState
		want []string
	}{
		"the debugger of a fleet rebase that conflicted":   {PhaseDebugger, CycleState{ShipRecoveryCode: needed, ShipRecoveryConflicts: conflicts}, conflicts},
		"the debugger of a reclassified conflict":          {PhaseDebugger, CycleState{ShipRecoveryCode: reclassified, ShipRecoveryConflicts: conflicts}, conflicts},
		"a build in the same recovery":                     {PhaseBuild, CycleState{ShipRecoveryCode: needed, ShipRecoveryConflicts: conflicts}, nil},
		"the debugger of another ship error":               {PhaseDebugger, CycleState{ShipRecoveryCode: string(CodeWorktreeResolve), ShipRecoveryConflicts: conflicts}, nil},
		"the debugger of a rebase with nothing conflicted": {PhaseDebugger, CycleState{ShipRecoveryCode: needed}, nil},
	} {
		if got := worktreeWritablePaths(tc.next, tc.cs); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: writable = %v, want %v", name, got, tc.want)
		}
	}
}

func TestResumePath_TheDebuggerOfAFleetRebaseMayWriteItsConflicts(t *testing.T) {
	conflicts := []string{"go/.apicover-enforce"}
	st := &fakeStorage{cycleState: CycleState{CycleID: 1725, Phase: string(PhaseDebugger), ShipRecoveryCode: string(CodeGitFleetRebaseNeeded), ShipRecoveryConflicts: conflicts}}
	runners := buildRunners(map[Phase]string{})
	dbg := &fakeRunner{name: string(PhaseDebugger), verdict: VerdictPASS}
	runners[PhaseDebugger] = dbg
	o := NewOrchestrator(st, &fakeLedger{}, runners)

	_, _ = o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: t.TempDir()}, &ResumePoint{Phase: string(PhaseDebugger), CycleID: 1725})

	if len(dbg.requests) == 0 {
		t.Fatal("the resumed cycle never dispatched its debugger")
	}
	if got := dbg.requests[0].WorktreeWritablePaths; !reflect.DeepEqual(got, conflicts) {
		t.Fatalf("resumed debugger request writable = %v, want %v", got, conflicts)
	}
}
