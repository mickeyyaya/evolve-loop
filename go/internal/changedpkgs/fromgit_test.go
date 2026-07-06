package changedpkgs

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// fromgit_test.go — RED contract for cycle-573 Task 2
// (builder-handoff-extinct-deterministic-changedpkgs, inbox weight 0.96
// critical; standing memory warnship_apicover_ci_gap, 3rd recurrence).
//
// Today changedPackagesForAudit derives the cycle's changed-package set from an
// LLM-emitted handoff-build.json that has been extinct since ~cycle 215, so the
// apicover CI-parity gate is silently fail-open (nil, nil) on every real cycle.
// Rule 5 (deterministic work must not depend on an LLM artifact) says the source
// must be pure git. This task adds changedpkgs.FromGit(repoRoot, baseRef) — the
// deterministic replacement: the set of go test patterns for .go files that
// differ between baseRef and the working tree (tracked edits + untracked new
// files), mapped through the existing FileToPackage.
//
// RED today: FromGit is undefined, so this whole package fails to COMPILE — the
// intended RED signal (a compile failure is a hard non-zero exit, never a silent
// pass). GREEN once Builder adds FromGit.

// gitCmd runs `git <args>` in dir with a hermetic identity + config isolation so
// the result never depends on the host's global git config.
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

// writeFile creates file rel (repo-relative), making parent dirs as needed.
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

// newRepoWithBaseline returns a temp git repo with one committed baseline file,
// so HEAD resolves and "changed vs HEAD" is well defined.
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

// TestFromGit_DetectsChangedGoPackage — AC-2a (behavioural, real git): a new
// uncommitted .go file in an untouched package must appear in FromGit's output
// as that package's test pattern, with NO handoff file anywhere in sight. This
// is the deterministic replacement for the extinct-handoff lookup.
func TestFromGit_DetectsChangedGoPackage(t *testing.T) {
	root := newRepoWithBaseline(t)
	writeFile(t, root, "go/internal/foo/foo.go", "package foo\n\nfunc New() {}\n")

	got := FromGit(root, "HEAD")
	if !contains(got, "./internal/foo/...") {
		t.Errorf("FromGit did not detect the new package: want ./internal/foo/... in %v", got)
	}
}

// TestFromGit_NoChangesEmpty — AC-2b (edge): with the working tree identical to
// baseRef, FromGit returns no packages. Guards against a fix that always claims
// "everything changed" (which would make the gate pass vacuously as before).
func TestFromGit_NoChangesEmpty(t *testing.T) {
	root := newRepoWithBaseline(t)
	if got := FromGit(root, "HEAD"); len(got) != 0 {
		t.Errorf("FromGit on a clean tree = %v, want empty", got)
	}
}

// TestFromGit_IgnoresNonGoChanges — AC-2c (edge/negative): a changed non-.go
// file (docs) is not a Go package and must not be reported. Reuses the existing
// FileToPackage filter through the git seam.
func TestFromGit_IgnoresNonGoChanges(t *testing.T) {
	root := newRepoWithBaseline(t)
	writeFile(t, root, "docs/notes.md", "# notes\n")
	if got := FromGit(root, "HEAD"); len(got) != 0 {
		t.Errorf("FromGit reported a package for a non-Go change: %v", got)
	}
}
