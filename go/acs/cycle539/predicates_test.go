//go:build acs

package cycle539

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func sampleDossier(cycle int) *dossier.Dossier {
	return &dossier.Dossier{
		Cycle:        cycle,
		Goal:         "commit dossier writes at source",
		FinalVerdict: dossier.VerdictPass,
		Phases:       []dossier.PhaseRecord{{Name: "build", Verdict: dossier.VerdictPass}},
	}
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	stdout, stderr, code, err := acsassert.SubprocessOutput("git", full...)
	if err != nil {
		t.Fatalf("launch git %v: %v", args, err)
	}
	if code != 0 {
		t.Fatalf("git %v exit=%d stderr:\n%s", args, code, stderr)
	}
	return strings.TrimSpace(stdout)
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustGit(t, dir, "init", "-q")
	mustGit(t, dir, "config", "user.email", "acs@evolve.test")
	mustGit(t, dir, "config", "user.name", "ACS Cycle539")
	return dir
}

func TestC539_001_WriteCommitTrueCommitsBothFiles(t *testing.T) {
	dir := initGitRepo(t)
	if err := dossier.Write(sampleDossier(42), dir, true); err != nil {
		t.Fatalf("Write(commit=true): %v", err)
	}

	names := []string{"cycle-42.json", "cycle-42.md"}

	tracked := mustGit(t, dir, "ls-files")
	for _, name := range names {
		if !strings.Contains(tracked, name) {
			t.Errorf("Write(commit=true) must git-add+commit %s; ls-files=%q", name, tracked)
		}
	}

	status := mustGit(t, dir, "status", "--porcelain")
	for _, name := range names {
		if strings.Contains(status, name) {
			t.Errorf("Write(commit=true) must leave a clean tree; %s still dirty:\n%s", name, status)
		}
	}

	if log := mustGit(t, dir, "log", "--oneline"); log == "" {
		t.Error("Write(commit=true) must create a git commit; git log is empty")
	}
}

func TestC539_002_WriteCommitFalseCommitsNothing(t *testing.T) {
	dir := initGitRepo(t)
	if err := dossier.Write(sampleDossier(43), dir, false); err != nil {
		t.Fatalf("Write(commit=false): %v", err)
	}

	for _, name := range []string{"cycle-43.json", "cycle-43.md"} {
		if !acsassert.FileExists(t, filepath.Join(dir, name)) {
			t.Errorf("Write(commit=false) must still write %s to disk", name)
		}
	}

	if tracked := mustGit(t, dir, "ls-files"); tracked != "" {
		t.Errorf("Write(commit=false) must not git-add any file; ls-files=%q", tracked)
	}
	if log, _, _, _ := acsassert.SubprocessOutput("git", "-C", dir, "log", "--oneline"); strings.TrimSpace(log) != "" {
		t.Errorf("Write(commit=false) must not create a commit; git log=%q", log)
	}
}

func TestC539_003_WriteCommitTrueIsScopedNotAddAll(t *testing.T) {
	dir := initGitRepo(t)

	if err := os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("not a dossier\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := dossier.Write(sampleDossier(44), dir, true); err != nil {
		t.Fatalf("Write(commit=true): %v", err)
	}

	tracked := mustGit(t, dir, "ls-files")
	for _, name := range []string{"cycle-44.json", "cycle-44.md"} {
		if !strings.Contains(tracked, name) {
			t.Errorf("Write(commit=true) must commit %s; ls-files=%q", name, tracked)
		}
	}
	if strings.Contains(tracked, "unrelated.txt") {
		t.Errorf("Write(commit=true) must scope the commit to the two dossier files; it also committed unrelated.txt (ls-files=%q)", tracked)
	}
	if status := mustGit(t, dir, "status", "--porcelain"); !strings.Contains(status, "unrelated.txt") {
		t.Errorf("unrelated.txt must remain untracked after a scoped dossier commit; status=%q", status)
	}
}

func TestC539_004_TouchedPackagesBuildAndVetClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	pkgs := []string{
		filepath.Join(root, "go", "internal", "dossier"),
		filepath.Join(root, "go", "internal", "core"),
	}
	for _, pkg := range pkgs {
		if _, stderr, code, err := acsassert.SubprocessOutput("go", "build", pkg); err != nil {
			t.Fatalf("launch go build %s: %v", pkg, err)
		} else if code != 0 {
			t.Errorf("go build %s must be clean; exit=%d stderr:\n%s", pkg, code, stderr)
		}
		if _, stderr, code, err := acsassert.SubprocessOutput("go", "vet", pkg); err != nil {
			t.Fatalf("launch go vet %s: %v", pkg, err)
		} else if code != 0 {
			t.Errorf("go vet %s must be clean; exit=%d stderr:\n%s", pkg, code, stderr)
		}
	}
}
