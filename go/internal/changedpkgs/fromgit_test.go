package changedpkgs

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{
		"-c", "user.email=test@example.com",
		"-c", "user.name=test",
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func newRepoWithBaseline(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitCmd(t, root, "init")
	writeFile(t, root, "go/internal/base/base.go", "package base\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-m", "baseline")
	return root
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func TestFromGit_DetectsChangedGoPackage(t *testing.T) {
	root := newRepoWithBaseline(t)
	writeFile(t, root, "go/internal/foo/foo.go", "package foo\n\nfunc New() {}\n")

	got := FromGit(root, "HEAD")
	if !contains(got, "./internal/foo/...") {
		t.Errorf("FromGit did not detect the new package: want ./internal/foo/... in %v", got)
	}
}

func TestFromGit_NoChangesEmpty(t *testing.T) {
	root := newRepoWithBaseline(t)
	if got := FromGit(root, "HEAD"); len(got) != 0 {
		t.Errorf("FromGit on a clean tree = %v, want empty", got)
	}
}

func TestFromGit_IgnoresNonGoChanges(t *testing.T) {
	root := newRepoWithBaseline(t)
	writeFile(t, root, "docs/notes.md", "# notes\n")
	if got := FromGit(root, "HEAD"); len(got) != 0 {
		t.Errorf("FromGit reported a package for a non-Go change: %v", got)
	}
}
