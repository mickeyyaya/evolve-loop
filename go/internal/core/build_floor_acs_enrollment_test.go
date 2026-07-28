package core

// build_floor_acs_enrollment_test.go — regression pin for the cycle-1145
// gate-block (fingerprint build|gate-block|866df5da1e50), which recurred into
// cycle-1147 and blocked the lane for three attempts.
//
// Mechanism: cycles 1141/1144/1145 added ./acs/cycle<N> lines to
// go/.apicover-enforce under an invented "completeness invariant". Enrollment
// routes a package into the build floor's ENFORCED coverage run, which reads
// the untagged "build constraints exclude all Go files … [setup failed]" SETUP
// result as a TEST failure — so every subsequent cycle whose base diff carried
// those packages rejected its own handoff. These tests pin both halves of the
// fix: the enrollment file itself, and the floor's tag-visibility filter.

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// repoModuleDir returns the go/ module directory of the checkout this test runs
// in, located by walking up from the package dir until go.mod appears.
func repoModuleDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("go.mod not found above %s", dir)
	return ""
}

// enrolledPatterns reads the REAL go/.apicover-enforce (not a fixture — the
// point is to gate the checked-in file) and returns its pattern lines.
func enrolledPatterns(t *testing.T, moduleDir string) []string {
	t.Helper()
	f, err := os.Open(filepath.Join(moduleDir, ".apicover-enforce"))
	if err != nil {
		t.Fatalf("open .apicover-enforce: %v", err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan .apicover-enforce: %v", err)
	}
	return out
}

// legacyACSCeiling is the highest cycle number whose acs package predates the
// handoff-time enforced coverage run. Entries at or below it are grandfathered
// (removing them is a CI-scope change); anything above is the regression.
const legacyACSCeiling = 661

func TestAPICoverEnforceDoesNotEnrollModernACSPackages(t *testing.T) {
	moduleDir := repoModuleDir(t)
	for _, p := range enrolledPatterns(t, moduleDir) {
		n, ok := strings.CutPrefix(p, "./acs/cycle")
		if !ok {
			continue
		}
		cycle, err := strconv.Atoi(strings.TrimSuffix(n, "/..."))
		if err != nil {
			t.Errorf(".apicover-enforce enrolls %q — an acs pattern with an unparseable cycle number", p)
			continue
		}
		if cycle > legacyACSCeiling {
			t.Errorf(".apicover-enforce enrolls %q. ACS predicate packages export NOTHING and are //go:build acs, "+
				"so enrollment measures no API surface AND routes the package into the build floor's enforced "+
				"coverage run, where its untagged setup failure reads as a test failure (cycle-1145 gate-block). "+
				"Remove the line; do not re-add it.", p)
		}
	}
}

// TestBuildTagVisiblePackagesDropsACSPackages exercises the floor's second line
// of defense against the same class, using the real toolchain over the real
// checkout: an `//go:build acs` package must be dropped before the enforced
// split, while an ordinary package must survive.
func TestBuildTagVisiblePackagesDropsACSPackages(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain unavailable")
	}
	moduleDir := repoModuleDir(t)
	const acsPkg = "./acs/cycle1"
	if _, err := os.Stat(filepath.Join(moduleDir, "acs", "cycle1")); err != nil {
		t.Skipf("fixture package %s absent: %v", acsPkg, err)
	}

	got := buildTagVisiblePackages(context.Background(), moduleDir, []string{acsPkg, "./internal/phasecontract"})

	for _, p := range got {
		if p == acsPkg {
			t.Errorf("buildTagVisiblePackages kept %s — a build-tagged package reaching the enforced "+
				"coverage run fails the handoff on a SETUP condition, not a real test failure", acsPkg)
		}
	}
	if !contains(got, "./internal/phasecontract") {
		t.Errorf("buildTagVisiblePackages dropped ./internal/phasecontract (kept %v) — the filter must narrow "+
			"ONLY tag-invisible packages, never a package the floor is supposed to judge", got)
	}
}

// TestBuildTagVisiblePackagesFailsOpen pins the fail-open contract: with no
// packages to inspect the filter narrows nothing, so a plumbing edge can never
// silently empty the floor's work list.
func TestBuildTagVisiblePackagesFailsOpen(t *testing.T) {
	if got := buildTagVisiblePackages(context.Background(), "", nil); len(got) != 0 {
		t.Fatalf("buildTagVisiblePackages(nil) = %v, want empty", got)
	}
}
