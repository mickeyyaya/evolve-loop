//go:build integration

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type resumedLoginRunner struct{}

func (resumedLoginRunner) Name() string { return "audit" }
func (resumedLoginRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{}, core.WalkError{Wall: core.Wall{Ends: []core.FamilyEnd{loginEnd}}, Err: fmt.Errorf("bridge: launch exit=85: %w", core.ErrTransientBridgeFailure)}
}

func TestRunResumeBatch_ALoginWallAtTheResumeEmitsLoginRequired(t *testing.T) {
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
	cs := core.CycleState{CycleID: 7, RunID: "original-run", Phase: "audit", WorkspacePath: ws, QuotaWall: []core.FamilyEnd{quotaEnd}}
	if err := st.WriteState(ctx, core.State{LastCycleNumber: 7}); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteCycleState(ctx, cs); err != nil {
		t.Fatal(err)
	}
	if err := checkpoint.ApplyToStateFile(filepath.Join(evolveDir, core.CycleStateFile), checkpoint.Compose(cs, checkpoint.ReasonQuotaLikely, 0, "", time.Now())); err != nil {
		t.Fatal(err)
	}
	c, got := recordedCenter()
	ledger := newFakeLedger()
	orch := core.NewOrchestrator(st, ledger, map[core.Phase]core.PhaseRunner{core.PhaseAudit: resumedLoginRunner{}}, core.WithRetryConfig(policy.RetryConfig{PhaseMaxAttempts: 2}))
	var stdout, stderr bytes.Buffer
	var lr loopResult

	rc := runResumeBatch(ctx, loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, orch, ledger, c, nil, nil, &lr, &stdout, &stderr)

	events := loginEvents(*got)
	if rc != 5 || len(events) != 1 || events[0].Origin != "runResumeBatch" || events[0].Phase != "audit" {
		t.Fatalf("rc=%d events=%+v stderr=%s", rc, events, stderr.String())
	}
}
