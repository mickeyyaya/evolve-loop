package audit

import (
	"context"
	"fmt"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func auditTimeoutErr() error {
	return fmt.Errorf("bridge: launch exit=%d: %w", 81, core.ErrArtifactTimeout)
}

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

func TestRun_Timeout_PassReport_RedCountPositive_StaysFail(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 2)
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
