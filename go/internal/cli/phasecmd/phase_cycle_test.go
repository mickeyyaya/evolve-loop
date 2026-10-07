package phasecmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func writeRerunFixture(t *testing.T, cycle int) (string, core.CycleState) {
	t.Helper()
	root := t.TempDir()
	worktree := filepath.Join(root, "wt")
	runDir := paths.RunWorkspace(root, cycle)
	for _, dir := range []string{worktree, runDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	state := core.CycleState{CycleID: cycle, ActiveWorktree: worktree, RunID: "01RERUN", GoalHash: "goal", WorktreeBaseSHA: "base"}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, core.CycleStateFile), raw, 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	return root, state
}

func TestRequestForCycle_DerivesTheRequestAndFallsBackToTheRunDirWorkspace(t *testing.T) {
	root, state := writeRerunFixture(t, 9)

	req, err := RequestForCycle(root, 9, strings.NewReader(" \n"))
	if err != nil {
		t.Fatalf("RequestForCycle: %v", err)
	}
	want := core.PhaseRequest{Cycle: 9, ProjectRoot: root, Workspace: paths.RunWorkspace(root, 9), Worktree: state.ActiveWorktree,
		WorktreeBaseSHA: "base", RunID: "01RERUN", GoalHash: "goal"}
	if !reflect.DeepEqual(req, want) {
		t.Errorf("RequestForCycle = %+v, want %+v", req, want)
	}
}

func TestRequestForCycle_ExitCodeSeparatesUsageFromRefusal(t *testing.T) {
	cases := []struct {
		name     string
		cycle    int
		stdin    io.Reader
		wantCode int
		wantText string
	}{
		{"a non-positive cycle is usage", -3, nil, exitCycleUsage, "positive"},
		{"a stdin request beside --cycle is usage", 9, strings.NewReader(`{"cycle":9}`), exitCycleUsage, "stdin"},
		{"a cycle without state is refused", 10, nil, exitCycleRefused, "state not found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := writeRerunFixture(t, 9)

			_, err := RequestForCycle(root, tc.cycle, tc.stdin)
			if err == nil {
				t.Fatalf("RequestForCycle(cycle=%d) succeeded, want an error", tc.cycle)
			}
			if got := CycleRequestExitCode(err); got != tc.wantCode {
				t.Errorf("CycleRequestExitCode(%v) = %d, want %d", err, got, tc.wantCode)
			}
			if !strings.Contains(err.Error(), tc.wantText) {
				t.Errorf("error %q must name %q", err, tc.wantText)
			}
		})
	}
}
