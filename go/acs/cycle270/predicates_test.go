//go:build acs

package cycle270

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	lpOnce sync.Once
	lpOut  string
)

func runLoopPreflightSuite(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	lpOnce.Do(func() {
		goDir := filepath.Join(root, "go")
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", goDir, "-count=1", "-cover", "-v",
			"./internal/looppreflight/...")
		lpOut = stdout + "\n" + stderr
	})
	return lpOut
}

var (
	coverageRe = regexp.MustCompile(`coverage:\s+([0-9.]+)%\s+of statements`)
	topPassRe  = regexp.MustCompile(`(?m)^--- PASS: (Test\w+)`)
	anyFailRe  = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
)

func parseCoverage(out string) float64 {
	m := coverageRe.FindStringSubmatch(out)
	if m == nil {
		return -1
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return -1
	}
	return v
}

func topLevelPassed(out, name string) bool {
	for _, m := range topPassRe.FindAllStringSubmatch(out, -1) {
		if m[1] == name {
			return true
		}
	}
	return false
}

func subPasses(out, parent string) map[string]bool {
	re := regexp.MustCompile(`(?m)^\s+--- PASS: ` + regexp.QuoteMeta(parent) + `/(\S+)`)
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(out, -1) {
		seen[m[1]] = true
	}
	return seen
}

func TestC270_001_CoverageAtLeast82(t *testing.T) {
	out := runLoopPreflightSuite(t)
	cov := parseCoverage(out)
	if cov < 0 {
		t.Fatalf("RED: no `coverage: N%% of statements` line — suite did not build/run.\n%s", out)
	}
	if cov < 82.0 {
		t.Errorf("RED: looppreflight coverage = %.1f%%, want >= 82.0%% (baseline 69.3%%)", cov)
	}
}

func TestC270_002_SuitePassesWithRace(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-race", "-count=1",
		"./internal/looppreflight/...")
	combined := stdout + "\n" + stderr
	if code != 0 {
		low := strings.ToLower(combined)
		if strings.Contains(low, "-race requires cgo") ||
			strings.Contains(low, "race detector not supported") ||
			strings.Contains(low, "requires cgo") {
			t.Skipf("race detector unsupported on this lane; skipping C2:\n%s", combined)
		}
		t.Errorf("RED: `go test -race ./internal/looppreflight/...` exit=%d (race or test failure)\n%s", code, combined)
	}
	if anyFailRe.MatchString(combined) {
		t.Errorf("RED: looppreflight suite has a FAIL line under -race\n%s", combined)
	}
}

func TestC270_003_DefaultDirWritableTested(t *testing.T) {
	out := runLoopPreflightSuite(t)
	if !topLevelPassed(out, "TestDefaultDirWritable") {
		t.Errorf("RED: TestDefaultDirWritable did not run+PASS — defaultDirWritable (host.go) is still 0%% covered")
	}
	if subs := subPasses(out, "TestDefaultDirWritable"); len(subs) < 2 {
		t.Errorf("RED: TestDefaultDirWritable has %d passing sub-cases, want >= 2 (writable positive + unwritable/empty negative)", len(subs))
	}
}

func TestC270_004_DefaultTmuxSessionsTested(t *testing.T) {
	out := runLoopPreflightSuite(t)
	if !topLevelPassed(out, "TestDefaultTmuxSessions") {
		t.Errorf("RED: TestDefaultTmuxSessions did not run+PASS — defaultTmuxSessions (host.go) is still 0%% covered")
	}
}

func TestC270_005_BootRCNameTested(t *testing.T) {
	out := runLoopPreflightSuite(t)
	if !topLevelPassed(out, "TestBootRCName") {
		t.Errorf("RED: TestBootRCName did not run+PASS — bootRCName (boot.go) was only 33.3%% covered")
	}
	if subs := subPasses(out, "TestBootRCName"); len(subs) < 2 {
		t.Errorf("RED: TestBootRCName has %d passing sub-cases, want >= 2 (a known exit code + the unknown/default-branch case)", len(subs))
	}
}

func TestC270_006_ResolveNilDefaultsTested(t *testing.T) {
	out := runLoopPreflightSuite(t)
	if !topLevelPassed(out, "TestResolve_NilDefaults") {
		t.Errorf("RED: TestResolve_NilDefaults did not run+PASS — resolve() nil-default branches (looppreflight.go:171) are still under-covered")
	}
}
