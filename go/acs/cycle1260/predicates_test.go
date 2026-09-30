//go:build acs

package cycle1260

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const goTestTimeout = 10 * time.Minute

func runScoped(t *testing.T, pkg, runPattern string, extraArgs ...string) (string, int) {
	t.Helper()
	moduleDir := filepath.Join(acsassert.RepoRoot(t), "go")

	ctx, cancel := context.WithTimeout(context.Background(), goTestTimeout)
	defer cancel()

	args := append([]string{"test", "-count=1", "-run", runPattern}, extraArgs...)
	args = append(args, pkg)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleDir
	cmd.WaitDelay = 30 * time.Second
	out, err := cmd.CombinedOutput()

	code := 0
	if err != nil {
		var ee *exec.ExitError
		if ok := asExitError(err, &ee); ok {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	return string(out), code
}

func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

func TestC1260_001_regression_tia_policy_stage(t *testing.T) {
	out, code := runScoped(t, "./internal/policy", "TestRegressionTIAConfig")
	if code != 0 {
		t.Errorf("policy.RegressionTIAConfig contract is not satisfied (exit %d).\n%s", code, tail(out))
	}
}

func TestC1260_002_selection_failsafes(t *testing.T) {
	out, code := runScoped(t, "./internal/regressiontia", "TestSelect_")
	if code != 0 {
		t.Errorf("regressiontia.Select semantics/fail-safes are not satisfied (exit %d).\n%s", code, tail(out))
	}
}

func TestC1260_003_importer_closure_wired(t *testing.T) {
	out, code := runScoped(t, "./internal/regressiontia", "TestChangedScope_")
	if code != 0 {
		t.Errorf("reverse-dependency widening (changedpkgs.ImporterClosure) is not wired into the scope derivation (exit %d).\n%s", code, tail(out))
	}
}

func TestC1260_004_shadow_decision_emitted(t *testing.T) {
	out, code := runScoped(t, "./internal/regressiontia", "TestCompute_|TestEmit_")
	if code != 0 {
		t.Errorf("shadow decision compute/emit contract is not satisfied (exit %d).\n%s", code, tail(out))
	}
}

func TestC1260_005_audit_phase_reachability(t *testing.T) {
	out, code := runScoped(t, "./internal/phases/audit", "TestGenerateACSVerdict_")
	if code != 0 {
		t.Errorf("the shadow decision has no production caller in the audit phase (exit %d) — selection logic that never runs is the cycle-1253 dead-code shape repeated.\n%s", code, tail(out))
	}
}

// acs-predicate: config-check
func TestC1260_006_new_package_graduation(t *testing.T) {
	enroll := filepath.Join(acsassert.RepoRoot(t), "go", ".apicover-enforce")
	raw, err := os.ReadFile(enroll)
	if err != nil {
		t.Fatalf("read %s: %v", enroll, err)
	}
	if !hasEnrollLine(string(raw), "./internal/regressiontia") {
		t.Errorf("go/.apicover-enforce does not enroll ./internal/regressiontia — an unenrolled new package aborts the build phase (cycle-1218: three lanes, one halt, same cause)")
	}

	out, code := runScoped(t, "./internal/regressiontia", "Test", "-cover")
	if code != 0 {
		t.Fatalf("internal/regressiontia does not build/pass under -cover (exit %d).\n%s", code, tail(out))
	}
	pct, ok := coveragePercent(out)
	if !ok {
		t.Fatalf("no coverage figure in `go test -cover ./internal/regressiontia` output:\n%s", tail(out))
	}
	if pct < 85.0 {
		t.Errorf("internal/regressiontia line coverage = %.1f%%, want >= 85%% (the apicover Phase-5 Definition of Done the enrollment line commits to)", pct)
	}
}

func hasEnrollLine(body, pattern string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == pattern {
			return true
		}
	}
	return false
}

var coverageRe = regexp.MustCompile(`coverage:\s+([0-9.]+)% of statements`)

func coveragePercent(out string) (float64, bool) {
	m := coverageRe.FindStringSubmatch(out)
	if m == nil {
		return 0, false
	}
	pct, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return pct, true
}

func tail(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	return strings.Join(lines, "\n")
}
