package audit

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func runGitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func enforceFixtureNonGit(t *testing.T) (root string) {
	t.Helper()
	root, goDir := goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte("./internal/p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(goDir, "internal", "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "internal", "p", "x.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func enforceFixtureCleanGit(t *testing.T) (root string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	root, goDir := goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte("./internal/p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(goDir, "internal", "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "internal", "p", "x.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, root, "init")
	runGitIn(t, root, "add", "-A")
	runGitIn(t, root, "-c", "user.email=seed@example.com", "-c", "user.name=seed", "commit", "-m", "baseline")
	return root
}

func TestApicoverEnforceChangedDefault_UnderivableChangedSet_FailsLoud(t *testing.T) {
	root := enforceFixtureNonGit(t)
	off, err := apicoverEnforceChangedDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil {
		t.Fatalf("apicoverEnforceChangedDefault(underivable changed-set) unexpected error: %v", err)
	}
	if len(off) == 0 {
		t.Fatalf("apicoverEnforceChangedDefault(underivable changed-set) = (nil,nil), want non-empty offenders (fail loud, not silent no-op)")
	}
}

func TestApicoverEnforceChangedDefault_CleanGitTree_StaysNoOp(t *testing.T) {
	root := enforceFixtureCleanGit(t)
	off, err := apicoverEnforceChangedDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil || len(off) != 0 {
		t.Fatalf("apicoverEnforceChangedDefault(clean derivable tree) = (%v,%v), want (nil,nil)", off, err)
	}
}

func TestApicoverNewPackageGraduationDefault_UnderivableChangedSet_FailsLoud(t *testing.T) {
	root := enforceFixtureNonGit(t)
	off, err := apicoverNewPackageGraduationDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil {
		t.Fatalf("apicoverNewPackageGraduationDefault(underivable changed-set) unexpected error: %v", err)
	}
	if len(off) == 0 {
		t.Fatalf("apicoverNewPackageGraduationDefault(underivable changed-set) = (nil,nil), want non-empty offenders (fail loud, not silent no-op)")
	}
}

func TestApicoverNewPackageGraduationDefault_CleanGitTree_StaysNoOp(t *testing.T) {
	root := enforceFixtureCleanGit(t)
	off, err := apicoverNewPackageGraduationDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil || len(off) != 0 {
		t.Fatalf("apicoverNewPackageGraduationDefault(clean derivable tree) = (%v,%v), want (nil,nil)", off, err)
	}
}
