package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestRunLoop_DryRun verifies the --dry-run short-circuit prints the
// resolved config and exits 0 without invoking the orchestrator.
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

// TestRunLoop_EnvFlagPropagation drives the ConsensusAudit / Reset env
// var setters via --dry-run so we exercise the assignment without
// running a real cycle. Each flag should set its corresponding env key
// in the cycle invocation context.
//
// (The dry-run output doesn't include cycleEnv directly, but the
// branches at lines 85-93 are still exercised because runLoop builds
// the map before the dry-run short-circuit.)
func TestRunLoop_EnvFlagPropagation(t *testing.T) {
	// Force the env-setting branches to fire by passing the flags.
	// Dry-run still applies AFTER the env-map build, so all three
	// conditional branches run.
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

// TestRunLoop_EnvFlagPropagationNonDry covers lines 85-93 (env var
// assignments) by running with --consensus-audit and --reset against
// a real (stubbed) cycle. The DryRun-based test exercises parsing but
// short-circuits before those lines.
func TestRunLoop_EnvFlagPropagationNonDry(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "off")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
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

// erroringReadStorage returns an error on ReadState, driving the
// err != nil branch in readLastCycleNumber.
type erroringReadStorage struct{ fixtures.FakeStorage }

func (e *erroringReadStorage) ReadState(context.Context) (core.State, error) {
	return core.State{}, errors.New("synthetic ReadState error")
}

// TestReadLastCycleNumber_ReadStateError covers the err != nil branch
// in readLastCycleNumber.
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

// TestParseLoopArgs_FlagParseError covers parseLoopArgs's fs.Parse
// error branch (line 443). Passing an unknown flag triggers it.
func TestParseLoopArgs_FlagParseError(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	_, rc := parseLoopArgs([]string{"--no-such-flag"}, &stderr)
	if rc != 10 {
		t.Fatalf("rc=%d want 10", rc)
	}
}

// TestParseLoopArgs_MaxCyclesFlag covers the maxCyclesFlag>0 branch in
// the resolve-cycles switch (line 455). Only --max-cycles set.
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

// TestRunLoop_PolicyUnknownDefaultsToVerify covers the default branch
// in resolveDispatchPolicy (unknown value → verify with WARN). The
// WARN line is what we need to see.
func TestRunLoop_PolicyUnknownLogs(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "garbage")
	var stderr bytes.Buffer
	got := resolveDispatchPolicy(&stderr)
	if got != dispatchPolicyVerify {
		t.Fatalf("got %v want verify", got)
	}
	if !strings.Contains(stderr.String(), "WARN") {
		t.Fatalf("stderr should warn: %q", stderr.String())
	}
}

// erroringRunCycleFromPhaseRunner returns an error so the resume
// branch's err != nil path (line 132-135 + 142-144) fires.
//
// Note: we can't easily make RunCycleFromPhase return an error from
// the orchestrator without driving the state machine through an
// invalid phase. Easiest: have the phase runner return an error.

// TestRunLoop_ResumePhaseRunnerError exercises the resume err != nil
// branch by stubbing the build runner to return an error.
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
	wireOrchestratorDepsFn = func(string, string) orchDeps {
		st := &fixtures.FakeStorage{}
		ld := newFakeLedger()
		runners := map[core.Phase]core.PhaseRunner{
			core.PhaseBuild: errorRunner{phase: "build"}, // errors → resume err branch
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

// TestRunLoop_ResumeFailVerdict covers the resume result.FinalVerdict
// == FAIL branch.
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
	wireOrchestratorDepsFn = func(string, string) orchDeps {
		st := &fixtures.FakeStorage{}
		ld := newFakeLedger()
		// Audit + downstream all FAIL → final verdict is FAIL
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

// errorRunner satisfies core.PhaseRunner but always returns an error,
// driving the orchestrator's err return → runLoop's cycle-error branch.
type errorRunner struct{ phase string }

func (e errorRunner) Name() string { return e.phase }
func (e errorRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{}, fmt.Errorf("synthetic phase error in %s", e.phase)
}

// TestRunLoop_OrchestratorError covers the err != nil branch at line
// 174-177 (cycle error path). Replace the scout runner with errorRunner
// so the orchestrator returns an error on the very first phase.
func TestRunLoop_OrchestratorError(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "off")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	prev := wireOrchestratorDepsFn
	defer func() { wireOrchestratorDepsFn = prev }()
	wireOrchestratorDepsFn = func(string, string) orchDeps {
		st := &fixtures.FakeStorage{}
		ld := newFakeLedger()
		runners := map[core.Phase]core.PhaseRunner{
			core.PhaseScout:  errorRunner{phase: "scout"},
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

// erroringLedger satisfies core.Ledger but always errors on Iter,
// driving the verify-error branch in cmd_loop.
type erroringLedger struct{ *fakeLedgerNoAppend }

func (l *erroringLedger) Iter(context.Context) (core.LedgerIterator, error) {
	return nil, errors.New("synthetic ledger iter error")
}

// TestRunLoop_VerifyIterError covers the vErr != nil branch at line
// 227-229. VerifyCycle returns an error but the loop continues
// (verify-error is non-fatal — bash treats it as "log and proceed").
func TestRunLoop_VerifyIterError(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "verify")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	prev := wireOrchestratorDepsFn
	defer func() { wireOrchestratorDepsFn = prev }()
	wireOrchestratorDepsFn = func(string, string) orchDeps {
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
	// Workspace must exist so the orchestrator's WriteCycleState
	// succeeds for the first phase write.
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
	// Verify error is non-fatal — rc=0 because the cycle completed and
	// no breach was detected.
	if rc != 0 {
		t.Fatalf("rc=%d want 0 (verify error is non-fatal); stderr=%q stdout=%q", rc, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "verify cycle") {
		t.Fatalf("stderr should mention verify error: %q", stderr.String())
	}
}

// failVerdictRunner returns a FAIL verdict so the FinalVerdict==FAIL
// branch fires.
type failVerdictRunner struct{ name string }

func (f failVerdictRunner) Name() string { return f.name }
func (failVerdictRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{Verdict: core.VerdictFAIL}, nil
}

// TestRunLoop_FailVerdictBreaks covers the result.FinalVerdict == FAIL
// branch (lines 260-262 + 268-269). Audit returns FAIL, the loop
// breaks with stop_reason=fail, rc=2.
func TestRunLoop_FailVerdictBreaks(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "off") // skip verify so policy doesn't intercept

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	prev := wireOrchestratorDepsFn
	defer func() { wireOrchestratorDepsFn = prev }()
	wireOrchestratorDepsFn = func(string, string) orchDeps {
		st := &fixtures.FakeStorage{}
		ld := newFakeLedger()
		// Every phase returns FAIL so the final verdict at retro is FAIL.
		// build-planner uses noopRunner (PASS) because its ShouldSkip
		// would return SKIPPED in shadow mode; FAIL at build is sufficient.
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

// TestRunLoop_ResumeMissingCheckpoint covers the --resume path with no
// cycle-state.json present (LoadResumeState returns ErrNoCheckpoint).
// runLoop must exit with rc=2 and stop_reason=error.
func TestRunLoop_ResumeMissingCheckpoint(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// No cycle-state.json → LoadResumeState fails.

	prev := wireOrchestratorDepsFn
	defer func() { wireOrchestratorDepsFn = prev }()
	wireOrchestratorDepsFn = func(string, string) orchDeps {
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

// TestRunLoop_ResumeFullProtocol exercises the full --resume path
// (LoadResumeState success → RunCycleFromPhase → resumed_complete).
// Requires a real git repo so LoadResumeState's git rev-parse HEAD
// succeeds and matches the checkpoint's gitHead.
func TestRunLoop_ResumeFullProtocol(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Initialize a real git repo with one commit so HEAD is real.
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

	// Create a worktree dir so PathExists check succeeds.
	worktree := filepath.Join(projectRoot, ".evolve", "worktrees", "cycle-1")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	// Write cycle-state.json with a checkpoint block.
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
	wireOrchestratorDepsFn = func(string, string) orchDeps {
		st := &fixtures.FakeStorage{}
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
