package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func writeDispatchPolicy(t *testing.T, evolveDir, policyVal string) {
	t.Helper()
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("writeDispatchPolicy: mkdir: %v", err)
	}
	content := fmt.Sprintf(`{"dispatch":{"policy":%q},"workflow":{"auto_prune":false}}`, policyVal)
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("writeDispatchPolicy: write: %v", err)
	}
}

func writeDispatchPolicyFull(t *testing.T, evolveDir, policyVal string, repeatThreshold int) {
	t.Helper()
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("writeDispatchPolicyFull: mkdir: %v", err)
	}
	content := fmt.Sprintf(`{"dispatch":{"policy":%q,"repeat_threshold":%d},"workflow":{"auto_prune":false}}`, policyVal, repeatThreshold)
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("writeDispatchPolicyFull: write: %v", err)
	}
}

func TestRunLoop_DryRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--dry-run",
		"--goal-text", "test goal",
		"--strategy", "balanced",
		"--cycles", "3",
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%q", rc, stderr.String())
	}
	var out struct {
		DryRun bool `json:"dry_run"`
		Config struct {
			Strategy  string `json:"strategy"`
			MaxCycles int    `json:"max_cycles"`
		} `json:"config"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal stdout: %v\nstdout=%s", err, stdout.String())
	}
	if !out.DryRun {
		t.Fatalf("dry_run=false; want true")
	}
	if out.Config.Strategy != "balanced" || out.Config.MaxCycles != 3 {
		t.Fatalf("config mismatch: %+v", out.Config)
	}
}

func TestRunLoop_EnvFlagPropagation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--dry-run",
		"--consensus-audit",
		"--reset",
		"--goal-text", "test",
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
}

func TestRunLoop_EnvFlagPropagationNonDry(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeDispatchPolicy(t, evolveDir, "off")
	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--consensus-audit",
		"--reset",
		"--goal-text", "x",
		"--cycles", "1",
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%q", rc, stderr.String())
	}
}

type erroringReadStorage struct{ fixtures.FakeStorage }

func (e *erroringReadStorage) ReadState(context.Context) (core.State, error) {
	return core.State{}, errors.New("synthetic ReadState error")
}

func TestReadLastCycleNumber_ReadStateError(t *testing.T) {
	t.Parallel()
	s := &erroringReadStorage{}
	n, err := readLastCycleNumber(context.Background(), s)
	if err == nil {
		t.Fatalf("expected error")
	}
	if n != 0 {
		t.Fatalf("n=%d want 0 on error", n)
	}
}

func TestParseLoopArgs_FlagParseError(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	_, rc := parseLoopArgs([]string{"--no-such-flag"}, &stderr)
	if rc != 10 {
		t.Fatalf("rc=%d want 10", rc)
	}
}

func TestParseLoopArgs_MaxCyclesFlag(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	cfg, rc := parseLoopArgs([]string{"--max-cycles", "7", "x"}, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%q", rc, stderr.String())
	}
	if cfg.MaxCycles != 7 {
		t.Fatalf("MaxCycles=%d want 7", cfg.MaxCycles)
	}
}

func TestRunLoop_PolicyUnknownLogs(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	got := resolveDispatchPolicy("garbage", &stderr)
	if got != dispatchPolicyVerify {
		t.Fatalf("got %v want verify", got)
	}
	if !strings.Contains(stderr.String(), "WARN") {
		t.Fatalf("stderr should warn: %q", stderr.String())
	}
}

func TestRunLoop_ResumePhaseRunnerError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	gitRun := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = projectRoot
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitRun("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(projectRoot, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitRun("add", ".")
	gitRun("commit", "-m", "init")

	head, _ := exec.Command("git", "-C", projectRoot, "rev-parse", "HEAD").Output()
	headStr := strings.TrimSpace(string(head))

	wt := filepath.Join(projectRoot, ".evolve", "worktrees", "cycle-1")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatalf("mkdir wt: %v", err)
	}

	csContent := fmt.Sprintf(`{
  "cycle_id": 1,
  "phase": "build",
  "checkpoint": {
    "enabled": true,
    "resumeFromPhase": "build",
    "worktreePath": %q,
    "gitHead": %q,
    "reason": "test"
  }
}`, wt, headStr)
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), []byte(csContent), 0o644); err != nil {
		t.Fatalf("write cs: %v", err)
	}

	prev := wireOrchestratorDepsFn
	defer func() { wireOrchestratorDepsFn = prev }()
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		st := &fixtures.FakeStorage{CycleState: core.CycleState{
			CycleID: 1, Phase: string(core.PhaseBuild), ActiveWorktree: wt,
		}}
		ld := newFakeLedger()
		runners := map[core.Phase]core.PhaseRunner{
			core.PhaseBuild: errorRunner{phase: "build"},
			core.PhaseAudit: noopRunner{name: "audit"},
			core.PhaseShip:  noopRunner{name: "ship"},
			core.PhaseRetro: noopRunner{name: "retro"},
		}
		return orchDeps{Storage: st, Ledger: ld, Orchestrator: core.NewOrchestrator(st, ld, runners)}
	}
	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--resume",
	}, nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("rc=%d want 2; stderr=%q stdout=%q", rc, stderr.String(), stdout.String())
	}
}

func TestRunLoop_ResumeFailVerdict(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	gitRun := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = projectRoot
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitRun("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(projectRoot, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitRun("add", ".")
	gitRun("commit", "-m", "init")

	head, _ := exec.Command("git", "-C", projectRoot, "rev-parse", "HEAD").Output()
	headStr := strings.TrimSpace(string(head))

	wt := filepath.Join(projectRoot, ".evolve", "worktrees", "cycle-1")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatalf("mkdir wt: %v", err)
	}

	csContent := fmt.Sprintf(`{
  "cycle_id": 1,
  "phase": "audit",
  "checkpoint": {
    "enabled": true,
    "resumeFromPhase": "audit",
    "worktreePath": %q,
    "gitHead": %q,
    "reason": "test"
  }
}`, wt, headStr)
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), []byte(csContent), 0o644); err != nil {
		t.Fatalf("write cs: %v", err)
	}

	prev := wireOrchestratorDepsFn
	defer func() { wireOrchestratorDepsFn = prev }()
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		st := &fixtures.FakeStorage{CycleState: core.CycleState{
			CycleID: 1, Phase: string(core.PhaseAudit), ActiveWorktree: wt,
		}}
		ld := newFakeLedger()
		runners := map[core.Phase]core.PhaseRunner{
			core.PhaseAudit: failVerdictRunner{name: "audit"},
			core.PhaseRetro: failVerdictRunner{name: "retro"},
			core.PhaseShip:  failVerdictRunner{name: "ship"},
		}
		return orchDeps{Storage: st, Ledger: ld, Orchestrator: core.NewOrchestrator(st, ld, runners)}
	}
	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--resume",
	}, nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("rc=%d want 2 (resume fail); stderr=%q stdout=%q", rc, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"stop_reason": "fail"`) {
		t.Fatalf("expected stop_reason=fail; stdout=%q", stdout.String())
	}
}

type errorRunner struct{ phase string }

func (e errorRunner) Name() string { return e.phase }
func (e errorRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{}, fmt.Errorf("synthetic phase error in %s", e.phase)
}

type fatalRunner struct{ phase string }

func (f fatalRunner) Name() string { return f.phase }
func (f fatalRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{}, core.ErrLedgerChainBroken
}

func TestRunLoop_OrchestratorError(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeDispatchPolicy(t, evolveDir, "off")
	prev := wireOrchestratorDepsFn
	defer func() { wireOrchestratorDepsFn = prev }()
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		st := &fixtures.FakeStorage{}
		ld := newFakeLedger()
		runners := map[core.Phase]core.PhaseRunner{
			core.PhaseScout:  fatalRunner{phase: "scout"},
			core.PhaseTriage: noopRunner{name: "triage"},
			core.PhaseTDD:    noopRunner{name: "tdd"},
			core.PhaseBuild:  noopRunner{name: "build"},
			core.PhaseAudit:  noopRunner{name: "audit"},
			core.PhaseShip:   noopRunner{name: "ship"},
			core.PhaseRetro:  noopRunner{name: "retro"},
			core.PhaseIntent: noopRunner{name: "intent"},
		}
		return orchDeps{Storage: st, Ledger: ld, Orchestrator: core.NewOrchestrator(st, ld, runners)}
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "x",
		"--cycles", "1",
	}, nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("rc=%d want 2 (cycle error); stderr=%q", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"stop_reason": "error"`) {
		t.Fatalf("stop_reason should be error; stdout=%q", stdout.String())
	}
}

type erroringLedger struct{ *fakeLedgerNoAppend }

func (l *erroringLedger) Iter(context.Context) (core.LedgerIterator, error) {
	return nil, errors.New("synthetic ledger iter error")
}

func TestRunLoop_VerifyIterError(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	initLoopContractRepo(t, projectRoot)
	writeDispatchPolicy(t, evolveDir, "verify")
	prev := wireOrchestratorDepsFn
	defer func() { wireOrchestratorDepsFn = prev }()
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		st := &fixtures.FakeStorage{}
		ld := &erroringLedger{fakeLedgerNoAppend: newFakeLedger()}
		runners := map[core.Phase]core.PhaseRunner{
			core.PhaseIntent:       noopRunner{name: "intent"},
			core.PhaseScout:        noopRunner{name: "scout"},
			core.PhaseTriage:       noopRunner{name: "triage"},
			core.PhaseTDD:          noopRunner{name: "tdd"},
			core.PhaseBuildPlanner: noopRunner{name: "build-planner"},
			core.PhaseBuild:        noopRunner{name: "build"},
			core.PhaseAudit:        noopRunner{name: "audit"},
			core.PhaseShip:         noopRunner{name: "ship"},
			core.PhaseRetro:        noopRunner{name: "retro"},
		}
		return orchDeps{Storage: st, Ledger: ld, Orchestrator: core.NewOrchestrator(st, ld, runners)}
	}
	if err := os.MkdirAll(cycleWorkspace(projectRoot, 1), 0o755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cycleWorkspace(projectRoot, 1), "orchestrator-report.md"), []byte("OK"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "x",
		"--cycles", "1",
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0 (verify error is non-fatal); stderr=%q stdout=%q", rc, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "verify cycle") {
		t.Fatalf("stderr should mention verify error: %q", stderr.String())
	}
}

type failVerdictRunner struct{ name string }

func (f failVerdictRunner) Name() string { return f.name }
func (f failVerdictRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	if f.name == string(core.PhaseBuild) && req.ExplanationDocumentationVersion != 0 {
		if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
			return core.PhaseResponse{}, err
		}
		report := explanationdocs.RenderNotApplicableDeclaration("the base-bound Build diff contains no material changes")
		if err := os.WriteFile(filepath.Join(req.Workspace, "build-report.md"), []byte(report), 0o644); err != nil {
			return core.PhaseResponse{}, err
		}
	}
	return core.PhaseResponse{Verdict: core.VerdictFAIL}, nil
}

func TestRunLoop_FailVerdictBreaks(t *testing.T) {
	t.Setenv("EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS", "1")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	initLoopContractRepo(t, projectRoot)
	writeDispatchPolicy(t, evolveDir, "off")
	prev := wireOrchestratorDepsFn
	defer func() { wireOrchestratorDepsFn = prev }()
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		st := &fixtures.FakeStorage{}
		ld := newFakeLedger()
		runners := map[core.Phase]core.PhaseRunner{
			core.PhaseIntent:       failVerdictRunner{name: "intent"},
			core.PhaseScout:        failVerdictRunner{name: "scout"},
			core.PhaseTriage:       failVerdictRunner{name: "triage"},
			core.PhaseTDD:          failVerdictRunner{name: "tdd"},
			core.PhaseBuildPlanner: noopRunner{name: "build-planner"},
			core.PhaseBuild:        failVerdictRunner{name: "build"},
			core.PhaseAudit:        failVerdictRunner{name: "audit"},
			core.PhaseShip:         failVerdictRunner{name: "ship"},
			core.PhaseRetro:        failVerdictRunner{name: "retro"},
		}
		return orchDeps{Storage: st, Ledger: ld, Orchestrator: core.NewOrchestrator(st, ld, runners)}
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "x",
		"--cycles", "2",
	}, nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("rc=%d want 2 (fail verdict); stderr=%q stdout=%q", rc, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"stop_reason": "fail"`) {
		t.Fatalf("stop_reason should be fail; stdout=%q", stdout.String())
	}
}

func TestRunLoop_ResumeMissingCheckpoint(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	prev := wireOrchestratorDepsFn
	defer func() { wireOrchestratorDepsFn = prev }()
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		return orchDeps{
			Storage: &fixtures.FakeStorage{},
			Ledger:  newFakeLedger(),
		}
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--resume",
	}, nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("rc=%d want 2 (resume error); stderr=%q", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"stop_reason": "error"`) {
		t.Fatalf("stop_reason should be error; stdout=%q", stdout.String())
	}
}

func TestRunLoop_ResumeFullProtocol(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	gitInit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = projectRoot
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@example",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@example",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitInit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(projectRoot, "README.md"), []byte("test"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitInit("add", ".")
	gitInit("commit", "-m", "init")
	headCmd := exec.Command("git", "rev-parse", "HEAD")
	headCmd.Dir = projectRoot
	headBytes, err := headCmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	head := strings.TrimSpace(string(headBytes))

	worktree := filepath.Join(projectRoot, ".evolve", "worktrees", "cycle-1")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	csContent := fmt.Sprintf(`{
  "cycle_id": 1,
  "phase": "build",
  "checkpoint": {
    "enabled": true,
    "resumeFromPhase": "build",
    "worktreePath": %q,
    "gitHead": %q,
    "reason": "test resume",
    "savedAt": "2026-05-23T00:00:00Z",
    "costAtCheckpoint": 1.25
  }
}`, worktree, head)
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), []byte(csContent), 0o644); err != nil {
		t.Fatalf("write cycle-state: %v", err)
	}

	prev := wireOrchestratorDepsFn
	defer func() { wireOrchestratorDepsFn = prev }()
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		st := &fixtures.FakeStorage{CycleState: core.CycleState{
			CycleID: 1, Phase: string(core.PhaseBuild), ActiveWorktree: worktree,
		}}
		ld := newFakeLedger()
		runners := map[core.Phase]core.PhaseRunner{
			core.PhaseBuild: noopRunner{name: "build"},
			core.PhaseAudit: noopRunner{name: "audit"},
			core.PhaseShip:  noopRunner{name: "ship"},
			core.PhaseRetro: noopRunner{name: "retro"},
		}
		return orchDeps{Storage: st, Ledger: ld, Orchestrator: core.NewOrchestrator(st, ld, runners)}
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--resume",
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0 (resumed_complete); stderr=%q stdout=%q", rc, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"resumed": true`) {
		t.Fatalf("output should mark resumed=true; stdout=%q", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"stop_reason": "resumed_complete"`) {
		t.Fatalf("stop_reason should be resumed_complete; stdout=%q", stdout.String())
	}
}
