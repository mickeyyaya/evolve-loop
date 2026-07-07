package router

// digest_git_fallback_test.go — RED contract for cycle-589 Task
// changedpkgs-acssuite-router-git-fallback (scout-report.md Task 1, AC2;
// inbox builder-handoff-extinct-deterministic-changedpkgs weight 0.96, 3rd
// recurrence of warnship_apicover_ci_gap).
//
// TODAY: Digest's "build" branch (digest.go ~L43) reads ONLY
// handoff-build.json / handoff-builder.json via readFirstTracked. Both have
// been extinct since ~cycle 215, so on every real cycle sig.Build stays
// Present:false / FilesTouched:0 with an EMPTY DigestDegraded — a silent gap
// indistinguishable from "the build phase did nothing."
//
// FIX CONTRACT (undefined until Builder adds it, so this file's assertions
// fail today — that failure IS the RED evidence):
//
//   - When done["build"] is true but neither handoff file is present, Digest
//     falls back to a git-derived signal — reusing
//     changedpkgs.FromGitChecked(projectRoot, "HEAD") (same shared helper
//     internal/phases/audit.changedPackagesForAudit already uses; no
//     re-implemented git-diff logic), deriving projectRoot from workspace
//     via the standard <projectRoot>/.evolve/runs/cycle-<N> layout
//     (core.RunWorkspacePath's inverse).
//   - A git-derivable tree (even with a fallback, even if the handoff never
//     existed) yields sig.Build.Present == true with FilesTouched reflecting
//     the actually-changed files — never silently staying Present:false.
//   - A git-UNDERIVABLE tree (no git repo, git failure) must degrade LOUDLY:
//     sig.DigestDegraded gains an entry (mentioning "build") rather than the
//     current silent Present:false / empty-DigestDegraded combination — the
//     read-miss vs genuine-gap distinction (R5) the digest already applies to
//     handoff read errors must also cover an underivable git fallback.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// gitInRouter runs `git <args>` in dir with an isolated, host-independent
// config — mirrors internal/phases/audit/changedpkgs_git_test.go's gitInAudit.
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

// runWorkspacePath mirrors core.RunWorkspacePath (<root>/.evolve/runs/cycle-<N>)
// without importing internal/core (router is a leaf package by design — see
// signals.go's package doc — so it must not depend on core).
func runWorkspacePath(root string, cycle int) string {
	return filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle))
}

// TestDigest_Build_FallsBackToGitWhenHandoffAbsent — AC2 (positive case): a
// git-derivable worktree with NO handoff-build.json/handoff-builder.json must
// still populate sig.Build (Present:true, FilesTouched > 0) via the git
// fallback, instead of silently staying Present:false — the same silent gap
// that let cycle-587 ship internal/ciwatch without the router ever seeing it
// as a "files touched" signal.
func TestDigest_Build_FallsBackToGitWhenHandoffAbsent(t *testing.T) {
	root := t.TempDir()
	gitInRouter(t, root, "init")
	writeRouterFile(t, root, "go/internal/base/base.go", "package base\n")
	gitInRouter(t, root, "add", "-A")
	gitInRouter(t, root, "commit", "-m", "baseline")

	// The cycle's change: a new package, uncommitted, deliberately with NO
	// handoff-build.json / handoff-builder.json in the run workspace.
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

// TestDigest_Build_GitFailureDegradesLoudly — AC2 (negative case): a
// workspace whose project root is NOT a git repository (git itself fails)
// must degrade LOUDLY — DigestDegraded records the failure — rather than the
// current silent Present:false / empty-DigestDegraded combination a naive
// "swallow every git error" fallback would produce.
func TestDigest_Build_GitFailureDegradesLoudly(t *testing.T) {
	root := t.TempDir()
	// Deliberately no `git init` — root is a plain directory, so any git
	// invocation the fallback makes will fail.
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
