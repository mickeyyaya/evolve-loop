package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cmd_subagent_workspace_test.go is the cycle-616 regression for the
// fable5_deep_scan finding "subagent-workspace-absolutize": cmd_subagent.go
// passes the LLM-typed <workspace_path> positional argument (runSubagentRun,
// runSubagentDispatchParallel, runSubagentCachePrefix's --workspace) straight
// through to subagent/run.go and bridge/engine.go, which only os.Stat/MkdirAll
// it — no absolutization, no validation. A relative arg resolves against
// whatever the process's ambient CWD happens to be at invocation time
// (confirmed in the repo tree by untracked go/tmux-sessions.jsonl and
// go/.bridge-inbox/ scattered by exactly this path), which is the class of
// untracked-tree-drift the tree-diff cycle-killer guard reacts to (see the
// boundary_only_main_tree_writes standing rule).
//
// This test targets `run`, the highest-traffic of the three call sites. It
// proves the CURRENT behavior black-box: point EVOLVE_PROJECT_ROOT at one temp
// dir, chdir the test process into a DIFFERENT (empty) temp dir, and invoke
// `evolve subagent run <agent> <cycle> <relative-workspace>` where
// <relative-workspace> exists under the project root but NOT under cwd.
//
//   - Today: workspace is used exactly as typed, so
//     os.Stat("<relative-workspace>") resolves against cwd (the empty temp
//     dir), fails, and subagent/run.go returns "workspace dir does not exist" —
//     even though the SAME relative path is a real, valid directory one level
//     up under the project root. This is the observable symptom of "no
//     absolutization at the ingestion point."
//   - After the fix: the ingestion point must resolve a relative workspace arg
//     against a known base (EVOLVE_PROJECT_ROOT / layout.ProjectRoot) — NOT
//     raw cwd — before handing it to subagent.Run, so the stat succeeds and
//     the run proceeds (failing later for an unrelated, expected reason: no
//     profile fixture is set up in this test). Either way, nothing may be
//     created under the invoking cwd.
func TestRunSubagentRun_RelativeWorkspaceResolvesAgainstProjectRootNotCwd(t *testing.T) {
	projectRoot := t.TempDir()
	cwd := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	t.Setenv("EVOLVE_PROJECT_ROOT", projectRoot)
	t.Setenv("PROMPT_FILE_OVERRIDE", "") // force stdin path is irrelevant; we fail before reading it

	const rel = "runs/cycle-9"
	if err := os.MkdirAll(filepath.Join(projectRoot, rel), 0o755); err != nil {
		t.Fatalf("seed project-root workspace: %v", err)
	}

	var stdout, stderr bytes.Buffer
	promptFile := filepath.Join(projectRoot, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte("prompt\n"), 0o644); err != nil {
		t.Fatalf("write prompt fixture: %v", err)
	}
	t.Setenv("PROMPT_FILE_OVERRIDE", promptFile)

	_ = runSubagentRun([]string{"builder", "9", rel}, &stdout, &stderr)

	if strings.Contains(stderr.String(), "workspace dir does not exist") {
		t.Errorf("relative --workspace %q must resolve against EVOLVE_PROJECT_ROOT (%s), not the raw "+
			"invoking cwd (%s) — got: %s", rel, projectRoot, cwd, stderr.String())
	}

	entries, err := os.ReadDir(cwd)
	if err != nil {
		t.Fatalf("readdir cwd: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("relative workspace arg must not scatter artifacts into the invoking cwd; found: %v", names)
	}
}

// TestRunSubagentRun_AbsoluteWorkspacePassesThroughUnchanged is regression
// coverage: the fix for relative args must not disturb the existing (working)
// absolute-path contract every real loop invocation already relies on.
func TestRunSubagentRun_AbsoluteWorkspacePassesThroughUnchanged(t *testing.T) {
	projectRoot := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "runs", "cycle-9")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("seed absolute workspace: %v", err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", projectRoot)

	promptFile := filepath.Join(projectRoot, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte("prompt\n"), 0o644); err != nil {
		t.Fatalf("write prompt fixture: %v", err)
	}
	t.Setenv("PROMPT_FILE_OVERRIDE", promptFile)

	var stdout, stderr bytes.Buffer
	_ = runSubagentRun([]string{"builder", "9", workspace}, &stdout, &stderr)

	if strings.Contains(stderr.String(), "workspace dir does not exist") {
		t.Errorf("an already-absolute --workspace must be used as-is; got: %s", stderr.String())
	}
}

// --- Test-amplification additions (cycle 616 black-box adversarial pass) ---
//
// The AC for subagent-workspace-absolutize names THREE ingestion points
// (runSubagentRun, runSubagentDispatchParallel, runSubagentCachePrefix), but
// the TDD RED test above only exercises `run` — "the highest-traffic of the
// three call sites" per its own comment. The tests below extend the same
// black-box contract (relative workspace resolves against project root, not
// cwd; nothing is ever written into the invoking cwd) to the other two
// ingestion points, plus edge/limit inputs (empty, parent-traversal,
// deeply-nested) against the `run` call site the RED test already covers.

// TestRunSubagentCachePrefix_RelativeWorkspaceResolvesAgainstProjectRootNotCwd
// exercises the SECOND ingestion point named in the build report:
// runSubagentCachePrefix resolves its own `--project-root` for a different
// purpose and, per the build report, "can reuse the same value" for
// `--workspace`. A relative `--workspace` must resolve against
// `--project-root`, not the invoking cwd, and the emitted artifact's
// `workspace=` metadata line must reflect the resolved absolute path.
func TestRunSubagentCachePrefix_RelativeWorkspaceResolvesAgainstProjectRootNotCwd(t *testing.T) {
	projectRoot := t.TempDir()
	cwd := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	const rel = "runs/cycle-9"
	if err := os.MkdirAll(filepath.Join(projectRoot, rel), 0o755); err != nil {
		t.Fatalf("seed project-root workspace: %v", err)
	}
	out := filepath.Join(projectRoot, "out.md")

	var stdout, stderr bytes.Buffer
	rc := runSubagentCachePrefix([]string{
		"--cycle", "9",
		"--agent", "scout",
		"--workspace", rel,
		"--out", out,
		"--project-root", projectRoot,
	}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%q", rc, stderr.String())
	}

	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("output not written: %v", err)
	}
	wantWorkspace := filepath.Join(projectRoot, rel)
	if !strings.Contains(string(body), "workspace="+wantWorkspace) {
		t.Errorf("relative --workspace %q must resolve against --project-root (%s); "+
			"expected metadata to contain workspace=%s, got: %s", rel, projectRoot, wantWorkspace, body)
	}

	entries, err := os.ReadDir(cwd)
	if err != nil {
		t.Fatalf("readdir cwd: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("relative --workspace must not scatter artifacts into the invoking cwd; found: %v", entries)
	}
}

// TestRunSubagentDispatchParallel_RelativeWorkspaceDoesNotPolluteCwd exercises
// the THIRD ingestion point named in the build report. Unlike `run` and
// `cache-prefix`, dispatch-parallel's happy path requires substantial
// additional fixture scaffolding (an agent profile marked parallel-eligible
// with parallel_subtasks configured) that lies outside this phase's black-box
// budget and, per the test-amplifier's anti-bias mandate, must not be
// discovered by reading cmd_subagent.go itself. Regardless of how far
// dispatch-parallel gets before failing, the cwd-pollution invariant the AC
// promises ("no artifacts land in cwd for a relative-path invocation") must
// hold even on the early-failure path — this is the implementation-agnostic
// slice of the contract that is safe to pin without that scaffolding.
func TestRunSubagentDispatchParallel_RelativeWorkspaceDoesNotPolluteCwd(t *testing.T) {
	projectRoot := t.TempDir()
	cwd := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	t.Setenv("EVOLVE_PROJECT_ROOT", projectRoot)

	const rel = "runs/cycle-9"
	if err := os.MkdirAll(filepath.Join(projectRoot, rel), 0o755); err != nil {
		t.Fatalf("seed project-root workspace: %v", err)
	}

	var stdout, stderr bytes.Buffer
	_ = runSubagentDispatchParallel([]string{"builder", "9", rel}, &stdout, &stderr)

	if strings.Contains(stderr.String(), "workspace dir does not exist") {
		t.Errorf("relative --workspace %q must resolve against EVOLVE_PROJECT_ROOT (%s), not the "+
			"invoking cwd (%s); got: %s", rel, projectRoot, cwd, stderr.String())
	}

	entries, err := os.ReadDir(cwd)
	if err != nil {
		t.Fatalf("readdir cwd: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("relative workspace arg must not scatter artifacts into the invoking cwd "+
			"even when dispatch-parallel fails for an unrelated (profile/config) reason; found: %v", entries)
	}
}

// TestRunSubagentRun_EmptyWorkspaceArgDoesNotPanicOrPolluteCwd is the null/empty
// boundary case: an empty positional workspace argument (e.g. a template
// expansion gone wrong upstream) must fail cleanly — no panic, no crash loop,
// and critically no fallback to "resolve against cwd" (an empty relative
// value must not silently become the cwd itself).
func TestRunSubagentRun_EmptyWorkspaceArgDoesNotPanicOrPolluteCwd(t *testing.T) {
	projectRoot := t.TempDir()
	cwd := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	t.Setenv("EVOLVE_PROJECT_ROOT", projectRoot)

	promptFile := filepath.Join(projectRoot, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte("prompt\n"), 0o644); err != nil {
		t.Fatalf("write prompt fixture: %v", err)
	}
	t.Setenv("PROMPT_FILE_OVERRIDE", promptFile)

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("runSubagentRun panicked on empty workspace arg: %v", r)
			}
		}()
		var stdout, stderr bytes.Buffer
		rc := runSubagentRun([]string{"builder", "9", ""}, &stdout, &stderr)
		if rc == 0 {
			t.Errorf("empty --workspace must not be treated as a valid workspace; rc=0 stdout=%q", stdout.String())
		}
	}()

	entries, err := os.ReadDir(cwd)
	if err != nil {
		t.Fatalf("readdir cwd: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("an empty workspace arg must not resolve to (or pollute) the invoking cwd; found: %v", entries)
	}
}

// TestRunSubagentRun_DeeplyNestedRelativeWorkspaceResolvesAgainstProjectRoot is
// the large-scale/limit case: a relative path with many path segments (as a
// deeply-nested cycle/worker layout could plausibly produce) must resolve
// correctly against the project root, exactly like a single-segment relative
// path — the join must not truncate, mis-index, or otherwise mishandle a long
// component chain.
func TestRunSubagentRun_DeeplyNestedRelativeWorkspaceResolvesAgainstProjectRoot(t *testing.T) {
	projectRoot := t.TempDir()
	cwd := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	t.Setenv("EVOLVE_PROJECT_ROOT", projectRoot)

	rel := filepath.Join("runs", "cycle-9", "fanout", "workers", "scout", "w0", "artifacts", "deep")
	if err := os.MkdirAll(filepath.Join(projectRoot, rel), 0o755); err != nil {
		t.Fatalf("seed project-root workspace: %v", err)
	}

	promptFile := filepath.Join(projectRoot, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte("prompt\n"), 0o644); err != nil {
		t.Fatalf("write prompt fixture: %v", err)
	}
	t.Setenv("PROMPT_FILE_OVERRIDE", promptFile)

	var stdout, stderr bytes.Buffer
	_ = runSubagentRun([]string{"builder", "9", rel}, &stdout, &stderr)

	if strings.Contains(stderr.String(), "workspace dir does not exist") {
		t.Errorf("deeply-nested relative --workspace %q must resolve against EVOLVE_PROJECT_ROOT (%s); "+
			"got: %s", rel, projectRoot, stderr.String())
	}

	entries, err := os.ReadDir(cwd)
	if err != nil {
		t.Fatalf("readdir cwd: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("deeply-nested relative workspace arg must not scatter artifacts into the invoking cwd; found: %v", entries)
	}
}

// TestRunSubagentRun_WorkspaceOutsideProjectRootRejected is the cycle-619
// containment slice of subagent-workspace-absolutize. Cycle 616 landed
// absolutization (a relative arg resolves against project root, not cwd) but
// left the ".." escape unhandled — a relative workspace like "../sibling"
// still resolves to a real directory OUTSIDE the project root and then gets
// os.Stat'd + MkdirAll'd (workers/, logs), scattering run artifacts into an
// arbitrary sibling tree. That is the same untracked-tree-drift class the
// absolutization fixed for cwd, just relocated. The resolver must now REJECT a
// relative workspace whose cleaned join escapes the project root, loudly and
// with no MkdirAll side effect. (Absolute args keep their documented
// passthrough contract — the real loop hands absolute worktree/runs paths.)
func TestRunSubagentRun_WorkspaceOutsideProjectRootRejected(t *testing.T) {
	parent := t.TempDir()
	projectRoot := filepath.Join(parent, "proj")
	sibling := filepath.Join(parent, "sibling")
	for _, d := range []string{projectRoot, sibling} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("seed %s: %v", d, err)
		}
	}
	cwd := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	t.Setenv("EVOLVE_PROJECT_ROOT", projectRoot)

	promptFile := filepath.Join(projectRoot, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte("prompt\n"), 0o644); err != nil {
		t.Fatalf("write prompt fixture: %v", err)
	}
	t.Setenv("PROMPT_FILE_OVERRIDE", promptFile)

	const rel = "../sibling"
	var stdout, stderr bytes.Buffer
	rc := runSubagentRun([]string{"builder", "9", rel}, &stdout, &stderr)

	if rc == 0 {
		t.Errorf("a relative workspace escaping the project root must be rejected; rc=0 stdout=%q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "project root") {
		t.Errorf("rejection must name the containment violation; got stderr=%q", stderr.String())
	}
	// No MkdirAll side effect: the escaped sibling tree must gain no artifacts.
	entries, err := os.ReadDir(sibling)
	if err != nil {
		t.Fatalf("readdir sibling: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("a rejected out-of-root workspace must not create artifacts under it; found: %v", entries)
	}
}

// TestRunSubagentRun_ParentTraversalRelativeWorkspaceDoesNotPolluteCwd probes
// the traversal edge case: a relative workspace containing ".." components
// (escaping the project root's own subtree, e.g. into a sibling directory).
// The AC does not promise sandboxing against ".." (only "resolved against
// project root, not cwd"), so this test does not assert containment — it
// pins the one invariant the AC does guarantee regardless of how ".." is
// handled: the invoking cwd is never touched.
func TestRunSubagentRun_ParentTraversalRelativeWorkspaceDoesNotPolluteCwd(t *testing.T) {
	parent := t.TempDir()
	projectRoot := filepath.Join(parent, "proj")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("seed project root: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(parent, "sibling-workspace"), 0o755); err != nil {
		t.Fatalf("seed sibling workspace: %v", err)
	}
	cwd := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	t.Setenv("EVOLVE_PROJECT_ROOT", projectRoot)

	promptFile := filepath.Join(projectRoot, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte("prompt\n"), 0o644); err != nil {
		t.Fatalf("write prompt fixture: %v", err)
	}
	t.Setenv("PROMPT_FILE_OVERRIDE", promptFile)

	const rel = "../sibling-workspace"
	var stdout, stderr bytes.Buffer
	_ = runSubagentRun([]string{"builder", "9", rel}, &stdout, &stderr)

	entries, err := os.ReadDir(cwd)
	if err != nil {
		t.Fatalf("readdir cwd: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("a parent-traversal relative workspace arg must never resolve to (or pollute) "+
			"the invoking cwd, regardless of how the \"..\" is handled; found: %v", entries)
	}
}
