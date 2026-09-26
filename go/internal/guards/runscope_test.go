package guards

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRoleAndPhaseGuards_ReadOwnRunState(t *testing.T) {

	root := t.TempDir()
	// A fake non-/tmp worktree: on Linux t.TempDir() is under /tmp, where isAlwaysSafe decides before
	// cycle state is read. The role gate only compares prefixes, so the worktree need not exist.
	wt := "/work/wt-cycle-7"
	linkHost := filepath.Join(root, "wt-evolve")
	runWS := filepath.Join(root, ".evolve", "runs", "cycle-7")
	for _, d := range []string{filepath.Join(root, ".evolve"), runWS, linkHost} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// The host-global state belongs to a different concurrent run, in scout with no worktree.
	global := `{"cycle_id":99,"phase":"scout","workspace_path":"/elsewhere"}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "cycle-state.json"), []byte(global), 0o644); err != nil {
		t.Fatal(err)
	}
	own := `{"cycle_id":7,"phase":"build","workspace_path":"` + runWS + `","active_worktree":"` + wt + `"}`
	if err := os.WriteFile(filepath.Join(runWS, core.RunStateFile), []byte(own), 0o644); err != nil {
		t.Fatal(err)
	}
	// The core.linkGuardDeps layout: the worktree's cycle-state.json links to this run's run.json.
	if err := os.Symlink(filepath.Join(runWS, core.RunStateFile), filepath.Join(linkHost, "cycle-state.json")); err != nil {
		t.Fatal(err)
	}

	ownStore := storage.New(linkHost)
	ctx := context.Background()

	edit := core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": filepath.Join(wt, "go", "x.go")},
	}
	if d := NewRole(ownStore, false).Decide(ctx, edit); !d.Allow {
		t.Errorf("role gate read the wrong run's state: denied own-worktree build write: %s", d.Reason)
	}
	globalStore := storage.New(filepath.Join(root, ".evolve"))
	if d := NewRole(globalStore, false).Decide(ctx, edit); d.Allow {
		t.Error("fixture self-check: the global (other-run) state should deny this write")
	}

	agent := core.GuardInput{ToolName: "Agent", ToolInput: map[string]any{}}
	d := NewPhase(ownStore, false).Decide(ctx, agent)
	if d.Allow {
		t.Fatal("phase guard must deny Agent during the run's own active cycle")
	}
	if !strings.Contains(d.Reason, "build") || strings.Contains(d.Reason, "scout") {
		t.Errorf("phase guard denial must cite the run's OWN phase (build), got: %s", d.Reason)
	}
}
