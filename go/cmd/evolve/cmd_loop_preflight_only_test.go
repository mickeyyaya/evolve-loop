package main

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type preflightOnlyProbe struct {
	gateCalls int
	gateCfg   loopConfig
	launched  bool
}

func stubPreflightOnlyGate(t *testing.T, res looppreflight.Result) *preflightOnlyProbe {
	t.Helper()
	probe := &preflightOnlyProbe{}
	prevGate, prevDeps := runLoopPreflightFn, wireOrchestratorDepsFn
	t.Cleanup(func() { runLoopPreflightFn, wireOrchestratorDepsFn = prevGate, prevDeps })
	runLoopPreflightFn = func(cfg loopConfig, _ io.Writer) looppreflight.Result {
		probe.gateCalls++
		probe.gateCfg = cfg
		return res
	}
	wireOrchestratorDepsFn = func(string, string, io.Writer) orchDeps {
		probe.launched = true
		return orchDeps{Storage: &fixtures.FakeStorage{}, Ledger: newFakeLedger()}
	}
	return probe
}

func gateOutcome(checks ...looppreflight.CheckResult) looppreflight.Result {
	r := looppreflight.Result{Checks: checks, ChecksTotal: len(checks), GeneratedAt: "1970-01-01T00:00:00Z"}
	for _, c := range checks {
		if c.Level == looppreflight.LevelPass {
			r.ChecksPassed++
		}
		if c.Level > r.OverallLevel {
			r.OverallLevel = c.Level
		}
	}
	return r
}

func gateCheck(name string, level looppreflight.CheckLevel) looppreflight.CheckResult {
	return looppreflight.CheckResult{Name: name, Level: level, Message: name + " verdict"}
}

func preflightOnlyProject(t *testing.T) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	if err := os.MkdirAll(filepath.Join(evolveDir, "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root, evolveDir
}

func requireNoRunState(t *testing.T, evolveDir string) {
	t.Helper()
	for _, name := range []string{"cycle-state.json", "state.json", "ledger.jsonl"} {
		if _, err := os.Stat(filepath.Join(evolveDir, name)); err == nil {
			t.Errorf("--preflight-only wrote %s; it must leave no run state", name)
		}
	}
	_ = filepath.WalkDir(evolveDir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == ".lease" {
			t.Errorf("--preflight-only wrote a run lease at %s", path)
		}
		return nil
	})
}

func blockingLines(stderr string) []string {
	var out []string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, "blocking") {
			out = append(out, line)
		}
	}
	return out
}

func TestLoopPreflightOnly_PassingGateExitsZeroReadyWithoutDispatch(t *testing.T) {
	variants := map[string][]string{
		"no goal":           {},
		"with a goal":       {"--goal-text", "ship the readiness probe"},
		"chain flag":        {"--until-inbox-empty"},
		"warnings only":     {},
		"positional cycles": {"3", "harden"},
	}
	for name, extra := range variants {
		t.Run(name, func(t *testing.T) {
			root, evolveDir := preflightOnlyProject(t)
			second := gateCheck("cli-health", looppreflight.LevelPass)
			if name == "warnings only" {
				second = gateCheck("disk-space", looppreflight.LevelWarn)
			}
			probe := stubPreflightOnlyGate(t, gateOutcome(gateCheck("pipeline-structure", looppreflight.LevelPass), second))
			args := append([]string{"--preflight-only", "--project-root", root, "--skip-preflight-boot"}, extra...)
			var stdout, stderr bytes.Buffer
			rc := runLoop(args, nil, &stdout, &stderr)
			if rc != 0 {
				t.Fatalf("a gate with no halting check must exit 0, got %d\nstdout:\n%s\nstderr:\n%s", rc, stdout.String(), stderr.String())
			}
			if probe.gateCalls != 1 {
				t.Errorf("--preflight-only must run the readiness gate exactly once, ran it %d times", probe.gateCalls)
			}
			if !probe.gateCfg.SkipPreflightBoot {
				t.Errorf("--preflight-only must honour --skip-preflight-boot when it runs the gate")
			}
			if probe.launched {
				t.Errorf("--preflight-only reached the batch launch path; it must dispatch nothing")
			}
			for _, want := range []string{"READY", "pipeline-structure", second.Name} {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("stdout must carry %q (the verdict and each check); got:\n%s", want, stdout.String())
				}
			}
			requireNoRunState(t, evolveDir)
		})
	}
}

func TestLoopPreflightOnly_HaltingGateExitsOneNamingEachBlockingCheck(t *testing.T) {
	root, evolveDir := preflightOnlyProject(t)
	probe := stubPreflightOnlyGate(t, gateOutcome(
		gateCheck("pipeline-structure", looppreflight.LevelPass),
		gateCheck("bridge-boot", looppreflight.LevelHalt),
		gateCheck("disk-space", looppreflight.LevelWarn),
		gateCheck("llm-cli-status", looppreflight.LevelHalt),
	))
	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{"--preflight-only", "--project-root", root}, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("a halting gate under --preflight-only must exit 1 (a launch keeps exit 2), got %d\nstdout:\n%s\nstderr:\n%s", rc, stdout.String(), stderr.String())
	}
	if probe.launched {
		t.Errorf("--preflight-only reached the batch launch path on a halt")
	}
	if strings.Contains(stdout.String(), "READY") {
		t.Errorf("a halting gate must not print READY:\n%s", stdout.String())
	}
	blocking := strings.Join(blockingLines(stderr.String()), "\n")
	for _, halted := range []string{`"bridge-boot"`, `"llm-cli-status"`} {
		if !strings.Contains(blocking, halted) {
			t.Errorf("stderr must name blocking check %s on a blocking line; blocking lines:\n%s\nstderr:\n%s", halted, blocking, stderr.String())
		}
	}
	for _, passing := range []string{`"disk-space"`, `"pipeline-structure"`} {
		if strings.Contains(blocking, passing) {
			t.Errorf("check %s did not halt, so it must not be reported as blocking:\n%s", passing, blocking)
		}
	}
	requireNoRunState(t, evolveDir)
}

func TestLoopPreflightOnly_LeavesTheBoundaryReexecHandoffForTheLoop(t *testing.T) {
	root, evolveDir := preflightOnlyProject(t)
	prevCommit := chainRunningCommitFn
	t.Cleanup(func() { chainRunningCommitFn = prevCommit })
	chainRunningCommitFn = func() string { return "0ddba11beef0" }
	marker := writeHandoff(t, evolveDir, os.Getpid(), 0)
	armed, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	stubPreflightOnlyGate(t, gateOutcome(gateCheck("pipeline-structure", looppreflight.LevelPass)))
	var stdout, stderr bytes.Buffer
	if rc := runLoop([]string{"--preflight-only", "--project-root", root}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc=%d want 0\nstderr:\n%s", rc, stderr.String())
	}
	after, err := os.ReadFile(marker)
	if err != nil || !bytes.Equal(after, armed) {
		t.Fatalf("--preflight-only consumed or rewrote the boundary re-exec handoff (err=%v):\nbefore %s\nafter  %s", err, armed, after)
	}
	if waves, handedOff := takeReexecHandoff(evolveDir, io.Discard); waves != 1 || !handedOff {
		t.Errorf("the handoff must still be honourable by the real loop after --preflight-only: waves=%d handedOff=%v want 1, true", waves, handedOff)
	}
}

func TestLoopPreflightOnly_ConflictingModesExitTenWithoutRunningTheGate(t *testing.T) {
	cases := map[string][]string{
		"skip-preflight": {"--skip-preflight"},
		"dry-run":        {"--dry-run"},
		"detach":         {"--detach", "--log", "detach.log"},
	}
	for name, extra := range cases {
		t.Run(name, func(t *testing.T) {
			root, evolveDir := preflightOnlyProject(t)
			probe := stubPreflightOnlyGate(t, gateOutcome(gateCheck("pipeline-structure", looppreflight.LevelPass)))
			args := append([]string{"--preflight-only", "--project-root", root, "--goal-text", "g"}, extra...)
			var stdout, stderr bytes.Buffer
			rc := runLoop(args, nil, &stdout, &stderr)
			if rc != 10 {
				t.Fatalf("--preflight-only with %v must exit 10, got %d\nstderr:\n%s", extra, rc, stderr.String())
			}
			if strings.Contains(stderr.String(), "flag provided but not defined") {
				t.Fatalf("exit 10 came from an undefined flag, not the mode conflict:\n%s", stderr.String())
			}
			if !strings.Contains(stderr.String(), "mutually exclusive") {
				t.Errorf("stderr must say the modes are mutually exclusive:\n%s", stderr.String())
			}
			if probe.gateCalls != 0 || probe.launched {
				t.Errorf("a rejected combination must run nothing: gate=%d launched=%v", probe.gateCalls, probe.launched)
			}
			requireNoRunState(t, evolveDir)
		})
	}
}
