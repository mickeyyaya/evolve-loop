//go:build acs

// Package cycle539 materialises the cycle-539 acceptance criteria for the single
// triage-committed (`## top_n`) task: fix-dossier-tree-diff-guard-blocker.
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly ONE task to this cycle:
//	  fix-dossier-tree-diff-guard-blocker (H) — C539_001..004
//	Every `## deferred` item (the distinct real-source-leak bug, the
//	SELF_SHA_TAMPERED ship backlog, the review-gate/bridge-launch tail) gets
//	ZERO predicates here.
//
// FEATURE CONTEXT
//
//	internal/dossier/write.go's `commit bool` parameter is documented "reserved
//	for a future slice ... pass false for now", and internal/core/
//	dossier_producer.go:59 always calls Write(d, dir, false). Every cycle's
//	closeout dossier (knowledge-base/cycles/cycle-N.{json,md}) is therefore
//	written to the main tree but NEVER git-committed, leaving 40 untracked pairs
//	(cycle-474..537) that a later, unrelated phase's tree-diff guard flags as a
//	main-tree leak — aborting the whole cycle (538/524/520/...). This cycle
//	implements the reserved parameter: Write(d, dir, true) git-adds + git-commits
//	exactly the two new files, scoped, so future dossiers leave a clean tree.
//
// PREDICATE QUALITY (cycle-85): every load-bearing predicate EXERCISES the SUT —
// it CALLS the real dossier.Write against a seeded temp git repo and asserts on
// the actual git side effect (committed / tracked / untracked), never a "source
// file contains text X" grep.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : C539_001 Write(true) commits BOTH files (tracked, clean tree).
//   - Negative : C539_002 Write(false) commits NOTHING (backward-compat guard —
//     an implementation that ignores the flag and always commits FAILS here).
//   - Negative : C539_003 Write(true) is SCOPED — a pre-existing unrelated dirty
//     file stays UNTRACKED (a `git add -A`/`git add .` implementation FAILS here;
//     the anti-no-op that pins "scoped to just the two new files").
//   - Hygiene  : C539_004 the touched packages (dossier + core) build + vet clean.
package cycle539

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// sampleDossier returns a minimal VALID Dossier matching the producer's shape
// (cycle + goal + verdict + one phase — exactly the fields RenderJSON/Markdown
// validate). Cycle drives the filenames cycle-N.json / cycle-N.md.
func sampleDossier(cycle int) *dossier.Dossier {
	return &dossier.Dossier{
		Cycle:        cycle,
		Goal:         "commit dossier writes at source",
		FinalVerdict: dossier.VerdictPass,
		Phases:       []dossier.PhaseRecord{{Name: "build", Verdict: dossier.VerdictPass}},
	}
}

// mustGit runs `git -C dir <args...>`, failing the test on a launch error or a
// non-zero exit, and returns trimmed stdout.
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

// initGitRepo creates a fresh temp git repo with a commit identity (so `git
// commit` works deterministically regardless of the host's global config) and
// returns its path.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustGit(t, dir, "init", "-q")
	mustGit(t, dir, "config", "user.email", "acs@evolve.test")
	mustGit(t, dir, "config", "user.name", "ACS Cycle539")
	return dir
}

// TestC539_001_WriteCommitTrueCommitsBothFiles is the core POSITIVE: Write with
// commit=true must git-add AND git-commit both cycle-N.{json,md} into the repo,
// leaving a clean working tree. Exercises the real dossier.Write and asserts the
// on-disk git side effect (ls-files + porcelain + log), not a source grep.
func TestC539_001_WriteCommitTrueCommitsBothFiles(t *testing.T) {
	dir := initGitRepo(t)
	if err := dossier.Write(sampleDossier(42), dir, true); err != nil {
		t.Fatalf("Write(commit=true): %v", err)
	}

	names := []string{"cycle-42.json", "cycle-42.md"}

	// Both files must be TRACKED (committed) — listed by ls-files.
	tracked := mustGit(t, dir, "ls-files")
	for _, name := range names {
		if !strings.Contains(tracked, name) {
			t.Errorf("Write(commit=true) must git-add+commit %s; ls-files=%q", name, tracked)
		}
	}

	// The working tree must be CLEAN — neither file is left dirty/untracked.
	status := mustGit(t, dir, "status", "--porcelain")
	for _, name := range names {
		if strings.Contains(status, name) {
			t.Errorf("Write(commit=true) must leave a clean tree; %s still dirty:\n%s", name, status)
		}
	}

	// A real commit must exist.
	if log := mustGit(t, dir, "log", "--oneline"); log == "" {
		t.Error("Write(commit=true) must create a git commit; git log is empty")
	}
}

// TestC539_002_WriteCommitFalseCommitsNothing is the backward-compat NEGATIVE:
// commit=false must still write both files to disk but commit NOTHING. An
// implementation that ignores the flag and always commits FAILS here — pinning
// the commit parameter as load-bearing, not cosmetic.
func TestC539_002_WriteCommitFalseCommitsNothing(t *testing.T) {
	dir := initGitRepo(t)
	if err := dossier.Write(sampleDossier(43), dir, false); err != nil {
		t.Fatalf("Write(commit=false): %v", err)
	}

	// Files are written to disk...
	for _, name := range []string{"cycle-43.json", "cycle-43.md"} {
		if !acsassert.FileExists(t, filepath.Join(dir, name)) {
			t.Errorf("Write(commit=false) must still write %s to disk", name)
		}
	}

	// ...but nothing is staged/committed.
	if tracked := mustGit(t, dir, "ls-files"); tracked != "" {
		t.Errorf("Write(commit=false) must not git-add any file; ls-files=%q", tracked)
	}
	// `git log` fails (exit!=0) in a repo with no commits — read stdout directly
	// and assert it is empty rather than mustGit (which would Fatal on exit!=0).
	if log, _, _, _ := acsassert.SubprocessOutput("git", "-C", dir, "log", "--oneline"); strings.TrimSpace(log) != "" {
		t.Errorf("Write(commit=false) must not create a commit; git log=%q", log)
	}
}

// TestC539_003_WriteCommitTrueIsScopedNotAddAll is the anti-no-op NEGATIVE that
// pins "scoped to just the two new files": a pre-existing UNRELATED untracked
// file must NOT be swept into the dossier commit. A `git add -A` / `git add .`
// implementation commits unrelated.txt too and FAILS here.
func TestC539_003_WriteCommitTrueIsScopedNotAddAll(t *testing.T) {
	dir := initGitRepo(t)

	// A pre-existing unrelated untracked file sitting in the repo.
	if err := os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("not a dossier\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := dossier.Write(sampleDossier(44), dir, true); err != nil {
		t.Fatalf("Write(commit=true): %v", err)
	}

	tracked := mustGit(t, dir, "ls-files")
	// The two dossier files ARE committed...
	for _, name := range []string{"cycle-44.json", "cycle-44.md"} {
		if !strings.Contains(tracked, name) {
			t.Errorf("Write(commit=true) must commit %s; ls-files=%q", name, tracked)
		}
	}
	// ...but the unrelated file must NOT be swept in.
	if strings.Contains(tracked, "unrelated.txt") {
		t.Errorf("Write(commit=true) must scope the commit to the two dossier files; it also committed unrelated.txt (ls-files=%q)", tracked)
	}
	if status := mustGit(t, dir, "status", "--porcelain"); !strings.Contains(status, "unrelated.txt") {
		t.Errorf("unrelated.txt must remain untracked after a scoped dossier commit; status=%q", status)
	}
}

// TestC539_004_TouchedPackagesBuildAndVetClean is the no-regression guard: the
// two production packages that change this cycle — internal/dossier (the new
// commit path) and internal/core (the dossier_producer.go call-site flip to
// commit=true) — must build and vet clean. Building core proves the call-site
// change compiles. Real subprocesses, absolute package paths under the worktree.
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
