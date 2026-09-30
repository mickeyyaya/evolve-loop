//go:build acs

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
	promotedPkg        = "go/acs/regression/cycle1270"
	orphanPkg          = "go/acs/cycle1270"
	promotedImportPath = "./acs/regression/cycle1270"

	d1MintTest     = "TestC1270_006_MintedScratchCwdClearsTheFleetGuard"
	d1DispatchTest = "TestC1270_007_RetroFleetDispatchCarriesLaneWorktreeEndToEnd"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

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

func TestC1277_001_D1ProofExecutesUnderTheCIEnforcedGlob(t *testing.T) {
	root := acsassert.RepoRoot(t)
	const enforcedGlob = "./acs/regression/..."

	for _, f := range []string{".github/workflows/ci.yml", "go/Makefile"} {
		p := filepath.Join(root, f)
		if !acsassert.FileContains(t, p, "-tags acs "+enforcedGlob) {
			t.Fatalf("%s no longer runs `-tags acs %s` — the durable ACS gate moved; "+
				"this predicate's notion of \"CI-enforced\" needs updating with it", f, enforcedGlob)
		}
	}

	out, code := runGo(t, "list", "-tags", "acs", promotedImportPath)
	if code != 0 {
		t.Fatalf("%s does not resolve as a Go package (exit %d) — the cycle-1270 predicates are "+
			"not in the tree `%s` walks, so the cycle-1255 D1 stale-worktree-fallback proof is "+
			"still invisible to CI:\n%s", promotedImportPath, code, enforcedGlob, out)
	}
	if got := strings.TrimSpace(out); !strings.HasSuffix(got, "/acs/regression/cycle1270") {
		t.Fatalf("go list resolved %s to %q, want a package under acs/regression/", promotedImportPath, got)
	}

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
