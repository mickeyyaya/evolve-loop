package opscmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func planePrimaryRoot(t *testing.T, withState bool) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if withState {
		if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".evolve", "state.json"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRunDoctorPlane_PrimaryWithLoopStateWarnsExit2(t *testing.T) {
	root := planePrimaryRoot(t, true)
	var out, errb bytes.Buffer
	rc := runDoctorPlane([]string{root}, &out, &errb)
	if rc != 2 {
		t.Fatalf("rc = %d, want 2 (WARN): %s%s", rc, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "ADR-0080") || !strings.Contains(out.String(), "PRIMARY") {
		t.Errorf("WARN goes to stderr with the ADR; the report names the plane on stdout: out=%s err=%s", out.String(), errb.String())
	}
}

func TestRunDoctorPlane_PrimaryWithoutStateIsClean(t *testing.T) {
	root := planePrimaryRoot(t, false)
	var out, errb bytes.Buffer
	if rc := runDoctorPlane([]string{root}, &out, &errb); rc != 0 {
		t.Fatalf("rc = %d, want 0: %s%s", rc, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "PRIMARY checkout") {
		t.Errorf("report must state the plane: %s", out.String())
	}
}

func TestRunDoctorPlane_NotARepoErrors(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := runDoctorPlane([]string{t.TempDir()}, &out, &errb); rc != 1 {
		t.Fatalf("rc = %d, want 1 on a non-repo", rc)
	}
}

func newDoctorPlaneHubFixture(t *testing.T) (seed *gittest.Repo, hubRoot string) {
	t.Helper()
	seed = gittest.Fixture(t)
	seed.Write("README.md", "seed\n")
	seed.Git("add", "README.md")
	seed.Git("commit", "-q", "-m", "seed")
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	hubRoot = filepath.Join(base, "hub")
	store := filepath.Join(hubRoot, ".repo.git")
	seed.CloneBare(seed.Dir, store)
	storeGit := "--git-dir=" + store
	seed.Git(storeGit, "config", "gc.auto", "0")
	seed.Git(storeGit, "config", "maintenance.auto", "false")
	seed.Git(storeGit, "worktree", "add", "-q", "--detach", filepath.Join(hubRoot, "console"), "main")
	seed.Git(storeGit, "worktree", "add", "-q", filepath.Join(hubRoot, "runtime"), "main")
	seed.Git(storeGit, "worktree", "add", "-q", "-b", "task", filepath.Join(hubRoot, "dev", "task"), "main")
	return seed, hubRoot
}

func TestRunDoctorPlane_InspectAndRepairMovedHub(t *testing.T) {
	_, hubRoot := newDoctorPlaneHubFixture(t)
	runtimeDir := filepath.Join(hubRoot, "runtime")

	var out, errb bytes.Buffer
	if rc := runDoctorPlane([]string{runtimeDir}, &out, &errb); rc != exitOK {
		t.Fatalf("healthy hub inspect rc = %d, want 0: out=%s err=%s", rc, out.String(), errb.String())
	}
	out.Reset()
	errb.Reset()
	if rc := runDoctorPlane([]string{"--repair", runtimeDir}, &out, &errb); rc != exitOK || strings.Contains(out.String(), "repaired:") {
		t.Fatalf("healthy hub repair rc = %d: out=%s err=%s", rc, out.String(), errb.String())
	}

	movedBase, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	movedHub := filepath.Join(movedBase, "moved-hub")
	if err := os.Rename(hubRoot, movedHub); err != nil {
		t.Fatal(err)
	}
	movedRuntime := filepath.Join(movedHub, "runtime")

	out.Reset()
	errb.Reset()
	if rc := runDoctorPlane([]string{movedRuntime}, &out, &errb); rc != exitFail {
		t.Fatalf("moved hub inspect rc = %d, want 1: out=%s err=%s", rc, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "broken: "+movedRuntime) {
		t.Fatalf("expected broken line for %s in %s", movedRuntime, out.String())
	}

	out.Reset()
	errb.Reset()
	if rc := runDoctorPlane([]string{"--repair", movedRuntime}, &out, &errb); rc != exitOK {
		t.Fatalf("moved hub repair rc = %d, want 0: out=%s err=%s", rc, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "repaired: "+movedRuntime) {
		t.Fatalf("expected repaired line for %s in %s", movedRuntime, out.String())
	}
}

func TestRunDoctorPlane_StoreSideBreakAndEdgeCases(t *testing.T) {
	seed, hubRoot := newDoctorPlaneHubFixture(t)
	store := filepath.Join(hubRoot, ".repo.git")
	seed.Git("--git-dir="+store, "worktree", "add", "-q", "-b", "other-task", filepath.Join(hubRoot, "dev", "aa-other"), "main")
	seed.Git("--git-dir="+store, "worktree", "add", "-q", "-b", "lane", filepath.Join(hubRoot, "runtime", ".evolve", "worktrees", "cycle-lane"), "main")

	primaryInHub := filepath.Join(hubRoot, "dev", "aa-nested-primary", ".git")
	if err := os.MkdirAll(primaryInHub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(primaryInHub, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	if rc := runDoctorPlane([]string{filepath.Join(hubRoot, "runtime")}, &out, &errb); rc != exitOK {
		t.Fatalf("healthy hub with foreign primary checkout rc = %d, want 0: out=%s", rc, out.String())
	}

	if err := os.Rename(filepath.Join(hubRoot, "dev", "task"), filepath.Join(hubRoot, "dev", "zz-task-renamed")); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	if rc := runDoctorPlane([]string{filepath.Join(hubRoot, "runtime")}, &out, &errb); rc != exitFail {
		t.Fatalf("renamed dev worktree rc = %d, want 1: out=%s", rc, out.String())
	}
	if !strings.Contains(out.String(), "broken: "+filepath.Join(hubRoot, "dev", "zz-task-renamed")) {
		t.Fatalf("expected broken renamed worktree in %s", out.String())
	}

	if err := os.RemoveAll(filepath.Join(hubRoot, "dev", "zz-task-renamed")); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(store, "worktrees", "aa-other", "gitdir"))
	out.Reset()
	errb.Reset()
	if rc := runDoctorPlane([]string{filepath.Join(hubRoot, "runtime")}, &out, &errb); rc != exitOK {
		t.Fatalf("pruned/missing worktree dir rc = %d, want 0: out=%s", rc, out.String())
	}

	if sameCleanPath("/nonexistent/path/a/../b") != "/nonexistent/path/b" {
		t.Fatalf("sameCleanPath fallback failed")
	}
}

func TestRunDoctorPlane_FlagsAndGitFailure(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := runDoctorPlane([]string{"--unknown"}, &out, &errb); rc != exitUsage {
		t.Fatalf("unknown flag rc = %d, want %d", rc, exitUsage)
	}
	out.Reset()
	errb.Reset()
	if rc := runDoctorPlane([]string{"a", "b"}, &out, &errb); rc != exitUsage {
		t.Fatalf("extra arg rc = %d, want %d", rc, exitUsage)
	}

	nonHubPrimary := planePrimaryRoot(t, false)
	out.Reset()
	errb.Reset()
	if rc := runDoctorPlane([]string{"--repair", nonHubPrimary}, &out, &errb); rc != exitOK {
		t.Fatalf("non-hub --repair rc = %d, want 0", rc)
	}

	_, hubRoot := newDoctorPlaneHubFixture(t)
	movedBase, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	movedHub := filepath.Join(movedBase, "moved-hub-fail")
	if err := os.Rename(hubRoot, movedHub); err != nil {
		t.Fatal(err)
	}
	gittest.FaultOn(t, "worktree")
	out.Reset()
	errb.Reset()
	if rc := runDoctorPlane([]string{"--repair", filepath.Join(movedHub, "runtime")}, &out, &errb); rc != exitRuntime {
		t.Fatalf("faulted git worktree rc = %d, want %d: err=%s", rc, exitRuntime, errb.String())
	}
}
