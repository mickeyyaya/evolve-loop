package router

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func gitInRouter(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-c", "user.email=test@example.com", "-c", "user.name=test"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeRouterFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// runWorkspacePath mirrors core.RunWorkspacePath, which this leaf package cannot import.
func runWorkspacePath(root string, cycle int) string {
	return filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle))
}

func TestDigest_Build_FallsBackToGitWhenHandoffAbsent(t *testing.T) {
	root := t.TempDir()
	gitInRouter(t, root, "init")
	writeRouterFile(t, root, "go/internal/base/base.go", "package base\n")
	gitInRouter(t, root, "add", "-A")
	gitInRouter(t, root, "commit", "-m", "baseline")

	writeRouterFile(t, root, "go/internal/newpkg/newpkg.go", "package newpkg\n\nfunc New() {}\n")

	ws := runWorkspacePath(root, 589)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	sig, err := Digest(ws, []string{"build"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	if !sig.Build.Present {
		t.Fatalf("Digest(no handoff, git-derivable tree).Build.Present = false, want true (git fallback)")
	}
	if sig.Build.FilesTouched == 0 {
		t.Errorf("Digest(no handoff, git-derivable tree).Build.FilesTouched = 0, want > 0 (git shows internal/newpkg/newpkg.go touched)")
	}
}

func TestDigest_Build_GitFailureDegradesLoudly(t *testing.T) {
	root := t.TempDir()
	// No git init: every git call the fallback makes fails.
	ws := runWorkspacePath(root, 589)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	sig, err := Digest(ws, []string{"build"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	if sig.Build.Present {
		t.Fatalf("Digest(no handoff, non-git tree).Build.Present = true, want false (git-underivable)")
	}
	found := false
	for _, d := range sig.DigestDegraded {
		if strings.Contains(strings.ToLower(d), "build") {
			found = true
		}
	}
	if !found {
		t.Errorf("Digest(no handoff, non-git tree).DigestDegraded = %v, want an entry mentioning \"build\" (loud degrade, not silent no-op)", sig.DigestDegraded)
	}
}
