//go:build integration

package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
)

func TestRecordAndBranch_AnAuditPassLeavesTheShipWindowFreeForASibling(t *testing.T) {
	t.Parallel()

	for _, verdict := range []string{VerdictFAIL, VerdictPASS, VerdictWARN} {
		t.Run(verdict, func(t *testing.T) {
			cr := retroGateHarness(t, phasespec.Catalog{})
			led := &fakeLedger{}
			cr.o = NewOrchestrator(&fakeStorage{}, led, buildRunners(nil))
			root, ws := initBindingRepo(t, "cycle-5")
			cr.req.ProjectRoot, cr.cs.WorkspacePath, cr.cs.ActiveWorktree = root, ws, root
			if _, err := cr.recordAndBranch(PhaseAudit, dispatchResult{resp: PhaseResponse{Verdict: verdict}, attemptCount: 1}); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			sibling, err := shipwindow.Acquire(ctx, filepath.Join(root, ".evolve"), shipwindow.Options{OwnerPID: 1})
			if err != nil {
				t.Fatalf("audit %s: a sibling lane cannot take the ship window while this lane heads into its slow ship checks: %v", verdict, err)
			}
			if err := sibling.Release(); err != nil {
				t.Fatal(err)
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
		})
	}
}
