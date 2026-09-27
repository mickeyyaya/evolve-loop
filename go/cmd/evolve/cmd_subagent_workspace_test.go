package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	entries, err := os.ReadDir(sibling)
	if err != nil {
		t.Fatalf("readdir sibling: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("a rejected out-of-root workspace must not create artifacts under it; found: %v", entries)
	}
}

// This does not assert containment (a relative arg escaping the project root
// is a separate invariant, see TestRunSubagentRun_WorkspaceOutsideProjectRootRejected);
// it pins only that the invoking cwd is never touched, regardless of how ".."
// is handled.
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
