package changedpkgs

// changedpkgs_derivability_test.go — RED contract for cycle-582's
// changedpkgs-derivability-failloud task (scout-report.md Task 1).
//
// FromGit swallows every git error internally (`if out, err := …; err == nil {
// add(out) }`), so a caller cannot distinguish "0 files changed" (git-clean
// tree) from "git command failed" (underivable — e.g. concurrent-fleet
// `.git/index.lock`, a non-git worktree, or a bad baseRef). That conflation is
// exactly what makes the apicover CI-parity gate fail-open (cycle-581 audit
// D1/D2, warnship_apicover_ci_gap 3rd recurrence).
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this file fails to compile today; that compile failure IS the RED
// evidence):
//
//   - FromGitChecked(repoRoot, baseRef string) (pkgs []string, derivable bool)
//     propagates git diff/ls-files errors into derivable=false instead of
//     swallowing them. derivable=true whenever every git invocation the
//     function needs succeeded (even if the resulting package set is empty on
//     a clean tree).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Negative: TestFromGitChecked_NonGitRepo_ReturnsUnderivable (the
//     strongest anti-no-op signal — a naive derivable-always-true
//     implementation fails this)
//   - Edge:     TestFromGitChecked_EmptyArgs_ReturnsUnderivable
//   - Semantic: TestFromGitChecked_CleanRepo_ReturnsDerivableEmpty (derivable
//     but zero packages — must not be conflated with the negative case) and
//     TestFromGitChecked_TrackedGoChange_ReturnsDerivableWithPackages (a real
//     tracked change is both derivable AND non-empty)

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// runGit runs `git <args>` in dir with an isolated, host-independent config —
// mirrors the pattern in internal/dossier/rollback_test.go.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// initCleanGoRepo creates a real git repo with one committed go file under
// go/internal/foo/a.go and returns its root.
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

// TestFromGitChecked_NonGitRepo_ReturnsUnderivable: the single strongest
// regression guard — a plain temp dir (no .git) makes every git invocation
// fail; the caller MUST be told the set is underivable, not handed an empty
// (and therefore falsely "nothing changed") slice.
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

// TestFromGitChecked_EmptyArgs_ReturnsUnderivable: missing repoRoot/baseRef is
// a config error, not a verified-clean tree — must not report derivable=true.
func TestFromGitChecked_EmptyArgs_ReturnsUnderivable(t *testing.T) {
	if _, derivable := FromGitChecked("", "HEAD"); derivable {
		t.Errorf("FromGitChecked(\"\", \"HEAD\") derivable=true, want false")
	}
	if _, derivable := FromGitChecked("/some/repo", ""); derivable {
		t.Errorf("FromGitChecked(repo, \"\") derivable=true, want false")
	}
}

// TestFromGitChecked_CleanRepo_ReturnsDerivableEmpty: a real, clean git repo
// must report derivable=true with an empty package set — this is the case
// FromGit's swallow-everything behavior currently makes indistinguishable from
// the negative case above.
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

// TestFromGitChecked_TrackedGoChange_ReturnsDerivableWithPackages: a real
// tracked modification must surface both derivable=true and the changed
// package pattern — proves derivable isn't hardcoded false-when-nonempty
// either.
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
