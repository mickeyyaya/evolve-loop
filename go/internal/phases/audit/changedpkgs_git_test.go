package audit

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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

func TestChangedPackagesForAudit_GitDerivedNoHandoff(t *testing.T) {
	root := t.TempDir()
	gitInAudit(t, root, "init")
	writeAuditFile(t, root, "go/internal/base/base.go", "package base\n")
	gitInAudit(t, root, "add", "-A")
	gitInAudit(t, root, "commit", "-m", "baseline")

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
