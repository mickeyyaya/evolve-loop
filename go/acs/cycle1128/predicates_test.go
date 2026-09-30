//go:build acs

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

const (
	guardTest = "TestCoreTests_NeverPinSharedTmpProjectRoot"
	guardPkg  = "./internal/core/"
)

var sharedTmpRootLiteral = `"/tmp/` + `p"`

func goDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), "go")
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatalf("go module dir not found at %s: %v", dir, err)
	}
	return dir
}

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

func probeSource(pkgName, constName string) string {
	return "package " + pkgName + "\n\n" +
		"// Transient probe planted by the cycle-1128 ACS predicate. Removed by\n" +
		"// t.Cleanup before the predicate returns; never commit this file.\n" +
		"const " + constName + " = " + sharedTmpRootLiteral + "\n"
}

func requireCleanBaseline(t *testing.T, dir string) {
	t.Helper()
	out, code := runGuard(t, dir)
	if code != 0 {
		t.Fatalf("baseline: guard %s is already RED before planting a probe (exit=%d); "+
			"the negative predicate below would be meaningless\n%s", guardTest, code, out)
	}
}

func TestC1128_001_GuardGreenOnCleanWorktree(t *testing.T) {
	dir := goDir(t)
	out, code := runGuard(t, dir)
	if code != 0 {
		t.Errorf("guard %s must be GREEN on the clean worktree, got exit=%d; "+
			"a widened walk that reds on an unviolated tree (e.g. by matching the guard's own "+
			"source, or a non-test file) is a false positive, not hardening\n%s", guardTest, code, out)
	}
}

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
