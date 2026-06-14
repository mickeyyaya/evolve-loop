//go:build integration

package core

// CB.4 (concurrency campaign): guard hooks inside a cycle worktree must read
// the run's OWN state. linkGuardDeps therefore points the worktree's
// .evolve/cycle-state.json symlink at the run workspace's run.json (the
// WriteCycleState dual-write mirror), NOT at the host-global
// cycle-state.json — the global file holds whichever concurrent run wrote
// last. state.json and ledger.jsonl stay host-global: the per-run events
// ledger is CC.1; retargeting the ledger link before it exists would have
// the chain guard verifying an empty file (a vacuous pass).

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitWorktree_GuardStateIsRunScoped(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "init")

	// The host-global cycle-state.json belongs to a DIFFERENT concurrent run.
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	global := `{"cycle_id":99,"phase":"scout","workspace_path":"/elsewhere"}`
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), []byte(global), 0o644); err != nil {
		t.Fatal(err)
	}
	// This run's own state, as the WriteCycleState mirror would persist it.
	runWS := RunWorkspacePath(root, 77)
	if err := os.MkdirAll(runWS, 0o755); err != nil {
		t.Fatal(err)
	}
	own := `{"cycle_id":77,"phase":"build","workspace_path":"` + runWS + `"}`
	if err := os.WriteFile(filepath.Join(runWS, RunStateFile), []byte(own), 0o644); err != nil {
		t.Fatal(err)
	}

	g := gitWorktree{}
	wt, err := g.Create(root, 77)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer func() { _ = g.Cleanup(root, wt) }()

	// Symlink pin: the worktree's guard-visible cycle-state resolves to the
	// run's own run.json.
	link := filepath.Join(wt, ".evolve", "cycle-state.json")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("worktree .evolve/cycle-state.json should be a symlink: %v", err)
	}
	if want := filepath.Join(runWS, RunStateFile); target != want {
		t.Errorf("cycle-state.json symlink target = %q, want per-run %q", target, want)
	}

	// Behavioral pin: reading through the worktree path yields the run's OWN
	// phase, not the concurrent run's.
	raw, err := os.ReadFile(link)
	if err != nil {
		t.Fatalf("read through worktree cycle-state link: %v", err)
	}
	var cs CycleState
	if err := json.Unmarshal(raw, &cs); err != nil {
		t.Fatal(err)
	}
	if cs.CycleID != 77 || cs.Phase != "build" {
		t.Errorf("worktree guard state = cycle %d phase %q; want own run (cycle 77 phase \"build\") — global state leaked in", cs.CycleID, cs.Phase)
	}

	// state.json + ledger.jsonl stay host-global until CC.1 (per-run events).
	for _, f := range []string{"state.json", "ledger.jsonl"} {
		got, err := os.Readlink(filepath.Join(wt, ".evolve", f))
		if err != nil {
			t.Errorf("worktree .evolve/%s should be a symlink: %v", f, err)
			continue
		}
		if want := filepath.Join(root, ".evolve", f); got != want {
			t.Errorf("%s symlink target = %q, want host-global %q", f, got, want)
		}
	}
}
