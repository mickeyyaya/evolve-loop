//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func cycleRunBackfillRoot(t *testing.T, runners map[core.Phase]core.PhaseRunner) (string, string) {
	t.Helper()
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
	fake := newFakeLedger()
	old := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = old })
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		orch := core.NewOrchestrator(st, fake, runners, core.WithRetryConfig(policy.RetryConfig{PhaseMaxAttempts: 1}))
		return orchDeps{Storage: st, Ledger: fake, Orchestrator: orch}
	}
	return root, evolveDir
}

func TestRunCycleRun_ACompletedCyclePrintsItsResultAndExitsByVerdict(t *testing.T) {
	runners := map[core.Phase]core.PhaseRunner{}
	for _, p := range []core.Phase{core.PhaseIntent, core.PhaseScout, core.PhaseTriage, core.PhaseTDD, core.PhaseBuildPlanner, core.PhaseBuild, core.PhaseAudit, core.PhaseShip, core.PhaseRetro} {
		runners[p] = noopRunner{name: string(p)}
	}
	root, _ := cycleRunBackfillRoot(t, runners)
	var stdout, stderr bytes.Buffer

	rc := runCycleRun([]string{"--project-root", root, "--goal-hash", "abcd1234"}, &stdout, &stderr)

	var res core.CycleResult
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("stdout %q is not a cycle result: %v (rc=%d stderr=%s)", stdout.String(), err, rc, stderr.String())
	}
	if res.FinalVerdict != core.VerdictPASS || res.Cycle != 1 || res.SystemFailure != nil {
		t.Errorf("result = %+v, want cycle 1 printed with a PASS verdict and no system failure", res)
	}
	if rc != 0 {
		t.Errorf("rc = %d, want 0 for a PASS cycle (stderr=%s)", rc, stderr.String())
	}
}

type cycleForgedAuditRunner struct{}

func (cycleForgedAuditRunner) Name() string { return string(core.PhaseAudit) }

func (cycleForgedAuditRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
		return core.PhaseResponse{}, err
	}
	report := "# Audit\n\n" + phasecontract.RenderVerdictSentinel("audit", "PASS") + "\n"
	if err := os.WriteFile(filepath.Join(req.Workspace, phasecontract.ArtifactFilename("audit")), []byte(report), 0o644); err != nil {
		return core.PhaseResponse{}, err
	}
	if err := os.WriteFile(filepath.Join(req.Workspace, "acs-verdict.json"), []byte(`{"verdict":"PASS"}`), 0o644); err != nil {
		return core.PhaseResponse{}, err
	}
	return core.PhaseResponse{Phase: string(core.PhaseAudit), Verdict: core.VerdictFAIL}, nil
}

func TestRunCycleRun_AForgedFailVerdictHaltsWithTheSystemFailureCode(t *testing.T) {
	runners := map[core.Phase]core.PhaseRunner{}
	for _, p := range []core.Phase{core.PhaseIntent, core.PhaseScout, core.PhaseTriage, core.PhaseTDD, core.PhaseBuildPlanner, core.PhaseBuild, core.PhaseShip, core.PhaseRetro} {
		runners[p] = noopRunner{name: string(p)}
	}
	runners[core.PhaseAudit] = cycleForgedAuditRunner{}
	root, evolveDir := cycleRunBackfillRoot(t, runners)
	var stdout, stderr bytes.Buffer

	rc := runCycleRun([]string{"--project-root", root, "--goal-hash", "abcd1234"}, &stdout, &stderr)

	var res core.CycleResult
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("stdout %q: %v (rc=%d stderr=%s)", stdout.String(), err, rc, stderr.String())
	}
	if res.SystemFailure == nil || !res.SystemFailure.Halt {
		t.Fatalf("result = %+v, want a halting system failure", res)
	}
	if rc != systemFailureHaltExitCode {
		t.Errorf("rc = %d, want %d", rc, systemFailureHaltExitCode)
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "pipeline-escalation.json")); err != nil {
		t.Errorf("the halt must write the pipeline escalation dossier: %v", err)
	}
}

type cycleInboxBreakingAuditRunner struct{}

func (cycleInboxBreakingAuditRunner) Name() string { return string(core.PhaseAudit) }

func (cycleInboxBreakingAuditRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
		return core.PhaseResponse{}, err
	}
	if err := os.WriteFile(filepath.Join(req.Workspace, "triage-decision.json"), []byte(`{"top_n":[{"id":"poison"}]}`), 0o644); err != nil {
		return core.PhaseResponse{}, err
	}
	inbox := filepath.Join(req.ProjectRoot, ".evolve", "inbox")
	if err := os.RemoveAll(inbox); err != nil {
		return core.PhaseResponse{}, err
	}
	if err := os.WriteFile(inbox, []byte("not a dir"), 0o644); err != nil {
		return core.PhaseResponse{}, err
	}
	return core.PhaseResponse{Phase: string(core.PhaseAudit), Verdict: core.VerdictFAIL}, nil
}

func TestRunCycleRun_AFailedCycleWhoseInboxCannotBeWalkedWarnsAndExits2(t *testing.T) {
	runners := map[core.Phase]core.PhaseRunner{}
	for _, p := range []core.Phase{core.PhaseIntent, core.PhaseScout, core.PhaseTriage, core.PhaseTDD, core.PhaseBuildPlanner, core.PhaseBuild, core.PhaseShip, core.PhaseRetro} {
		runners[p] = noopRunner{name: string(p)}
	}
	runners[core.PhaseAudit] = cycleInboxBreakingAuditRunner{}
	root, _ := cycleRunBackfillRoot(t, runners)
	var stdout, stderr bytes.Buffer

	rc := runCycleRun([]string{"--project-root", root, "--goal-hash", "abcd1234"}, &stdout, &stderr)

	if rc != 2 {
		t.Errorf("rc = %d, want 2 for a FAIL verdict", rc)
	}
	if !strings.Contains(stderr.String(), "evolve cycle run: WARN: could not apply cycle ") || !strings.Contains(stderr.String(), " failure outcome to the inbox: ") {
		t.Errorf("stderr = %q, want the closeout WARN", stderr.String())
	}
}
