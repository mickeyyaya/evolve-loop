//go:build acs

// Package cycle1277 materialises the cycle-1277 acceptance criteria.
//
// Fleet scope pins this lane to one todo-id, `retro-fleet-stale-worktree-fallback`,
// and triage committed exactly one task from it (`triage-report.md` ## top_n):
//
//	wire-c1270-stale-worktree-regression → 001, 002, 003 (+ 004 as anti-goal)
//
// The subject is NOT the fix. Cycle-1255 defect D1 (CRITICAL — "retroWorktree
// gates the scratch-cwd fallback on req.Worktree != "", so a torn-down lane's
// stale worktree loses its retro") is landed at go/internal/phases/retro/retro.go
// and proven end-to-end by TestC1270_006/007. The subject is that the proof does
// not RUN: CI's durable ACS gate walks exactly one glob —
//
//	.github/workflows/ci.yml:57  →  go test -count=1 -tags acs ./acs/regression/...
//	go/Makefile:108 (test-acs-durable) → the same command
//
// — and `go/acs/cycle1270` sits outside it, an orphaned sibling under go/acs/
// alongside ~250 other never-promoted cycle packages. So the D1 guard is
// green-by-skip: it passes when someone runs it by hand and enforces nothing.
//
// That distinction is what these predicates are built to catch, and it is why
// none of them asserts on the CONTENT of the moved file. A predicate that
// grepped `go/acs/regression/cycle1270/predicates_test.go` for a test name would
// pass on a hand-copied stub that never executes; 001 and 003 instead RUN the
// CI-enforced command and require the real `--- PASS:` lines for the two named
// tests, so a stub, an empty file, or a `-run` pattern that matches nothing all
// stay RED. `go test -run` exits 0 while printing "no tests to run", so exit
// status alone is not evidence here and is never the sole assertion.
//
// 002 is the negative half: the failure mode of a "move" is a COPY. A duplicated
// package leaves the orphan in place (still unenforced, now divergent) while the
// promoted copy goes green, so the predicate demands exactly one declaration
// site for TestC1270_006 across the whole go/acs tree, on disk AND in the git
// index, plus all nine TestC1270_* functions at the new site — a cherry-picked
// two-test extract is a different artifact than the one cycle-1270 shipped.
//
// 004 is an anti-goal and is expected GREEN at RED time (recorded as
// pre-existing GREEN in test-report.md). This task wires coverage; it must not
// re-touch the fix. If the retro fallback contract regresses while the wiring
// lands, 004 says so.
//
// No new package under ./internal/... is created here (go/acs/regression/cycle1270
// is test-only, outside ./internal/...), so ADR-0069's repo-wide apicover
// enrollment does not apply — the same exemption acs/regression/noorphan and
// acs/regression/flagreaders document.
package cycle1277

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	// promotedPkg is the CI-reachable location the cycle-1270 predicates must end up in.
	promotedPkg = "go/acs/regression/cycle1270"
	// orphanPkg is the never-promoted location they must leave behind entirely.
	orphanPkg = "go/acs/cycle1270"
	// promotedImportPath is promotedPkg as a module-relative package pattern.
	promotedImportPath = "./acs/regression/cycle1270"

	d1MintTest     = "TestC1270_006_MintedScratchCwdClearsTheFleetGuard"
	d1DispatchTest = "TestC1270_007_RetroFleetDispatchCarriesLaneWorktreeEndToEnd"
)

// goDir returns <repo>/go, the directory every `go` invocation below runs in.
// Every subprocess sets cmd.Dir explicitly: this suite runs from the main tree,
// from a cycle worktree, and from each fleet lane, so process cwd is never a
// reliable anchor for module-relative package paths.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// runGo runs `go <args...>` in <repo>/go and returns combined output plus the
// exit code. Combined output is deliberate: `go vet` and build failures land on
// stderr, and a compile error in a sibling regression package must be visible in
// the failure message rather than swallowed.
func runGo(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = goDir(t)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out), code
}

// TestC1277_001_D1ProofExecutesUnderTheCIEnforcedGlob is the criterion:
// TestC1270_006/007 execute under the glob CI actually walks.
//
// It establishes that in three linked steps rather than by shelling the durable
// tier's whole-subtree sweep. Running `go test ./acs/regression/...` here would
// be the most literal restatement of the CI command, but it is also the shape
// the host's flaky-predicate lint bans on evidence — a recursive sweep inside a
// cycle predicate is contention-sensitive under fleet load and produced the
// false REDs of cycles 1173/1175/1178. Sidestepping that lint by splitting the
// pattern string would be worse than either option, so the coverage claim is
// decomposed into checks that each stand on their own:
//
//  1. the enforced command really is `-tags acs ./acs/regression/...`, read out
//     of CI's own config and the Makefile target rather than assumed;
//  2. the promoted package resolves as a real acs-tagged package at a path that
//     glob covers (`go list`, one named package — a directory of .go files that
//     do not build, or whose build tag was lost, does not resolve);
//  3. the two D1 tests actually PASS there.
//
// Step 3 asserts on the `--- PASS:` lines, not the exit code. `go test -run`
// with a pattern that matches nothing exits 0 while printing "testing: warning:
// no tests to run" — which is exactly today's state, and exactly what a
// hand-copied stub would leave behind. Exit code alone would score that green.
func TestC1277_001_D1ProofExecutesUnderTheCIEnforcedGlob(t *testing.T) {
	root := acsassert.RepoRoot(t)
	const enforcedGlob = "./acs/regression/..."

	// 1. The gate CI runs, from CI's own files. If either drifts to a different
	//    glob, "under acs/regression/" stops meaning "enforced" and this whole
	//    predicate's premise is stale — fail loudly rather than pass on it.
	for _, f := range []string{".github/workflows/ci.yml", "go/Makefile"} {
		p := filepath.Join(root, f)
		if !acsassert.FileContains(t, p, "-tags acs "+enforcedGlob) {
			t.Fatalf("%s no longer runs `-tags acs %s` — the durable ACS gate moved; "+
				"this predicate's notion of \"CI-enforced\" needs updating with it", f, enforcedGlob)
		}
	}

	// 2. The promoted package resolves under that glob's root.
	out, code := runGo(t, "list", "-tags", "acs", promotedImportPath)
	if code != 0 {
		t.Fatalf("%s does not resolve as a Go package (exit %d) — the cycle-1270 predicates are "+
			"not in the tree `%s` walks, so the cycle-1255 D1 stale-worktree-fallback proof is "+
			"still invisible to CI:\n%s", promotedImportPath, code, enforcedGlob, out)
	}
	if got := strings.TrimSpace(out); !strings.HasSuffix(got, "/acs/regression/cycle1270") {
		t.Fatalf("go list resolved %s to %q, want a package under acs/regression/", promotedImportPath, got)
	}

	// 3. The two D1 tests pass there.
	pattern := "^(" + d1MintTest + "|" + d1DispatchTest + ")$"
	out, code = runGo(t, "test", "-count=1", "-tags", "acs", "-run", pattern, "-v", promotedImportPath)
	if code != 0 {
		t.Fatalf("D1 proof failed under the enforced tree (exit %d):\n%s", code, out)
	}
	for _, name := range []string{d1MintTest, d1DispatchTest} {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("%s did not execute in %s — no `--- PASS: %s` line.\n"+
				"The cycle-1255 D1 stale-worktree-fallback proof is still not enforced "+
				"(.github/workflows/ci.yml:57, go/Makefile:108).\nOutput:\n%s",
				name, promotedPkg, name, out)
		}
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("`go test -run` matched nothing in %s — the promoted package does not contain "+
			"the D1 predicates.\nOutput:\n%s", promotedPkg, out)
	}
}

// TestC1277_002_OrphanLocationIsGoneNotDuplicated is the negative half: a move,
// not a copy.
//
// Three distinct ways the wiring can be faked, each checked:
//   - the orphan stays on disk (proof duplicated, orphan still unenforced),
//   - the orphan stays in the git index while gone from disk (CI checks out the
//     index, so a disk-only delete regrows the duplicate on a fresh clone),
//   - only the two D1 tests are extracted (the promoted package is then a
//     different artifact than the one cycle-1270 shipped and audited).
func TestC1277_002_OrphanLocationIsGoneNotDuplicated(t *testing.T) {
	root := acsassert.RepoRoot(t)

	if _, err := os.Stat(filepath.Join(root, orphanPkg)); err == nil {
		t.Errorf("%s still exists on disk: the cycle-1270 predicates were copied, not moved, "+
			"leaving an unenforced orphan that will diverge from the promoted copy", orphanPkg)
	}

	tracked, code := runGitLsFiles(t, root, orphanPkg)
	if code != 0 {
		t.Fatalf("git ls-files %s failed (exit %d)", orphanPkg, code)
	}
	if tracked != "" {
		t.Errorf("%s is still tracked in the git index (CI checks out the index, not this disk):\n%s",
			orphanPkg, tracked)
	}

	sites := declarationSites(t, filepath.Join(root, "go", "acs"), "func "+d1MintTest+"(")
	if len(sites) != 1 {
		t.Fatalf("expected exactly 1 declaration of %s under go/acs, found %d: %v",
			d1MintTest, len(sites), sites)
	}
	want := filepath.Join(root, promotedPkg, "predicates_test.go")
	if sites[0] != want {
		t.Errorf("%s is declared at %s, want %s", d1MintTest, sites[0], want)
	}

	body, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read %s: %v", want, err)
	}
	if got := strings.Count(string(body), "func TestC1270_"); got != 9 {
		t.Errorf("%s declares %d TestC1270_* functions, want 9 — the cycle-1270 package must be "+
			"promoted whole, not cherry-picked down to the two D1 tests", promotedPkg, got)
	}
	if pkg := "package cycle1270"; !strings.Contains(string(body), pkg) {
		t.Errorf("%s does not declare %q — the package clause must survive the move intact",
			promotedPkg, pkg)
	}
}

// runGitLsFiles lists index entries under pathspec. `git -C` is mandatory: bare
// `git` resolves the repo from process cwd, which differs between the main tree,
// a cycle worktree, and each fleet lane.
func runGitLsFiles(t *testing.T, root, pathspec string) (string, int) {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "ls-files", "--", pathspec)
	out, err := cmd.Output()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("git -C %s ls-files %s: %v", root, pathspec, err)
	}
	return strings.TrimSpace(string(out)), code
}

// declarationSites walks dir and returns every .go file containing needle.
func declarationSites(t *testing.T, dir, needle string) []string {
	t.Helper()
	var hits []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(body), needle) {
			hits = append(hits, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return hits
}

// TestC1277_003_PromotedPackageVetsAndPassesInPlace closes the gap 001 cannot:
// 001 narrows with -run, so it never exercises the other seven cycle-1270
// predicates that ride along with the move. This runs the promoted package
// whole — one named package, never a ./... sweep — and vets it, so a package
// clause, import path, or build-tag left inconsistent by the move is caught
// here instead of on CI.
func TestC1277_003_PromotedPackageVetsAndPassesInPlace(t *testing.T) {
	pkg := promotedImportPath

	if out, code := runGo(t, "vet", "-tags", "acs", pkg); code != 0 {
		t.Errorf("go vet -tags acs %s failed (exit %d):\n%s", pkg, code, out)
	}

	out, code := runGo(t, "test", "-count=1", "-tags", "acs", "-v", pkg)
	if code != 0 {
		t.Fatalf("go test -tags acs %s failed (exit %d):\n%s", pkg, code, out)
	}
	if strings.Contains(out, "[no test files]") || strings.Contains(out, "no tests to run") {
		t.Errorf("%s ran no tests — the promoted package is empty or its `acs` build tag was lost "+
			"in the move.\nOutput:\n%s", pkg, out)
	}
	for _, name := range []string{d1MintTest, d1DispatchTest} {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("%s did not pass in the promoted package.\nOutput:\n%s", name, out)
		}
	}
}

// TestC1277_004_RetroFallbackContractStillHolds is an ANTI-GOAL and is expected
// GREEN at RED time (see test-report.md "pre-existing GREEN").
//
// Scout's third criterion is that retro.go is not re-touched: the D1 fix is
// landed and correct, and this task wires its coverage rather than revisiting
// it. Stated as a behavioural guard rather than a diff check, because "did not
// change" is not the property that matters — "still holds" is. It drives the
// three named contract tests the D1 predicate itself joins: the fleet mint must
// satisfy the bridge guard's own predicate, the fallback must never resolve to
// the shared main tree or the dispatching process cwd, and a provisioned
// worktree must pass through verbatim.
func TestC1277_004_RetroFallbackContractStillHolds(t *testing.T) {
	names := []string{
		"TestRetroWorktree_FleetScratchCwdSatisfiesBridgeGuardPredicate",
		"TestRetro_EmptyWorktree_NeverMainTreeOrProcessCwd",
		"TestRetro_RealWorktree_PassedThroughUnchanged",
	}
	pattern := "^(" + strings.Join(names, "|") + ")$"
	out, code := runGo(t, "test", "-count=1", "-run", pattern, "-v", "./internal/phases/retro")
	if code != 0 {
		t.Fatalf("retro fallback contract tests failed (exit %d) — the cycle-1255 D1 fix regressed "+
			"while wiring its coverage:\n%s", code, out)
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("%s did not execute — the D1 fallback contract test was renamed or removed "+
				"(TestC1270_007 pins this exact name).\nOutput:\n%s", name, out)
		}
	}
}
