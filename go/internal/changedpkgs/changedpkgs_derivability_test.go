package changedpkgs

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func initCleanGoRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	dir := t.TempDir()
	runGit(t, dir, "init")
	if err := os.MkdirAll(filepath.Join(dir, "go", "internal", "foo"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "go", "internal", "foo", "a.go")
	if err := os.WriteFile(src, []byte("package foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "-c", "user.email=seed@example.com", "-c", "user.name=seed", "commit", "-m", "baseline")
	return dir
}

func TestFromGitChecked_NonGitRepo_ReturnsUnderivable(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "go", "internal", "foo"), 0o755); err != nil {
		t.Fatal(err)
	}
	pkgs, derivable := FromGitChecked(dir, "HEAD")
	if derivable {
		t.Fatalf("FromGitChecked(non-git dir) derivable=true, want false (git commands must fail here)")
	}
	if len(pkgs) != 0 {
		t.Errorf("FromGitChecked(non-git dir) pkgs=%v, want empty", pkgs)
	}
}

func TestFromGitChecked_EmptyArgs_ReturnsUnderivable(t *testing.T) {
	if _, derivable := FromGitChecked("", "HEAD"); derivable {
		t.Errorf("FromGitChecked(\"\", \"HEAD\") derivable=true, want false")
	}
	if _, derivable := FromGitChecked("/some/repo", ""); derivable {
		t.Errorf("FromGitChecked(repo, \"\") derivable=true, want false")
	}
}

func TestFromGitChecked_CleanRepo_ReturnsDerivableEmpty(t *testing.T) {
	dir := initCleanGoRepo(t)
	pkgs, derivable := FromGitChecked(dir, "HEAD")
	if !derivable {
		t.Fatalf("FromGitChecked(clean repo) derivable=false, want true (git succeeded, tree is genuinely clean)")
	}
	if len(pkgs) != 0 {
		t.Errorf("FromGitChecked(clean repo) pkgs=%v, want empty", pkgs)
	}
}

func TestFromGitChecked_TrackedGoChange_ReturnsDerivableWithPackages(t *testing.T) {
	dir := initCleanGoRepo(t)
	src := filepath.Join(dir, "go", "internal", "foo", "a.go")
	if err := os.WriteFile(src, []byte("package foo\n\nfunc Bar() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs, derivable := FromGitChecked(dir, "HEAD")
	if !derivable {
		t.Fatalf("FromGitChecked(tracked go change) derivable=false, want true")
	}
	want := []string{"./internal/foo/..."}
	if !reflect.DeepEqual(pkgs, want) {
		t.Errorf("FromGitChecked(tracked go change) pkgs=%v, want %v", pkgs, want)
	}
}
