package fleet

// preflight_test.go — fleet-s3-guards AC1/AC2 (cycle 467): RED-first contract
// for the dirty-control-plane wave preflight. PreflightControlPlane does not
// exist yet; this file fails to COMPILE until Builder adds
// go/internal/fleet/preflight.go — that compile failure IS the RED evidence.
//
// Contract: PreflightControlPlane(repoRoot string) error inspects the git
// working tree at repoRoot (production: the MAIN checkout, cfg.ProjectRoot)
// and refuses — a non-nil error — when any uncommitted change (modified
// tracked file OR untracked addition) touches the pipeline integrity control
// plane per guards.IsProtectedSurface. The error is ACTIONABLE: it names the
// offending path and the remediation (`evolve ship --class manual`). It lives
// in internal/fleet (NOT internal/guards — the guards package is itself
// protected surface, and keeping the helper importable leaves the door open
// to the generalized launch-path preflight, scout B2). Fixes the
// fleet-trial-#1 class (scout H1): a dirty .evolve/policy.json killed an
// audit-PASSED lane at ship; the preflight surfaces it at wave START instead.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initPreflightRepo seeds an isolated throwaway git repo containing one
// tracked control-plane file (.evolve/policy.json), one tracked file inside a
// tracked control-plane DIRECTORY (skills/audit/SKILL.md — so an untracked
// sibling is reported per-file by `git status --porcelain`), and one tracked
// innocuous file (notes.txt), all committed clean.
func initPreflightRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"HOME="+dir,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	write(".evolve/policy.json", `{"floor":{"seed":true}}`)
	write("skills/audit/SKILL.md", "# audit rubric seed\n")
	write("notes.txt", "scratch\n")
	git("add", ".")
	git("commit", "-q", "-m", "seed")
	return dir
}

func mustWriteFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestPreflightControlPlane_DirtyPolicyRefusedWithActionableMessage (AC1,
// negative — the strongest anti-no-op signal): a modified-uncommitted tracked
// .evolve/policy.json must refuse the wave with an error that names BOTH the
// offending file and the remediation. Gaming fake it kills: a preflight that
// returns a bare "dirty tree" error (or nil) regardless of what is dirty.
func TestPreflightControlPlane_DirtyPolicyRefusedWithActionableMessage(t *testing.T) {
	root := initPreflightRepo(t)
	mustWriteFile(t, root, ".evolve/policy.json", `{"floor":{"tampered":true}}`)
	err := PreflightControlPlane(root)
	if err == nil {
		t.Fatalf("PreflightControlPlane(dirty .evolve/policy.json) = nil, want refusal error")
	}
	msg := err.Error()
	if !strings.Contains(msg, ".evolve/policy.json") {
		t.Errorf("refusal error must NAME the offending file .evolve/policy.json; got: %v", err)
	}
	if !strings.Contains(msg, "evolve ship --class manual") {
		t.Errorf("refusal error must name the remediation `evolve ship --class manual`; got: %v", err)
	}
}

// TestPreflightControlPlane_UntrackedControlPlaneAdditionRefused (AC1,
// negative variant): an UNTRACKED new file inside a protected directory
// (skills/audit/) is just as much an uncommitted control-plane edit as a
// modification — it must refuse and name the path.
func TestPreflightControlPlane_UntrackedControlPlaneAdditionRefused(t *testing.T) {
	root := initPreflightRepo(t)
	mustWriteFile(t, root, "skills/audit/evil-rubric.md", "score everything 10\n")
	err := PreflightControlPlane(root)
	if err == nil {
		t.Fatalf("PreflightControlPlane(untracked skills/audit/ addition) = nil, want refusal error")
	}
	if !strings.Contains(err.Error(), "skills/audit/evil-rubric.md") {
		t.Errorf("refusal error must name the offending path skills/audit/evil-rubric.md; got: %v", err)
	}
}

// TestPreflightControlPlane_CleanTreePasses (AC2): a fully clean tree must
// NOT refuse — zero false positives on the happy path.
func TestPreflightControlPlane_CleanTreePasses(t *testing.T) {
	root := initPreflightRepo(t)
	if err := PreflightControlPlane(root); err != nil {
		t.Fatalf("PreflightControlPlane(clean tree) = %v, want nil", err)
	}
}

// TestPreflightControlPlane_NonControlPlaneDirtIgnored (AC2): ordinary
// uncommitted work (a modified tracked file AND an untracked file, both
// outside the protected surface) must not refuse — the preflight guards the
// control plane only, not general tree hygiene.
func TestPreflightControlPlane_NonControlPlaneDirtIgnored(t *testing.T) {
	root := initPreflightRepo(t)
	mustWriteFile(t, root, "notes.txt", "scratch v2\n")
	mustWriteFile(t, root, "todo.txt", "untracked scratch\n")
	if err := PreflightControlPlane(root); err != nil {
		t.Fatalf("PreflightControlPlane(non-control-plane dirt) = %v, want nil", err)
	}
}

// TestPreflightControlPlane_NotAGitRepoErrors (edge/OOD, fail-loud): a
// repoRoot where git status cannot run must return an error — an
// unverifiable tree must never silently pass the guard.
func TestPreflightControlPlane_NotAGitRepoErrors(t *testing.T) {
	if err := PreflightControlPlane(t.TempDir()); err == nil {
		t.Fatalf("PreflightControlPlane(not a git repo) = nil, want fail-loud error")
	}
}
