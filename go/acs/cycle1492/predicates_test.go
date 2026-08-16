//go:build acs

// Package cycle1492 materialises the acceptance criteria for the single
// fleet-scoped item pinned to this lane: `verdict-cache-fresh-base-collision`
// (triage top_n: verdict-cache-fresh-base-guard, verdict-cache-post-build-regression).
//
// State of the tree at RED (measured, not assumed):
//
//   - The fresh-base guard itself is already implemented and behaviourally
//     GREEN. `verdictcache.ProbeEligible` is the single-sourced predicate; the
//     pre-loop ADR-0048 shadow probe (orchestrator.go) and the audit-binding Put
//     (phase_bindings.go) both route through it, and the integration oracles
//     TestVerdictCacheCollisionRegression / TestVerdictCacheProbeEligibilityWiring
//     pass. Predicates 001 and 002 pin that behaviour and are PRE-EXISTING GREEN
//     — they are frozen anti-regression contracts, not work items. Their value is
//     adversarial: 002 is the anti-gaming negative that fails if the Builder
//     "fixes" the collision by disabling verdict-cache reuse wholesale.
//
//   - What is RED is the lane's ability to SHIP the guard. Cycle 1488 produced
//     this exact code and still FAILed with `EGPS: red_count=1`: its own
//     predicate TestC1488_003 asserts the retired inline comparison is gone by
//     calling acsassert.FileContains inside a negation. FileContains reports the
//     miss through tb.Errorf, so the predicate fails precisely BECAUSE the fix
//     landed. Predicate 003 is that RED, and its fix is the one-symbol swap to
//     acsassert.FileNotContains — the API that exists for negative assertions.
//
// Predicate strategy (cycle-85 degenerate-predicate ban): every predicate below
// either runs the system under test as a subprocess and asserts on its exit
// code, or calls the production function over real git-derived tree identities.
// No load-bearing source grep appears anywhere in this file, and no
// acsassert.FileContains call is used in a negated position.
package cycle1492

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// runFromGoRoot executes cmd with its working directory pinned to the module
// root and returns combined output plus exit code. Every caller builds its own
// exec.Command literal so the NARROW scoping — one named package, always with a
// -run filter — stays visible at the call site; whole-repo sweeps are the
// regression suite's job, never a cycle predicate's.
func runFromGoRoot(t *testing.T, cmd *exec.Cmd) (string, int) {
	t.Helper()
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%v: %v", cmd.Args, err)
	}
	return string(out), code
}

// git runs a git command against dir. `-C dir` is mandatory: a bare `git`
// resolves its repository from the process working directory, which differs
// between the main tree, a cycle worktree, and each fleet lane.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestC1492_001_FreshBaseTreeIsNeverALookupKey materialises AC-1 of
// verdict-cache-fresh-base-guard: a pre-build worktree whose content tree equals
// the cycle's base tree cannot perform a verdict-cache lookup, even when that key
// carries a cached PASS.
//
// Two halves, both behavioural:
//
//	(a) The identity half is derived from REAL git plumbing rather than a
//	    hand-written string table — `git write-tree` on an untouched clone is
//	    provably the base commit's tree, which is exactly why every sibling lane
//	    cut from one main tip collides. The production predicate is then CALLED
//	    with those two SHAs.
//	(b) The production half runs the orchestrator's real RunCycle path
//	    (TestVerdictCacheCollisionRegression/clean_cached_base_is_suppressed
//	    seeds a cached PASS under the fresh key and asserts the probe reports
//	    skipped=true, matched=false), which is the caller proof: the guard must
//	    be reached from the pre-loop probe, not merely exist.
func TestC1492_001_FreshBaseTreeIsNeverALookupKey(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte("base content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "seed.txt")
	git(t, repo, "commit", "-q", "-m", "base")

	baseTree := git(t, repo, "rev-parse", "HEAD^{tree}")
	freshTree := git(t, repo, "write-tree")
	if freshTree != baseTree {
		t.Fatalf("fixture invalid: fresh worktree tree %q != base tree %q", freshTree, baseTree)
	}
	if verdictcache.ProbeEligible(baseTree, freshTree) {
		t.Errorf("RED: ProbeEligible(base=%s, fresh=%s) = true — an untouched worktree is a "+
			"lookup key, so every sibling lane from this tip reuses the same stale verdict",
			baseTree, freshTree)
	}

	out, code := runFromGoRoot(t, exec.Command("go", "test", "-tags", "integration", "-count=1",
		"-run", "TestVerdictCacheCollisionRegression/clean_cached_base_is_suppressed", "./internal/core"))
	if code != 0 {
		t.Errorf("RED: fresh-base suppression oracle failed on the real RunCycle path (rc=%d):\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("RED: the fresh-base suppression subtest did not run — the caller proof is vacuous:\n%s", out)
	}
}

// TestC1492_002_ChangedWorktreeStaysEligible materialises AC-1 of
// verdict-cache-post-build-regression, and is this cycle's anti-gaming negative.
// The cheapest way to make 001 pass is to disable verdict-cache lookups
// outright; that would also destroy the genuine re-land reuse ADR-0048 exists
// for. A changed worktree must therefore remain DISTINGUISHABLE from its base
// and must still reach the advisory lookup (skipped=false, matched=true on a
// seeded hit), and the orchestrator's decision must still be derived from the
// shared predicate rather than a re-introduced local copy.
func TestC1492_002_ChangedWorktreeStaysEligible(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte("base content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "seed.txt")
	git(t, repo, "commit", "-q", "-m", "base")
	baseTree := git(t, repo, "rev-parse", "HEAD^{tree}")

	if err := os.WriteFile(filepath.Join(repo, "built.txt"), []byte("builder output\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A")
	changedTree := git(t, repo, "write-tree")
	if changedTree == baseTree {
		t.Fatalf("fixture invalid: changed worktree tree still equals base tree %q", baseTree)
	}
	if !verdictcache.ProbeEligible(baseTree, changedTree) {
		t.Errorf("RED: ProbeEligible(base=%s, changed=%s) = false — real post-build content is "+
			"no longer cacheable, the guard was widened into a blanket disable",
			baseTree, changedTree)
	}

	out, code := runFromGoRoot(t, exec.Command("go", "test", "-tags", "integration", "-count=1",
		"-run", "TestVerdictCacheCollisionRegression/dirty_cache_hit_remains_observable", "./internal/core"))
	if code != 0 {
		t.Errorf("RED: changed-worktree advisory lookup no longer observable on the real RunCycle path (rc=%d):\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("RED: the changed-worktree subtest did not run — the anti-gaming proof is vacuous:\n%s", out)
	}

	wire, wcode := runFromGoRoot(t, exec.Command("go", "test", "-tags", "integration", "-count=1",
		"-run", "TestVerdictCacheProbeEligibilityWiring", "./internal/core"))
	if wcode != 0 {
		t.Errorf("RED: orchestrator probe decision diverged from verdictcache.ProbeEligible (rc=%d):\n%s", wcode, wire)
	}
}

// TestC1492_003_LaneACSSuiteIsGreen is the RED that actually blocks this lane.
//
// The guard shipped in cycle 1488's worktree, yet the cycle FAILed on
// `EGPS: red_count=1` — the lane's own carried predicate suite is red, so the
// fix cannot land, and cycles 1488/1492 keep re-deriving the same code. The red
// predicate is TestC1488_003, which asserts the retired inline comparison
// `worktreeTree == headTree` is absent by calling acsassert.FileContains inside
// an `if` negation. FileContains signals a miss through tb.Errorf, so the
// predicate reports FAIL exactly when the criterion HOLDS — an inverted oracle.
// acsassert.FileNotContains is the assertion for that shape.
//
// This predicate runs the carried suite and requires it GREEN. It is behavioural
// (subprocess exit code of the real suite) and it is the ship precondition for
// both top_n tasks: neither can reach main while red_count > 0.
func TestC1492_003_LaneACSSuiteIsGreen(t *testing.T) {
	out, code := runFromGoRoot(t, exec.Command("go", "test", "-tags", "acs", "-count=1", "./acs/cycle1488"))
	if code != 0 {
		t.Errorf("RED: the lane's carried ACS suite (acs/cycle1488) is red, so EGPS red_count > 0 "+
			"and the fresh-base guard cannot ship (rc=%d):\n%s", code, out)
	}
	if strings.Contains(out, "[no test files]") {
		t.Errorf("RED: acs/cycle1488 built no tests — the suite check is vacuous:\n%s", out)
	}
}
