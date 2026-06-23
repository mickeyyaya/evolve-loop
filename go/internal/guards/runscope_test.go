package guards

// CB.4 acceptance (concurrency campaign): the role-gate and the phase guard
// must decide on the run's OWN state when invoked from inside a cycle
// worktree. The on-disk layout below is exactly what core.linkGuardDeps
// provisions: <worktree>/.evolve/cycle-state.json is a symlink to the run
// workspace's run.json (the storage WriteCycleState dual-write mirror),
// while the host-global cycle-state.json may hold a DIFFERENT concurrent
// run's phase. Wired through the real filesystem storage adapter so the
// whole guard read path (symlink → run.json → CycleState) is pinned.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

func TestRoleAndPhaseGuards_ReadOwnRunState(t *testing.T) {

	root := t.TempDir()
	// The DECIDED paths use a fake non-/tmp worktree: on Linux t.TempDir()
	// is under /tmp, where isAlwaysSafe would short-circuit the role gate
	// before it ever reads cycle state — the decisions below must come from
	// the run-scoped state, on every OS. The role gate only compares path
	// prefixes, so this worktree never needs to exist on disk; the symlink
	// host directory (where the guard's --evolve-dir points) is real.
	wt := "/work/wt-cycle-7"
	linkHost := filepath.Join(root, "wt-evolve")
	runWS := filepath.Join(root, ".evolve", "runs", "cycle-7")
	for _, d := range []string{filepath.Join(root, ".evolve"), runWS, linkHost} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Host-global cycle-state.json: a DIFFERENT concurrent run, in scout,
	// with no active worktree — under the pre-CB.4 layout the role gate
	// would read this and deny the build write below.
	global := `{"cycle_id":99,"phase":"scout","workspace_path":"/elsewhere"}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "cycle-state.json"), []byte(global), 0o644); err != nil {
		t.Fatal(err)
	}
	// This run's own mirror: build phase, this worktree active.
	own := `{"cycle_id":7,"phase":"build","workspace_path":"` + runWS + `","active_worktree":"` + wt + `"}`
	if err := os.WriteFile(filepath.Join(runWS, core.RunStateFile), []byte(own), 0o644); err != nil {
		t.Fatal(err)
	}
	// The linkGuardDeps layout: worktree guard state → own run.json.
	if err := os.Symlink(filepath.Join(runWS, core.RunStateFile), filepath.Join(linkHost, "cycle-state.json")); err != nil {
		t.Fatal(err)
	}

	ownStore := storage.New(linkHost)
	ctx := context.Background()

	// Role gate: a build write inside the run's own worktree must be allowed
	// per the run's OWN phase/worktree …
	edit := core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": filepath.Join(wt, "go", "x.go")},
	}
	if d := NewRole(ownStore, false).Decide(ctx, edit); !d.Allow {
		t.Errorf("role gate read the wrong run's state: denied own-worktree build write: %s", d.Reason)
	}
	// … whereas the global view (the other run: scout, no worktree) denies it —
	// the exact cross-run poisoning CB.4 closes.
	globalStore := storage.New(filepath.Join(root, ".evolve"))
	if d := NewRole(globalStore, false).Decide(ctx, edit); d.Allow {
		t.Error("fixture self-check: the global (other-run) state should deny this write")
	}

	// Phase guard: Agent stays denied during the run's own cycle, and the
	// denial must cite the run's OWN phase (build), not the other run's.
	agent := core.GuardInput{ToolName: "Agent", ToolInput: map[string]any{}}
	d := NewPhase(ownStore, false).Decide(ctx, agent)
	if d.Allow {
		t.Fatal("phase guard must deny Agent during the run's own active cycle")
	}
	if !strings.Contains(d.Reason, "build") || strings.Contains(d.Reason, "scout") {
		t.Errorf("phase guard denial must cite the run's OWN phase (build), got: %s", d.Reason)
	}
}
