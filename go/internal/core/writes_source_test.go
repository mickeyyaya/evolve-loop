package core

import "testing"

func TestPhaseRequest_WritesSource_TheWorktreeFenceDecidesWhoWrites(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	conflicted := CycleState{ShipRecoveryCode: string(CodeGitFleetRebaseNeeded), ShipRecoveryConflicts: []string{"go/internal/x.go"}}
	otherShipError := CycleState{ShipRecoveryCode: string(CodeWorktreeResolve), ShipRecoveryConflicts: []string{"go/internal/x.go"}}
	cases := []struct {
		name  string
		phase Phase
		cs    CycleState
		want  bool
	}{
		{"build", PhaseBuild, CycleState{}, true},
		{"tdd", PhaseTDD, CycleState{}, true},
		{"the debugger resolving a fleet-rebase conflict", PhaseDebugger, conflicted, true},
		{"scout", PhaseScout, CycleState{}, false},
		{"triage", PhaseTriage, CycleState{}, false},
		{"audit", PhaseAudit, CycleState{}, false},
		{"the decision-only debugger", PhaseDebugger, otherShipError, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := o.withWorktreeFence(PhaseRequest{Worktree: "/wt/cycle-1"}, tc.phase, tc.cs)
			if got := req.WritesSource(); got != tc.want {
				t.Errorf("WritesSource() = %v, want %v (read_only=%v writable=%v)", got, tc.want, req.WorktreeReadOnly, req.WorktreeWritablePaths)
			}
		})
	}
}
