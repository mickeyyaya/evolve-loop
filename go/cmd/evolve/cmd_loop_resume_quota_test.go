//go:build integration

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type resumedQuotaRunner struct{}

func (resumedQuotaRunner) Name() string { return "audit" }
func (resumedQuotaRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{}, fmt.Errorf("bridge: launch exit=85: %w", core.ErrTransientBridgeFailure)
}

func TestRunLoop_ResumeQuotaPauseReturnsFiveAndPreservesCheckpoint(t *testing.T) {
	// Stub only the tmux process boundary: this CLI test must not sweep host
	// sessions while exercising real storage, checkpoint and orchestration.
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "tmux"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	root := t.TempDir()
	initLoopContractRepo(t, root)
	evolveDir := filepath.Join(root, ".evolve")
	ws := core.RunWorkspacePath(root, 7)
	if err := os.MkdirAll(ws, 0755); err != nil {
		t.Fatal(err)
	}
	st := storage.New(evolveDir)
	ctx := context.Background()
	cs := core.CycleState{CycleID: 7, RunID: "original-run", Phase: "audit", WorkspacePath: ws}
	if err := st.WriteState(ctx, core.State{LastCycleNumber: 7}); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteCycleState(ctx, cs); err != nil {
		t.Fatal(err)
	}
	if err := checkpoint.ApplyToStateFile(filepath.Join(evolveDir, core.CycleStateFile), checkpoint.Compose(cs, checkpoint.ReasonQuotaLikely, 0, "", time.Now())); err != nil {
		t.Fatal(err)
	}
	old := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = old })
	wireOrchestratorDepsFn = func(string, string) orchDeps {
		ledger := &fixtures.FakeLedger{}
		orch := core.NewOrchestrator(st, ledger, map[core.Phase]core.PhaseRunner{core.PhaseAudit: resumedQuotaRunner{}}, core.WithRetryConfig(policy.RetryConfig{PhaseMaxAttempts: 2}))
		return orchDeps{Storage: st, Ledger: ledger, Orchestrator: orch}
	}
	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{"--project-root", root, "--resume"}, nil, &stdout, &stderr)
	if rc != 5 || !strings.Contains(stdout.String(), `"stop_reason": "quota-pause"`) {
		t.Fatalf("resume quota became terminal failure: rc=%d stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
	}
	if qp, ok := detectQuotaPause(evolveDir); !ok || qp.Cycle != 7 {
		t.Fatalf("quota checkpoint lost: %+v present=%v", qp, ok)
	}
	if _, err := os.Stat(filepath.Join(root, "knowledge-base", "cycles", "cycle-7.json")); !os.IsNotExist(err) {
		t.Fatalf("quota pause emitted terminal dossier: %v", err)
	}
}
