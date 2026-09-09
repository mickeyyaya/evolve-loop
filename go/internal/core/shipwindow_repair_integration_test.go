//go:build integration

package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestRecordAndBranch_AuditLeaseOnlyForShippableVerdict(t *testing.T) {
	for _, verdict := range []string{VerdictFAIL, VerdictPASS, VerdictWARN} {
		t.Run(verdict, func(t *testing.T) {
			cr := retroGateHarness(t, phasespec.Catalog{})
			led := &fakeLedger{}
			cr.o = NewOrchestrator(&fakeStorage{}, led, buildRunners(nil))
			root, ws := initBindingRepo(t, "cycle-5")
			cr.req.ProjectRoot, cr.cs.WorkspacePath, cr.cs.ActiveWorktree = root, ws, root
			defer cr.releaseShipWindow()
			if _, err := cr.recordAndBranch(PhaseAudit, dispatchResult{resp: PhaseResponse{Verdict: verdict}, attemptCount: 1}); err != nil {
				t.Fatal(err)
			}
			if got, want := cr.shipLease != nil, verdict != VerdictFAIL; got != want {
				t.Fatalf("audit %s holds ship lease=%v, want %v", verdict, got, want)
			}
			bound := false
			for _, entry := range led.entries {
				if entry.Role == "auditor" && entry.Kind == "agent_subprocess" {
					bound = len(entry.GitHEAD) == 40 && (verdict != VerdictFAIL || entry.ExitCode == 2)
				}
			}
			if !bound {
				t.Fatal("audit binding missing or rejection lost")
			}
			if verdict == VerdictFAIL && len(cr.state.FailedAt) != 1 {
				t.Fatalf("audit failure evidence lost: %+v", cr.state.FailedAt)
			}
			if verdict != VerdictFAIL {
				if _, err := cr.recordAndBranch(PhaseShip, dispatchResult{resp: PhaseResponse{Verdict: VerdictPASS}, attemptCount: 1}); err != nil {
					t.Fatal(err)
				}
				if cr.shipLease != nil {
					t.Fatal("ship completion retained lease")
				}
			}
		})
	}
}
