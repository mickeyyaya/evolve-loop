package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const composeFixtureCycle = 4242

type requestRecorder struct {
	ran      []string
	requests []core.PhaseRequest
}

func (r *requestRecorder) register(names ...string) {
	registry.ResetForTesting()
	for _, name := range names {
		registry.Register(name, func(core.PhaseRequest) core.PhaseRunner { return recordedRunner{rec: r, name: name} })
	}
}

type recordedRunner struct {
	rec  *requestRecorder
	name string
}

func (p recordedRunner) Name() string { return p.name }

func (p recordedRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	p.rec.ran = append(p.rec.ran, p.name)
	p.rec.requests = append(p.rec.requests, req)
	return core.PhaseResponse{Phase: p.name, Verdict: core.VerdictPASS}, nil
}

func composeCycleFixture(t *testing.T) (string, cyclestate.CycleState) {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	worktree := filepath.Join(evolveDir, "worktrees", "cycle-fixture-4242")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", worktree, err)
	}
	state := cyclestate.CycleState{
		CycleID:                         composeFixtureCycle,
		Phase:                           "tdd",
		WorkspacePath:                   filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(composeFixtureCycle)),
		ActiveWorktree:                  worktree,
		WorktreeBaseSHA:                 "4242424242424242424242424242424242424242",
		RunID:                           "01COMPOSEFIXTURE0000000000",
		GoalHash:                        "compose-cycle-fixture-goal",
		ExplanationDocumentationVersion: 1,
	}
	body, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal cycle state: %v", err)
	}
	writeCycleStateN(t, evolveDir, composeFixtureCycle, string(body))
	return root, state
}

func composeCycleArgs(root string) []string {
	return []string{"--phases", "scout,triage", "--cycle", strconv.Itoa(composeFixtureCycle), "--project-root", root}
}

func sameResolvedDir(a, b string) bool {
	resolvedA, errA := filepath.EvalSymlinks(a)
	resolvedB, errB := filepath.EvalSymlinks(b)
	return a == b || (errA == nil && errB == nil && resolvedA == resolvedB)
}

func TestCompose_CycleFlag_RunsEveryPhaseWithTheOneDerivedRequest(t *testing.T) {
	defer registry.SnapshotForTest()()
	rec := &requestRecorder{}
	rec.register("scout", "triage")
	root, state := composeCycleFixture(t)

	var stdout, stderr bytes.Buffer
	code := runCompose(composeCycleArgs(root), strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d, want 0; stderr=%s", code, stderr.String())
	}
	if !slices.Equal(rec.ran, []string{"scout", "triage"}) {
		t.Fatalf("ran %v, want [scout triage]", rec.ran)
	}
	if !reflect.DeepEqual(rec.requests[0], rec.requests[1]) {
		t.Errorf("scout and triage ran with different requests:\n scout=%+v\ntriage=%+v", rec.requests[0], rec.requests[1])
	}
	got := rec.requests[0]
	derived := core.PhaseRequest{
		Cycle:                           state.CycleID,
		Workspace:                       state.WorkspacePath,
		Worktree:                        state.ActiveWorktree,
		WorktreeBaseSHA:                 state.WorktreeBaseSHA,
		RunID:                           state.RunID,
		GoalHash:                        state.GoalHash,
		ExplanationDocumentationVersion: state.ExplanationDocumentationVersion,
	}
	if got.Cycle != derived.Cycle || got.Workspace != derived.Workspace || got.Worktree != derived.Worktree ||
		got.WorktreeBaseSHA != derived.WorktreeBaseSHA || got.RunID != derived.RunID || got.GoalHash != derived.GoalHash ||
		got.ExplanationDocumentationVersion != derived.ExplanationDocumentationVersion {
		t.Errorf("composed request %+v, want the cycle-state.json fields %+v", got, derived)
	}
	if !sameResolvedDir(got.ProjectRoot, root) {
		t.Errorf("composed request ProjectRoot=%q, want --project-root %q", got.ProjectRoot, root)
	}
	if !got.ComposePhases {
		t.Errorf("composed request must keep ComposePhases=true")
	}
}

func TestCompose_CycleFlag_ARefusedDerivationRunsNoPhase(t *testing.T) {
	cases := []struct {
		name     string
		arrange  func(t *testing.T, state cyclestate.CycleState)
		stdin    string
		wantCode int
		reasons  []string
	}{
		{"a live run lease", func(t *testing.T, state cyclestate.CycleState) {
			if err := runlease.Write(state.WorkspacePath, runlease.Lease{RunID: "01LIVEOWNERCOMPOSE00000000", OwnerPID: os.Getpid()}, time.Now()); err != nil {
				t.Fatalf("write lease: %v", err)
			}
		}, "", 1, []string{"01LIVEOWNERCOMPOSE00000000", strconv.Itoa(os.Getpid())}},
		{"the worktree is gone", func(t *testing.T, state cyclestate.CycleState) {
			if err := os.RemoveAll(state.ActiveWorktree); err != nil {
				t.Fatalf("remove worktree: %v", err)
			}
		}, "", 1, []string{"worktree"}},
		{"no cycle-state.json", func(t *testing.T, state cyclestate.CycleState) {
			if err := os.Remove(filepath.Join(state.WorkspacePath, "cycle-state.json")); err != nil {
				t.Fatalf("remove cycle-state.json: %v", err)
			}
		}, "", 1, []string{"cycle-state", "cycle state", "state not found"}},
		{"a request on stdin as well", func(*testing.T, cyclestate.CycleState) {}, `{"cycle":4242}`, 10, []string{"stdin"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer registry.SnapshotForTest()()
			rec := &requestRecorder{}
			rec.register("scout", "triage")
			root, state := composeCycleFixture(t)
			tc.arrange(t, state)

			var stdout, stderr bytes.Buffer
			code := runCompose(composeCycleArgs(root), strings.NewReader(tc.stdin), &stdout, &stderr)
			if code != tc.wantCode {
				t.Errorf("code=%d, want %d; stderr=%s", code, tc.wantCode, stderr.String())
			}
			if len(rec.ran) != 0 {
				t.Errorf("a refused derivation must run no phase; ran %v", rec.ran)
			}
			lowered := strings.ToLower(stderr.String())
			if !slices.ContainsFunc(tc.reasons, func(reason string) bool { return strings.Contains(lowered, strings.ToLower(reason)) }) {
				t.Errorf("stderr must name the reason (any of %q); got %q", tc.reasons, stderr.String())
			}
		})
	}
}
