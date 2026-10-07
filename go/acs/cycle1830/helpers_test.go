//go:build acs

package cycle1830

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	fixtureCycle              = 4242
	fixtureRunID              = "01FIXTURERUNC18300000000000"
	liveOwnerRunID            = "01LIVEOWNERC18300000000000"
	fixtureGoalHash           = "c1830c1830c1830c1830c1830c1830c1830c1830c1830c1830c1830c1830c18"
	fixtureBaseSHA            = "1830183018301830183018301830183018301830"
	fixtureExplanationVersion = 1
	exitOK                    = 0
	exitDerivationRefused     = 1
	exitUsage                 = 10
	exitStdinDecode           = 11
)

var (
	buildOnce    sync.Once
	buildDir     string
	buildPath    string
	buildFailure string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if buildDir != "" {
		if err := os.RemoveAll(buildDir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle1830: remove %s: %v\n", buildDir, err)
		}
	}
	os.Exit(code)
}

type cycleFixture struct {
	root     string
	runDir   string
	worktree string
	state    cyclestate.CycleState
}

func newCycleFixture(t *testing.T) cycleFixture {
	t.Helper()
	root := t.TempDir()
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(fixtureCycle))
	worktree := filepath.Join(root, ".evolve", "worktrees", "cycle-fixture-"+strconv.Itoa(fixtureCycle))
	for _, dir := range []string{runDir, worktree} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("fixture: mkdir %s: %v", dir, err)
		}
	}
	fx := cycleFixture{root: root, runDir: runDir, worktree: worktree, state: cyclestate.CycleState{
		CycleID:                         fixtureCycle,
		Phase:                           "tdd",
		WorkspacePath:                   runDir,
		ActiveWorktree:                  worktree,
		WorktreeBaseSHA:                 fixtureBaseSHA,
		RunID:                           fixtureRunID,
		GoalHash:                        fixtureGoalHash,
		ExplanationDocumentationVersion: fixtureExplanationVersion,
	}}
	fx.writeState(t, fx.state)
	return fx
}

func (fx cycleFixture) statePath() string { return filepath.Join(fx.runDir, "cycle-state.json") }

func (fx cycleFixture) writeState(t *testing.T, state cyclestate.CycleState) {
	t.Helper()
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("fixture: marshal cycle state: %v", err)
	}
	if err := os.WriteFile(fx.statePath(), raw, 0o644); err != nil {
		t.Fatalf("fixture: write %s: %v", fx.statePath(), err)
	}
}

func (fx cycleFixture) holdLease(t *testing.T, heartbeat time.Time) {
	t.Helper()
	lease := runlease.Lease{RunID: liveOwnerRunID, OwnerPID: os.Getpid()}
	if err := runlease.Write(fx.runDir, lease, heartbeat); err != nil {
		t.Fatalf("fixture: write lease in %s: %v", fx.runDir, err)
	}
}

func (fx cycleFixture) cycleFlags() []string {
	return []string{"--cycle", strconv.Itoa(fixtureCycle), "--project-root", fx.root}
}

func removeOrFail(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("fixture: remove %s: %v", path, err)
	}
}

type refusal struct {
	name    string
	arrange func(t *testing.T, fx cycleFixture)
	reasons func(fx cycleFixture) []string
}

func derivationRefusals() []refusal {
	namesTheStateFile := func(cycleFixture) []string { return []string{"cycle-state", "cycle state", "state not found"} }
	namesTheWorktree := func(fx cycleFixture) []string { return []string{fx.worktree, "worktree"} }
	return []refusal{
		{"no cycle-state.json", func(t *testing.T, fx cycleFixture) { removeOrFail(t, fx.statePath()) }, namesTheStateFile},
		{"no run directory for the cycle", func(t *testing.T, fx cycleFixture) { removeOrFail(t, fx.runDir) }, namesTheStateFile},
		{"an unparseable cycle-state.json", func(t *testing.T, fx cycleFixture) {
			if err := os.WriteFile(fx.statePath(), []byte(`{"cycle_id": 4242, "active_worktree":`), 0o644); err != nil {
				t.Fatalf("fixture: corrupt %s: %v", fx.statePath(), err)
			}
		}, func(cycleFixture) []string { return []string{"cycle-state", "cycle state", "unparseable"} }},
		{"the worktree directory is gone", func(t *testing.T, fx cycleFixture) { removeOrFail(t, fx.worktree) }, namesTheWorktree},
		{"no worktree recorded in the state", func(t *testing.T, fx cycleFixture) {
			stateWithoutWorktree := fx.state
			stateWithoutWorktree.ActiveWorktree = ""
			fx.writeState(t, stateWithoutWorktree)
		}, func(cycleFixture) []string { return []string{"worktree"} }},
		{"a live run lease holds the cycle", func(t *testing.T, fx cycleFixture) { fx.holdLease(t, time.Now()) },
			func(cycleFixture) []string { return []string{liveOwnerRunID, strconv.Itoa(os.Getpid())} }},
	}
}

func namesAny(text string, reasons []string) bool {
	lowered := strings.ToLower(text)
	for _, reason := range reasons {
		if reason != "" && strings.Contains(lowered, strings.ToLower(reason)) {
			return true
		}
	}
	return false
}

type result struct {
	stdout, stderr string
	code           int
}

func (r result) String() string {
	return fmt.Sprintf("exit=%d\n--- stdout ---\n%s--- stderr ---\n%s", r.code, r.stdout, r.stderr)
}

type recordingScout struct {
	dispatched []core.PhaseRequest
}

func (s *recordingScout) Name() string { return "scout" }

func (s *recordingScout) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	s.dispatched = append(s.dispatched, req)
	return core.PhaseResponse{Phase: "scout", Verdict: core.VerdictPASS}, nil
}

func registerRecordingScout(t *testing.T) *recordingScout {
	t.Helper()
	t.Cleanup(registry.SnapshotForTest())
	registry.ResetForTesting()
	scout := &recordingScout{}
	registry.Register("scout", func(core.PhaseRequest) core.PhaseRunner { return scout })
	return scout
}

func runPhaseCommand(stdin string, args ...string) result {
	var stdout, stderr bytes.Buffer
	code := phasecmd.NewRunPhase(nil, nil)(args, strings.NewReader(stdin), &stdout, &stderr)
	return result{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func sameDir(a, b string) bool {
	if a == b {
		return true
	}
	resolvedA, errA := filepath.EvalSymlinks(a)
	resolvedB, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && resolvedA == resolvedB
}

func assertDerivedFromState(t *testing.T, got core.PhaseRequest, fx cycleFixture) {
	t.Helper()
	fields := []struct {
		name      string
		got, want any
	}{
		{"Cycle", got.Cycle, fx.state.CycleID},
		{"Workspace", got.Workspace, fx.state.WorkspacePath},
		{"Worktree", got.Worktree, fx.state.ActiveWorktree},
		{"WorktreeBaseSHA", got.WorktreeBaseSHA, fx.state.WorktreeBaseSHA},
		{"RunID", got.RunID, fx.state.RunID},
		{"GoalHash", got.GoalHash, fx.state.GoalHash},
		{"ExplanationDocumentationVersion", got.ExplanationDocumentationVersion, fx.state.ExplanationDocumentationVersion},
	}
	for _, f := range fields {
		if f.got != f.want {
			t.Errorf("derived request %s = %v, want %v from %s", f.name, f.got, f.want, fx.statePath())
		}
	}
	if !sameDir(got.ProjectRoot, fx.root) {
		t.Errorf("derived request ProjectRoot = %q, want the --project-root %q", got.ProjectRoot, fx.root)
	}
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "cycle1830-evolve-")
		if err != nil {
			buildFailure = err.Error()
			return
		}
		buildDir, buildPath = dir, filepath.Join(dir, "evolve")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", buildPath, "./cmd/evolve")
		cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
		cmd.WaitDelay = 5 * time.Second
		if out, err := cmd.CombinedOutput(); err != nil {
			buildFailure = fmt.Sprintf("%v\n%s", err, out)
		}
	})
	if buildFailure != "" {
		t.Fatalf("go build ./cmd/evolve: %s", buildFailure)
	}
	return buildPath
}

func sandboxEnv(projectRoot string) []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "TMUX") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "EVOLVE_PROJECT_ROOT="+projectRoot)
}

func runEvolve(t *testing.T, fx cycleFixture, stdin string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBinary(t), args...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = fx.root, sandboxEnv(fx.root), 5*time.Second
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := result{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		r.code = exitErr.ExitCode()
	default:
		t.Fatalf("evolve %v: %v", args, err)
	}
	return r
}

func composeDryRun(fx cycleFixture) []string {
	return append([]string{"compose", "--phases", "scout,triage", "--dry-run"}, fx.cycleFlags()...)
}
