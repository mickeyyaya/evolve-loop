package gc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

func TestIsLive_AClosedOutRunsStalePointerDoesNotKeepItsWorktreeLive(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	wt := filepath.Join(evolveDir, "worktrees", "cycle-cd3ae73e-1757")
	for _, c := range []string{"cycle-1757", "cycle-1762"} {
		if err := os.MkdirAll(filepath.Join(evolveDir, "runs", c), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	live := filepath.Join(evolveDir, "worktrees", "cycle-cd3ae73e-1762")
	write := func(p, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(evolveDir, "runs", "cycle-1757", "run.json"), `{"active_worktree":"`+wt+`"}`)
	write(filepath.Join(evolveDir, "runs", "cycle-1762", "run.json"), `{"active_worktree":"`+live+`"}`)
	write(filepath.Join(dossier.CyclesDir(root), "cycle-1757.json"), `{}`)
	o := WorktreeOptions{ProjectRoot: root, EvolveDir: evolveDir, PidAlive: func(int) bool { return false }}

	if o.isLive(wt) {
		t.Error("cycle 1757 closed out: its run.json's active_worktree is a stale pointer, not liveness (gc reaped none of 87 worktrees)")
	}
	if !o.isLive(live) {
		t.Error("cycle 1762 has no dossier: its run.json still protects its worktree")
	}
}

func TestIsLive_ANonCycleRunDirsPointerIsNeverDiscountedByACycleDossier(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	wt := filepath.Join(evolveDir, "worktrees", "cycle-cd3ae73e-1700")
	runJSON := filepath.Join(evolveDir, "runs", "reset-sealed-1700", "run.json")
	if err := os.MkdirAll(filepath.Dir(runJSON), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runJSON, []byte(`{"active_worktree":"`+wt+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dossier.CyclesDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dossier.CyclesDir(root), "cycle-1700.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	o := WorktreeOptions{ProjectRoot: root, EvolveDir: evolveDir, PidAlive: func(int) bool { return false }}

	if !o.isLive(wt) {
		t.Error("a run dir without the cycle- prefix names no cycle, so cycle 1700's dossier must not discount its pointer")
	}
}
