package runner

// budget_scale_thread_test.go — ADR-0076 slice A: the runner must thread
// PhaseRequest.BudgetScale onto the BridgeRequest verbatim, or the dispatch's
// difficulty multiplier dies between core and the engine (the I2 dead-link
// class this campaign exists to kill).

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type budgetRecordingBridge struct{ got float64 }

func (b *budgetRecordingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.got = req.BudgetScale
	if req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte("ok"), 0o644)
	}
	return core.BridgeResponse{Stdout: "ok"}, nil
}
func (b *budgetRecordingBridge) Probe(_ context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func runWithBudgetScale(t *testing.T, scale float64) float64 {
	t.Helper()
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "sonnet", prompt: "body", verdict: core.VerdictPASS}
	br := &budgetRecordingBridge{}
	r := New(Options{Hooks: hooks, Bridge: br, Prompts: fakePromptsFS("evolve-builder", "body")})
	req := core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir(), BudgetScale: scale}
	if _, err := r.Run(context.Background(), req); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return br.got
}

func TestRun_ThreadsBudgetScaleToBridge(t *testing.T) {
	t.Parallel()
	if got := runWithBudgetScale(t, 1.5); got != 1.5 {
		t.Errorf("BridgeRequest.BudgetScale = %v, want 1.5", got)
	}
	if got := runWithBudgetScale(t, 0); got != 0 {
		t.Errorf("unset scale must stay zero (byte-identical dispatch), got %v", got)
	}
}
