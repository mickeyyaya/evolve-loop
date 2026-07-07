package audit

// ciparity_derivability_test.go — RED contract for cycle-582's
// changedpkgs-derivability-failloud task (scout-report.md Task 1; cycle-581
// audit D1/D2, unshipped).
//
// TODAY: changedPackagesForAudit returns nil identically whether the tree is
// git-clean (nothing changed) or the changed-set is genuinely underivable
// (git error, no repo, fleet index-lock race). Both apicoverEnforceChangedDefault
// and apicoverNewPackageGraduationDefault treat a nil changed-set as "nothing to
// enforce" and silently no-op (nil, nil) — a fail-open PASS on the very cycle
// that most needs the gate.
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this file fails to compile today; that compile failure IS the RED
// evidence):
//
//   - changedPackagesForAudit gains a second return value, derivable bool
//     (true via the handoff path, or via changedpkgs.FromGitChecked's own
//     derivable flag).
//   - apicoverEnforceChangedDefault and apicoverNewPackageGraduationDefault
//     each return a single actionable offender (FAIL, not WARN — err stays
//     nil) when the module dir exists, an .apicover-enforce list is present,
//     and the changed-set is underivable — instead of falling through the
//     empty-intersection/empty-ungraduated no-op path.
//   - A genuinely clean, git-derivable tree remains (nil, nil) — the fix must
//     not turn every cycle into a FAIL.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive/negative pair per gate: *_UnderivableChangedSet_FailsLoud
//     (must FAIL) vs *_CleanGitTree_StaysNoOp (must NOT FAIL) — the paired
//     test is the strongest guard against a naive "always FAIL" or "never
//     FAIL" implementation.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// runGitIn runs `git <args>` in dir with an isolated, host-independent
// config — mirrors internal/dossier/rollback_test.go / the sibling helper in
// internal/changedpkgs/changedpkgs_derivability_test.go.
func runGitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// enforceFixtureNonGit builds a go module worktree (go.mod + .apicover-enforce
// + a real tracked-looking go source file) that is deliberately NOT a git
// repo and has NO build handoff — every git invocation FromGitChecked makes
// will fail, so the changed-package set is underivable by construction.
func enforceFixtureNonGit(t *testing.T) (root string) {
	t.Helper()
	root, goDir := goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte("./internal/p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(goDir, "internal", "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "internal", "p", "x.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// enforceFixtureCleanGit builds the same module + enforce list, but as a real,
// clean, committed git repo with no build handoff — changedPackagesForAudit
// must fall back to a DERIVABLE (git succeeds) empty set.
func enforceFixtureCleanGit(t *testing.T) (root string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	root, goDir := goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte("./internal/p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(goDir, "internal", "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "internal", "p", "x.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, root, "init")
	runGitIn(t, root, "add", "-A")
	runGitIn(t, root, "-c", "user.email=seed@example.com", "-c", "user.name=seed", "commit", "-m", "baseline")
	return root
}

// TestApicoverEnforceChangedDefault_UnderivableChangedSet_FailsLoud: an
// underivable changed-set on a cycle with a real .apicover-enforce list must
// FAIL loud (non-empty offenders, err==nil) instead of the current silent
// (nil, nil) no-op — closing the fail-open class from cycle-581 D1/D2.
func TestApicoverEnforceChangedDefault_UnderivableChangedSet_FailsLoud(t *testing.T) {
	root := enforceFixtureNonGit(t)
	off, err := apicoverEnforceChangedDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil {
		t.Fatalf("apicoverEnforceChangedDefault(underivable changed-set) unexpected error: %v", err)
	}
	if len(off) == 0 {
		t.Fatalf("apicoverEnforceChangedDefault(underivable changed-set) = (nil,nil), want non-empty offenders (fail loud, not silent no-op)")
	}
}

// TestApicoverEnforceChangedDefault_CleanGitTree_StaysNoOp: the paired
// anti-false-positive regression — a genuinely clean, git-derivable tree must
// remain (nil, nil). Without this test, a naive "always fail when no handoff"
// implementation would pass the negative test above but FAIL every real
// clean cycle.
func TestApicoverEnforceChangedDefault_CleanGitTree_StaysNoOp(t *testing.T) {
	root := enforceFixtureCleanGit(t)
	off, err := apicoverEnforceChangedDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil || len(off) != 0 {
		t.Fatalf("apicoverEnforceChangedDefault(clean derivable tree) = (%v,%v), want (nil,nil)", off, err)
	}
}

// TestApicoverNewPackageGraduationDefault_UnderivableChangedSet_FailsLoud:
// the graduation gate shares changedPackagesForAudit's resolution and must
// get the same fail-loud treatment (scout design: "In both
// apicoverEnforceChangedDefault and apicoverNewPackageGraduationDefault").
func TestApicoverNewPackageGraduationDefault_UnderivableChangedSet_FailsLoud(t *testing.T) {
	root := enforceFixtureNonGit(t)
	off, err := apicoverNewPackageGraduationDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil {
		t.Fatalf("apicoverNewPackageGraduationDefault(underivable changed-set) unexpected error: %v", err)
	}
	if len(off) == 0 {
		t.Fatalf("apicoverNewPackageGraduationDefault(underivable changed-set) = (nil,nil), want non-empty offenders (fail loud, not silent no-op)")
	}
}

// TestApicoverNewPackageGraduationDefault_CleanGitTree_StaysNoOp: paired
// anti-false-positive for the graduation gate.
func TestApicoverNewPackageGraduationDefault_CleanGitTree_StaysNoOp(t *testing.T) {
	root := enforceFixtureCleanGit(t)
	off, err := apicoverNewPackageGraduationDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil || len(off) != 0 {
		t.Fatalf("apicoverNewPackageGraduationDefault(clean derivable tree) = (%v,%v), want (nil,nil)", off, err)
	}
}
