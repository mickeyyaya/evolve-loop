package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runChangelogCLI(t *testing.T, args ...string) (stdout, stderr string, rc int) {
	t.Helper()
	var o, e bytes.Buffer
	rc = runChangelogGen(args, nil, &o, &e)
	return o.String(), e.String(), rc
}

func TestRunChangelogGen_Help(t *testing.T) {
	out, _, rc := runChangelogCLI(t, "--help")
	if rc != 0 || !strings.Contains(out, "Usage") {
		t.Errorf("help broken: rc=%d", rc)
	}
}

func TestRunChangelogGen_MissingArgs(t *testing.T) {
	_, errOut, rc := runChangelogCLI(t)
	if rc != 10 || !strings.Contains(errOut, "usage:") {
		t.Errorf("rc=%d err=%q", rc, errOut)
	}
}

func TestRunChangelogGen_UnknownFlag(t *testing.T) {
	_, errOut, rc := runChangelogCLI(t, "--bogus")
	if rc != 10 || !strings.Contains(errOut, "unknown flag") {
		t.Errorf("rc=%d err=%q", rc, errOut)
	}
}

func TestRunChangelogGen_BadSemver(t *testing.T) {
	_, errOut, rc := runChangelogCLI(t, "from", "to", "v1.0.0")
	if rc != 1 || !strings.Contains(errOut, "not semver") {
		t.Errorf("rc=%d err=%q", rc, errOut)
	}
}

func TestRunChangelogGen_ExtraPositional(t *testing.T) {
	_, errOut, rc := runChangelogCLI(t, "a", "b", "c", "d")
	if rc != 10 || !strings.Contains(errOut, "extra") {
		t.Errorf("rc=%d err=%q", rc, errOut)
	}
}

func TestRunChangelogGen_IdempotentSkipInProcess(t *testing.T) {
	// Seed an existing CHANGELOG.md with the target version already present.
	tmp := t.TempDir()
	cl := filepath.Join(tmp, "CHANGELOG.md")
	if err := os.WriteFile(cl, []byte("# Changelog\n\n## [1.0.0]\n\n- existing\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", tmp)
	_, errOut, rc := runChangelogCLI(t, "from", "to", "1.0.0")
	if rc != 0 {
		t.Errorf("rc=%d", rc)
	}
	if !strings.Contains(errOut, "idempotent skip") {
		t.Errorf("expected idempotent skip: %q", errOut)
	}
}

func TestRunChangelogGen_DryRunAgainstRealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	tmp := t.TempDir()
	mustRun := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	mustRun("init", "-q", "-b", "main")
	_ = os.WriteFile(filepath.Join(tmp, "a"), []byte("a"), 0o644)
	mustRun("add", "a")
	mustRun("commit", "-m", "feat: first")
	mustRun("tag", "v0.1.0")
	_ = os.WriteFile(filepath.Join(tmp, "b"), []byte("b"), 0o644)
	mustRun("add", "b")
	mustRun("commit", "-m", "fix: second")

	t.Setenv("EVOLVE_PROJECT_ROOT", tmp)
	out, errOut, rc := runChangelogCLI(t, "v0.1.0", "HEAD", "0.2.0", "--dry-run")
	if rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errOut)
	}
	// v0.1.0..HEAD excludes commits AT v0.1.0 (the feat one), so only "fix: second" is in range.
	if !strings.Contains(out, "### Fixed") {
		t.Errorf("missing Fixed section in dry-run:\n%s", out)
	}
	if !strings.Contains(out, "- second") {
		t.Errorf("missing fix subject:\n%s", out)
	}
	if !strings.Contains(out, "BEGIN GENERATED") || !strings.Contains(out, "END GENERATED") {
		t.Errorf("missing dry-run markers:\n%s", out)
	}
	if !strings.Contains(errOut, "DRY-RUN") {
		t.Errorf("missing DRY-RUN log: %q", errOut)
	}
}

func TestRunChangelogGen_InvalidRefSurfaces(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	tmp := t.TempDir()
	exec.Command("git", "-C", tmp, "init", "-q", "-b", "main").Run()
	t.Setenv("EVOLVE_PROJECT_ROOT", tmp)
	_, errOut, rc := runChangelogCLI(t, "v99.99.99", "HEAD", "1.0.0")
	if rc != 1 || !strings.Contains(errOut, "FAIL") {
		t.Errorf("rc=%d err=%q", rc, errOut)
	}
}
