//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
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
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nexit 0\n")
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
	wireOrchestratorDepsFn = func(string, string, io.Writer) orchDeps {
		ledger := newFakeLedger() // satisfies rootLedger; see ADR-0101.
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
	var state struct {
		FailedApproaches []json.RawMessage `json:"failedApproaches"`
	}
	raw, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if err != nil || json.Unmarshal(raw, &state) != nil {
		t.Fatalf("state.json unreadable after the pause: %v", err)
	}
	if len(state.FailedApproaches) != 0 {
		t.Fatalf("a capacity wall is nobody's failed approach, yet the pause recorded one: %s", raw)
	}
}

func TestRunLoop_AFreshCycleWalledOnCapacityIsNobodysFailedApproach(t *testing.T) {
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	root := t.TempDir()
	initLoopContractRepo(t, root)
	evolveDir := filepath.Join(root, ".evolve")
	st := storage.New(evolveDir)
	if err := st.WriteState(context.Background(), core.State{}); err != nil {
		t.Fatal(err)
	}
	old := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = old })
	wireOrchestratorDepsFn = func(string, string, io.Writer) orchDeps {
		ledger := newFakeLedger()
		runners := map[core.Phase]core.PhaseRunner{}
		for _, p := range []core.Phase{core.PhaseIntent, core.PhaseScout, core.PhaseTriage, core.PhaseTDD, core.PhaseBuildPlanner, core.PhaseBuild, core.PhaseAudit, core.PhaseShip, core.PhaseRetro} {
			runners[p] = resumedQuotaRunner{}
		}
		orch := core.NewOrchestrator(st, ledger, runners, core.WithRetryConfig(policy.RetryConfig{PhaseMaxAttempts: 2}))
		return orchDeps{Storage: st, Ledger: ledger, Orchestrator: orch}
	}
	var stdout, stderr bytes.Buffer

	rc := runLoop([]string{"--project-root", root, "--max-cycles", "1", "--goal-text", "walled"}, nil, &stdout, &stderr)

	if rc != 5 || !strings.Contains(stdout.String(), `"stop_reason": "quota-pause"`) {
		t.Fatalf("a walled fresh cycle pauses the loop: rc=%d stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
	}
	var state struct {
		FailedApproaches []json.RawMessage `json:"failedApproaches"`
	}
	raw, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if err != nil || json.Unmarshal(raw, &state) != nil {
		t.Fatalf("state.json unreadable after the pause: %v", err)
	}
	if len(state.FailedApproaches) != 0 {
		t.Fatalf("a capacity wall is nobody's failed approach, yet the pause recorded one: %s", raw)
	}
	var summary struct {
		RecoverableFailures int `json:"recoverable_failures"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
		t.Fatalf("the loop's summary is JSON: %v\n%s", err, stdout.String())
	}
	if summary.RecoverableFailures != 0 {
		t.Errorf("a deferral is not a recoverable failure: %+v", summary)
	}
}
