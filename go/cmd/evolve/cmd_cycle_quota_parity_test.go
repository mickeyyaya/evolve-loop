package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// cycleRunQuotaRunner hits the capacity wall the way the bridge reports it.
type cycleRunQuotaRunner struct{}

func (cycleRunQuotaRunner) Name() string { return "audit" }
func (cycleRunQuotaRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{}, fmt.Errorf("bridge: launch exit=85: %w", core.ErrTransientBridgeFailure)
}

// scoutFailRunner is a plain, non-capacity phase failure.
type scoutFailRunner struct{}

func (scoutFailRunner) Name() string { return "scout" }
func (scoutFailRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{}, errors.New("synthetic scout bridge death (cycle-level)")
}

// requireCycleRunUsesDepsSeam keeps this test from running the production
// orchestrator (real bridge, real tmux) while the root is still unseamed.
func requireCycleRunUsesDepsSeam(t *testing.T) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "cmd_cycle.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	seamed := false
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || (fn.Name.Name != "runCycleRun" && fn.Name.Name != "wireCycleRun") {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "wireOrchestratorDepsFn" {
				seamed = true
			}
			return true
		})
	}
	if !seamed {
		t.Fatal("RED: runCycleRun builds its orchestrator without wireOrchestratorDepsFn, so no test can drive the cycle-run root's closeout without the production bridge")
	}
}

// runCycleRunWith drives the cycle-run root over one seeded item with every
// phase answered by runner (scout only, for a non-capacity failure).
func runCycleRunWith(t *testing.T, runner core.PhaseRunner, onlyScout bool) (rc int, fake *fakeLedgerNoAppend, itemStillInInbox bool, stderr string) {
	t.Helper()
	requireCycleRunUsesDepsSeam(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "tmux"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	root := t.TempDir()
	initLoopContractRepo(t, root)
	evolveDir, _ := seedFailedCycleInbox(t, root, "poison", 1)
	st := storage.New(evolveDir)
	if err := st.WriteState(context.Background(), core.State{}); err != nil {
		t.Fatal(err)
	}
	fake = newFakeLedger()
	old := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = old })
	wireOrchestratorDepsFn = func(string, string, io.Writer) orchDeps {
		runners := map[core.Phase]core.PhaseRunner{}
		for _, p := range []core.Phase{core.PhaseIntent, core.PhaseScout, core.PhaseTriage, core.PhaseTDD, core.PhaseBuildPlanner, core.PhaseBuild, core.PhaseAudit, core.PhaseShip, core.PhaseRetro} {
			if onlyScout && p != core.PhaseScout {
				runners[p] = noopRunner{name: string(p)}
				continue
			}
			runners[p] = runner
		}
		orch := core.NewOrchestrator(st, fake, runners, core.WithRetryConfig(policy.RetryConfig{PhaseMaxAttempts: 2}))
		return orchDeps{Storage: st, Ledger: fake, Orchestrator: orch}
	}
	var stdout, errBuf bytes.Buffer
	rc = runCycleRun([]string{"--project-root", root, "--goal-hash", "abcd1234"}, &stdout, &errBuf)
	_, err := os.Stat(filepath.Join(evolveDir, "inbox", "2026-09-13T00-00-00Z-poison.json"))
	return rc, fake, err == nil, errBuf.String()
}

func TestCycleRunRoot_QuotaWallPausesLikeResumeAndChargesNobody(t *testing.T) {
	rc, fake, stillInInbox, stderr := runCycleRunWith(t, cycleRunQuotaRunner{}, false)
	if rc != 5 {
		t.Errorf("a quota wall pauses the cycle-run root with exit 5, as the loop and resume roots do; rc=%d stderr=%s", rc, stderr)
	}
	if len(fake.lifecycle) != 0 {
		t.Errorf("a capacity wall is nobody's failure: the failure walk wrote lifecycle lines %+v", fake.lifecycle)
	}
	if !stillInInbox {
		t.Errorf("the committed item must be left where it was for the resumed attempt")
	}
}

func TestCycleRunRoot_NonQuotaCycleFailureStillWalksTheFailureOutcome(t *testing.T) {
	rc, fake, _, stderr := runCycleRunWith(t, scoutFailRunner{}, true)
	if rc != 1 {
		t.Errorf("a non-capacity cycle-level failure keeps exit 1; rc=%d stderr=%s", rc, stderr)
	}
	if len(fake.lifecycle) == 0 {
		t.Errorf("a non-capacity cycle-level failure must still walk the inbox failure lifecycle; stderr=%s", stderr)
	}
}
