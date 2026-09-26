package core

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

// enrolledPatterns reads the real go/.apicover-enforce, because the point is to gate the checked-in file.
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

// legacyACSCeiling is the highest grandfathered acs cycle enrollment; anything above it is the regression.
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

func TestBuildTagVisiblePackagesFailsOpen(t *testing.T) {
	if got := buildTagVisiblePackages(context.Background(), "", nil); len(got) != 0 {
		t.Fatalf("buildTagVisiblePackages(nil) = %v, want empty", got)
	}
}
