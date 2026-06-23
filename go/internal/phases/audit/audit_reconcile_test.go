package audit

import (
	"context"
	"fmt"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// auditTimeoutErr mimics what bridge.Engine.Launch returns on exit 81.
func auditTimeoutErr() error {
	return fmt.Errorf("bridge: launch exit=%d: %w", 81, core.ErrArtifactTimeout)
}

// TestRun_Timeout_PassReport_RedCountZero_ReconcilesToPass — end-to-end proof of
// reconcile-on-timeout through the REAL audit hooks + deliverable.Verify: the
// bridge times out (exit 81) but the auditor's PASS report is on disk and the
// EGPS suite is green (red_count==0), so the cycle ships instead of false-FAILing.
func TestRun_Timeout_PassReport_RedCountZero_ReconcilesToPass(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{err: auditTimeoutErr(), writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("well-formed PASS audit on timeout must reconcile to nil error; got %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS (reconciled)", resp.Verdict)
	}
	if !resp.Reconciled {
		t.Error("resp.Reconciled must be true")
	}
}

// TestRun_Timeout_PassReport_RedCountPositive_StaysFail is the CRITICAL
// anti-Goodhart test: a PASS-declaring report on a timeout must STILL FAIL when
// the EGPS predicate suite is red (red_count>0). This proves reconciliation
// routes through Classify (the real EGPS gate), not a bare sentinel read — a
// green-looking report with red predicates can never ship.
func TestRun_Timeout_PassReport_RedCountPositive_StaysFail(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 2) // two red predicates
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{err: auditTimeoutErr(), writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("a reconciled (completed) phase returns nil error even when it FAILs; got %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL — a PASS report with red_count>0 must not ship even when reconciled", resp.Verdict)
	}
	if !resp.Reconciled {
		t.Error("resp.Reconciled must be true — reconcile engaged but EGPS correctly FAILed the red suite")
	}
}
