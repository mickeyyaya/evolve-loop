package subagent

// run_project_root_env_test.go — the subprocess sees the plane's root.
//
// core/phase.go documents PhaseRequest.ProjectRoot as "what a subprocess sees
// as EVOLVE_PROJECT_ROOT", and every phase runs with cwd = its cycle worktree
// (CB.1). Nothing exported the variable. Any `evolve` subcommand the agent
// runs — `inbox-mover claim` first among them — resolves its root through
// cmdutil.EnvOrCwd, so it fell back to the worktree, whose .evolve/inbox is a
// git-tracked SNAPSHOT of the plane's queue. Batch cycle 1631 (2026-09-12)
// printed `[inbox-mover] claimed:` against that copy while the plane's item
// never moved and its ledger recorded no claim; the cycle then FAILed with
// its lane's item still queued.

import (
	"context"
	"strings"
	"testing"
)

// TestRun_ExportsProjectRootToTheSubprocess pins the documented contract at
// the seam the adapter actually receives.
func TestRun_ExportsProjectRootToTheSubprocess(t *testing.T) {
	workspace := t.TempDir()
	projectRoot := t.TempDir() // deliberately NOT the workspace's parent
	worktree := t.TempDir()    // and not the worktree the agent runs in
	opts := runHappyOpts(t)
	// Capture the env, then let the fixture's own adapter materialize the
	// artifact exactly as every other happy-path test does.
	var captured map[string]string
	orig := opts.ExecAdapter
	opts.ExecAdapter = func(ctx context.Context, adapter string, env map[string]string) (int, error) {
		captured = env
		return orig(ctx, adapter, env)
	}
	if _, err := Run(context.Background(), RunRequest{
		Agent:         "triage",
		Cycle:         1631,
		WorkspacePath: workspace,
		ProjectRoot:   projectRoot,
		WorktreePath:  worktree,
		PromptReader:  strings.NewReader("hi"),
	}, opts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := captured["EVOLVE_PROJECT_ROOT"]; got != projectRoot {
		t.Fatalf("EVOLVE_PROJECT_ROOT handed to the adapter = %q, want the plane root %q — "+
			"without it, `evolve inbox-mover claim` run from the worktree cwd resolves the worktree's tracked inbox copy (cycle 1631)", got, projectRoot)
	}
}

// TestRun_NoProjectRootMeansNoEnvKey mirrors the tmux guard for headless
// drivers: no key rather than an empty value.
func TestRun_NoProjectRootMeansNoEnvKey(t *testing.T) {
	opts := runHappyOpts(t)
	var captured map[string]string
	orig := opts.ExecAdapter
	opts.ExecAdapter = func(ctx context.Context, adapter string, env map[string]string) (int, error) {
		captured = env
		return orig(ctx, adapter, env)
	}
	if _, err := Run(context.Background(), RunRequest{
		Agent: "triage", Cycle: 1, WorkspacePath: t.TempDir(), PromptReader: strings.NewReader("hi"),
	}, opts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if v, present := captured["EVOLVE_PROJECT_ROOT"]; present {
		t.Fatalf("an empty project root produced the env key (value %q); the contract is unset ⇒ cwd", v)
	}
}
