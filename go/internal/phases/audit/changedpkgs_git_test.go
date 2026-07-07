package audit

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// changedpkgs_git_test.go — RED contract for cycle-573 Task 2, the integration
// half. changedPackagesForAudit is the audit phase's changed-package locator; it
// gates apicover. Today it reads an extinct handoff-build.json and returns nil
// (fail-open) when absent, so the apicover gate never fires on a real cycle.
// After the fix it derives the set from git (changedpkgs.FromGit), so a cycle
// that changed a package is detected even with NO handoff file present.
//
// RED today: with no handoff file, changedPackagesForAudit returns nil, so this
// assertion (non-empty, includes the changed package) fails. GREEN once the
// locator is git-derived.

func gitInAudit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-c", "user.email=test@example.com", "-c", "user.name=test"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeAuditFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// TestChangedPackagesForAudit_GitDerivedNoHandoff — AC-2d (integration): a
// worktree that changed go/internal/foo/foo.go, with NO handoff-build.json in
// the cycle run dir, must still yield ./internal/foo/... The old handoff lookup
// returned nil here (silent fail-open); the git-derived locator must not.
func TestChangedPackagesForAudit_GitDerivedNoHandoff(t *testing.T) {
	root := t.TempDir()
	gitInAudit(t, root, "init")
	writeAuditFile(t, root, "go/internal/base/base.go", "package base\n")
	gitInAudit(t, root, "add", "-A")
	gitInAudit(t, root, "commit", "-m", "baseline")

	// The cycle's change: a new package, uncommitted, and deliberately NO
	// handoff-build.json / handoff-builder.json in .evolve/runs/cycle-573.
	writeAuditFile(t, root, "go/internal/foo/foo.go", "package foo\n\nfunc New() {}\n")

	got, _ := changedPackagesForAudit(root, 573)

	found := false
	for _, p := range got {
		if p == "./internal/foo/..." {
			found = true
		}
	}
	if !found {
		t.Errorf("changedPackagesForAudit fail-open on missing handoff: want ./internal/foo/... from git, got %v", got)
	}
}
