//go:build acs

// Package cycle1128 materialises the cycle-1128 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	core-suite-shared-tmp-p-hermeticity → widen
//	TestCoreTests_NeverPinSharedTmpProjectRoot from package-local reach
//	(os.ReadDir(".") sees only internal/core) to the whole go/internal tree.
//
// Why this cycle exists. PR #365 (97bcb4ec) killed the shared machine-global
// project-root class inside internal/core after it produced 19,521 accumulated
// run entries and the false suites_stay_green reds of cycles 1107/1116. The
// guard it added works — and currently sees exactly one package. A test added
// in ANY other package that re-pins the fixed shared root would ship silently,
// which defeats the guard's own stated purpose ("this guard keeps the class
// dead").
//
// Predicate strategy — every predicate EXERCISES the guard by running it
// (`go test -run TestCoreTests_NeverPinSharedTmpProjectRoot`) against a tree we
// mutate, and asserts on its exit code and reported offender. None of them
// greps the guard's source, so no magic string can satisfy them (the cycle-85
// degenerate-predicate ban). Concretely:
//
//   - 001 (positive / no-false-positive): the guard is GREEN on the untouched
//     worktree. A widened walk that trips on the current clean tree — or on the
//     guard's own concatenated self-reference — fails here.
//   - 002 (NEGATIVE, crux): plant the shared-root literal in a FLAT sibling
//     package (internal/redteamcheck) and require the guard to FAIL and to name
//     the planted file. Package-local reach cannot pass this; this is the
//     predicate that is RED today.
//   - 003 (NEGATIVE, depth): plant the same literal in a NESTED package
//     (internal/phases/build, two levels down) and require the same failure. A
//     naive one-level widening (os.ReadDir("..")) passes 002 but fails 003, so
//     the pair pins a real recursive walk.
//   - 004 (edge / over-broad-detector): plant the literal in a NON-test .go file
//     and require the guard to stay GREEN. The class is "tests pinning a shared
//     root"; a widened guard that greps every .go file would false-red on
//     production code that legitimately names a tmp path.
//
// Every planted probe is removed by t.Cleanup on every exit path, so the
// worktree is byte-identical before and after this suite runs.
package cycle1128

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// guardTest is the guard under test; goPkg is the package path it lives in.
const (
	guardTest = "TestCoreTests_NeverPinSharedTmpProjectRoot"
	guardPkg  = "./internal/core/"
)

// sharedTmpRootLiteral is the forbidden fixed project root, assembled by
// concatenation so this predicate file can never match itself (the cat-n
// self-trigger lesson). Value: the quoted shared machine-global tmp root.
var sharedTmpRootLiteral = `"/tmp/` + `p"`

// goDir returns the worktree's go/ module directory.
func goDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), "go")
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatalf("go module dir not found at %s: %v", dir, err)
	}
	return dir
}

// runGuard executes the hermeticity guard against the tree as it stands right
// now and returns its combined output plus exit code. -count=1 defeats the test
// cache, which is mandatory: the cache key does not know about the probe files
// we plant, so a cached PASS would silently fake every negative predicate.
func runGuard(t *testing.T, dir string) (string, int) {
	t.Helper()
	cmd := exec.Command("go", "test", "-count=1", "-run", guardTest, guardPkg)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("running guard in %s: %v\n%s", dir, err, out)
		}
		code = exitErr.ExitCode()
	}
	return string(out), code
}

// plantProbe writes content to pkgDir/name and registers its removal for every
// exit path (including t.Fatal), so the worktree is never left mutated.
func plantProbe(t *testing.T, pkgDir, name, content string) string {
	t.Helper()
	if _, err := os.Stat(pkgDir); err != nil {
		t.Fatalf("probe target package %s missing: %v", pkgDir, err)
	}
	path := filepath.Join(pkgDir, name)
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("probe path %s already exists — refusing to clobber", path)
	}
	t.Cleanup(func() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Errorf("cleanup: probe %s not removed: %v — worktree left dirty", path, err)
		}
	})
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("planting probe %s: %v", path, err)
	}
	return path
}

// probeSource builds a minimal, compilable probe file for pkgName that pins the
// forbidden shared root. The literal is interpolated at write time, so it lands
// CONTIGUOUS in the probe file (what the guard scans for) while staying split in
// this source file.
func probeSource(pkgName, constName string) string {
	return "package " + pkgName + "\n\n" +
		"// Transient probe planted by the cycle-1128 ACS predicate. Removed by\n" +
		"// t.Cleanup before the predicate returns; never commit this file.\n" +
		"const " + constName + " = " + sharedTmpRootLiteral + "\n"
}

// requireCleanBaseline asserts the guard is green BEFORE a probe is planted.
// Without it a negative predicate could "pass" on a tree that was already red
// for an unrelated reason.
func requireCleanBaseline(t *testing.T, dir string) {
	t.Helper()
	out, code := runGuard(t, dir)
	if code != 0 {
		t.Fatalf("baseline: guard %s is already RED before planting a probe (exit=%d); "+
			"the negative predicate below would be meaningless\n%s", guardTest, code, out)
	}
}

// TestC1128_001_GuardGreenOnCleanWorktree — no-false-positive criterion: the
// widened guard must still pass on the untouched tree (which contains zero
// occurrences of the literal under go/internal, verified at authoring time).
func TestC1128_001_GuardGreenOnCleanWorktree(t *testing.T) {
	dir := goDir(t)
	out, code := runGuard(t, dir)
	if code != 0 {
		t.Errorf("guard %s must be GREEN on the clean worktree, got exit=%d; "+
			"a widened walk that reds on an unviolated tree (e.g. by matching the guard's own "+
			"source, or a non-test file) is a false positive, not hardening\n%s", guardTest, code, out)
	}
}

// TestC1128_002_CatchesViolationInFlatSiblingPackage — the crux. RED until the
// guard's walk leaves its own package directory.
func TestC1128_002_CatchesViolationInFlatSiblingPackage(t *testing.T) {
	dir := goDir(t)
	requireCleanBaseline(t, dir)

	const probeName = "zz_acs1128_flat_probe_test.go"
	plantProbe(t, filepath.Join(dir, "internal", "redteamcheck"),
		probeName, probeSource("redteamcheck", "zzACS1128FlatProbe"))

	out, code := runGuard(t, dir)
	if code == 0 {
		t.Fatalf("guard %s stayed GREEN with the shared root pinned in internal/redteamcheck/%s — "+
			"its walk never leaves internal/core, so a re-pin in any other package ships undetected\n%s",
			guardTest, probeName, out)
	}
	if !strings.Contains(out, probeName) {
		t.Errorf("guard failed (exit=%d) but did not name the planted offender %s — the failure is "+
			"not attributable to the violation, so this predicate cannot distinguish a real catch "+
			"from an unrelated build break\n%s", code, probeName, out)
	}
}

// TestC1128_003_CatchesViolationInNestedPackage — depth criterion: a nested
// package two levels under go/internal must be reached too. Distinguishes a real
// recursive walk from a one-level os.ReadDir("..") widening.
func TestC1128_003_CatchesViolationInNestedPackage(t *testing.T) {
	dir := goDir(t)
	requireCleanBaseline(t, dir)

	const probeName = "zz_acs1128_nested_probe_test.go"
	plantProbe(t, filepath.Join(dir, "internal", "phases", "build"),
		probeName, probeSource("build", "zzACS1128NestedProbe"))

	out, code := runGuard(t, dir)
	if code == 0 {
		t.Fatalf("guard %s stayed GREEN with the shared root pinned in internal/phases/build/%s — "+
			"the walk is not recursive; a one-level listing of go/internal leaves every nested "+
			"package (all of internal/phases/*) unguarded\n%s", guardTest, probeName, out)
	}
	if !strings.Contains(out, probeName) {
		t.Errorf("guard failed (exit=%d) but did not name the planted offender %s — failure not "+
			"attributable to the nested violation\n%s", code, probeName, out)
	}
}

// TestC1128_004_IgnoresNonTestGoFiles — edge/OOD: the class is TESTS pinning a
// shared root. A widened detector that greps every .go file would red on
// production code that legitimately mentions a tmp path.
func TestC1128_004_IgnoresNonTestGoFiles(t *testing.T) {
	dir := goDir(t)
	requireCleanBaseline(t, dir)

	const probeName = "zz_acs1128_nontest_probe.go"
	plantProbe(t, filepath.Join(dir, "internal", "redteamcheck"),
		probeName, probeSource("redteamcheck", "zzACS1128NonTestProbe"))

	out, code := runGuard(t, dir)
	if code != 0 {
		t.Errorf("guard %s went RED on a NON-test file (internal/redteamcheck/%s) that merely names "+
			"the path (exit=%d) — the guard must keep its *_test.go filter; production code naming a "+
			"tmp path is not the cross-process-shared-state class\n%s", guardTest, probeName, code, out)
	}
}
