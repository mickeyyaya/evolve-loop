//go:build acs

package cycle281

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

var (
	coreOnce sync.Once
	coreOut  string
	advOnce  sync.Once
	advOut   string
)

func runCoreWorktree(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	coreOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", "TestInsertedPhaseWritableInheritsWorktree|TestAbortCleanupPreservesWorktreeDiff|TestInsertedReadOnlyPhaseDoesNotGetWorktree",
			"./internal/core/")
		coreOut = stdout + "\n" + stderr
	})
	return coreOut
}

func runAdversarial(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	advOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v", "-run", "TestAdversarial", "./internal/bridge/")
		advOut = stdout + "\n" + stderr
	})
	return advOut
}

var (
	passLineRe = regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
	anyFailRe  = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
)

func passNames(out string) []string {
	var names []string
	for _, m := range passLineRe.FindAllStringSubmatch(out, -1) {
		names = append(names, m[1])
	}
	return names
}

func topLevelPassed(out, name string) bool {
	for _, n := range passNames(out) {
		if n == name {
			return true
		}
	}
	return false
}

func faultCases(out string) []string {
	names := passNames(out)
	parents := map[string]bool{}
	for _, n := range names {
		if i := strings.Index(n, "/"); i >= 0 {
			parents[n[:i]] = true
		}
	}
	seen := map[string]bool{}
	for _, n := range names {
		if !strings.HasPrefix(n, "TestAdversarialFault") {
			continue
		}
		if strings.Contains(n, "/") {
			seen[n] = true
			continue
		}
		if !parents[n] {
			seen[n] = true
		}
	}
	out2 := make([]string, 0, len(seen))
	for n := range seen {
		out2 = append(out2, n)
	}
	return out2
}

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func coverageTotal(t *testing.T, pkg string) (float64, string) {
	t.Helper()
	dir := goDir(t)
	prof := filepath.Join(t.TempDir(), "cover.out")
	_, tErr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-short", "-count=1", "-coverprofile="+prof, pkg)
	funcOut, cErr, _, _ := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+prof)
	for _, ln := range strings.Split(funcOut, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(ln), "total:") {
			continue
		}
		fields := strings.Fields(ln)
		pctStr := strings.TrimSuffix(fields[len(fields)-1], "%")
		if pct, err := strconv.ParseFloat(pctStr, 64); err == nil {
			return pct, ""
		}
	}
	return -1, "test stderr:\n" + tail(tErr, 20) + "\ncover stderr:\n" + tail(cErr, 20)
}

func TestC281_001_InsertedPhaseWorktreeContract(t *testing.T) {
	out := runCoreWorktree(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED: a worktree-dispatch test FAILs:\n%s", tail(out, 40))
	}
	for _, name := range []string{
		"TestInsertedPhaseWritableInheritsWorktree",
		"TestAbortCleanupPreservesWorktreeDiff",
		"TestInsertedReadOnlyPhaseDoesNotGetWorktree",
	} {
		if !topLevelPassed(out, name) {
			t.Errorf("RED: %s did not PASS — the cycle-280 inserted-phase worktree contract is not yet satisfied", name)
		}
	}
}

func TestC281_010_AdversarialSuiteHasMinimumCases(t *testing.T) {
	out := runAdversarial(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: adversarial suite has a FAIL line:\n%s", tail(out, 40))
	}
	cases := faultCases(out)
	if len(cases) < 18 {
		t.Errorf("RED: TestAdversarialFault* has %d passing case(s), want >= 18 "+
			"(6 fault types × >= 3 driver families — scout T2 verifiableBy)", len(cases))
	}
}

func TestC281_011_AdversarialSuiteCoversAllFaultTypes(t *testing.T) {
	out := runAdversarial(t)
	norm := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(s), "-", ""), "_", "")
	}
	cases := faultCases(out)
	faultTypes := map[string][]string{
		"stall":       {"stall"},
		"crash":       {"crash"},
		"update-menu": {"updatemenu", "updatenag"},
		"weak-busy":   {"weakbusy"},
		"empty-pane":  {"emptypane"},
		"malformed":   {"malformed"},
	}
	for label, spellings := range faultTypes {
		found := false
		for _, c := range cases {
			nc := norm(c)
			for _, sp := range spellings {
				if strings.Contains(nc, sp) {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("RED: no passing adversarial case covers fault type %q (accepted: %v) — fault dimension uncovered", label, spellings)
		}
	}
}

func TestC281_012_AdversarialSuiteCoversAllDriverFamilies(t *testing.T) {
	out := runAdversarial(t)
	cases := faultCases(out)
	for _, fam := range []string{"claude", "codex", "agy"} {
		found := false
		for _, c := range cases {
			if strings.Contains(strings.ToLower(c), fam) {
				found = true
			}
		}
		if !found {
			t.Errorf("RED: no passing adversarial case covers driver family %q — family dimension uncovered", fam)
		}
	}
}

func TestC281_013_AdversarialMatrixGuardsPass(t *testing.T) {
	out := runAdversarial(t)
	for _, name := range []string{
		"TestAdversarialFaultMatrix_RequiredFamiliesCovered",
		"TestAdversarialFaultMatrix_RequiredFaultTypesPresent",
	} {
		if !topLevelPassed(out, name) {
			t.Errorf("RED: matrix-invariant guard %s did not PASS — the suite is not self-policing for completeness", name)
		}
	}
}

func TestC281_020_CoreCoverageFloor(t *testing.T) {
	pct, diag := coverageTotal(t, "./internal/core/")
	if pct < 0 {
		t.Fatalf("RED: no `total:` row from `go tool cover -func` for internal/core — profile not produced.\n%s", diag)
	}
	if pct < 90.0 {
		t.Errorf("RED: internal/core coverage = %.1f%%, want >= 90.0%% (baseline 86.2%%; failure_advisor 0%% + correction_ladder 15-70%% must be probed)", pct)
	}
}

func TestC281_021_RoutingtestCoverageFloor(t *testing.T) {
	pct, diag := coverageTotal(t, "./internal/routingtest/")
	if pct < 0 {
		t.Fatalf("RED: no `total:` row from `go tool cover -func` for internal/routingtest — profile not produced.\n%s", diag)
	}
	if pct < 90.0 {
		t.Errorf("RED: internal/routingtest coverage = %.1f%%, want >= 90.0%% (baseline 80.7%%; engine/bricks/agent 0%% funcs must be probed)", pct)
	}
}
