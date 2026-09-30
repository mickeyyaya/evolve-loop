//go:build acs

package cycle464

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const policyPkg = "github.com/mickeyyaya/evolve-loop/go/internal/policy"

func runGoTest(t *testing.T, runFilter string, race bool, pkgs ...string) (out string, code int) {
	t.Helper()
	args := []string{"test", "-count=1", "-v"}
	if race {
		args = append(args, "-race")
	}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout + "\n" + stderr, code
}

func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d", got, min)
	}
}

func TestC464_001_FleetConfigResolutionDefaults(t *testing.T) {
	out, code := runGoTest(t, "TestFleetConfig_Resolution", true, policyPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("FleetConfig resolution contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC464_002_FleetConfigPlanSourceClosedVocab(t *testing.T) {
	out, code := runGoTest(t, "TestFleetConfig_PlanSourceClosedVocab", true, policyPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("FleetConfig plan_source closed-vocab contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC464_003_PolicyPackageRegression(t *testing.T) {
	out, code := runGoTest(t, "", true, policyPkg)
	if code != 0 {
		t.Errorf("internal/policy package regression is red (exit=%d)\n%s", code, out)
	}
}

func TestC464_004_ApicoverNamingEnforced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	cmd := "cd " + goDir + " && " +
		"go build -o bin/apicover ./cmd/apicover && " +
		"go test -coverprofile=coverage.fleetpolicy464.txt ./internal/policy/ >/dev/null && " +
		"go tool cover -func=coverage.fleetpolicy464.txt > coverage.fleetpolicy464.func.txt && " +
		"bin/apicover -enforce -cover coverage.fleetpolicy464.func.txt $(go list -f '{{.Dir}}' ./internal/policy)"
	out, _, code, _ := acsassert.SubprocessOutput("bash", "-c", cmd)
	if code != 0 {
		t.Errorf("apicover -enforce over internal/policy is red (exit=%d)\n%s", code, out)
	}
}

// acs-predicate: config-check
func TestC464_005_NoNewFleetEnvFlags(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	for _, name := range []string{"EVOLVE_FLEET_COUNT", "EVOLVE_FLEET_CONCURRENCY", "EVOLVE_FLEET_PLAN"} {
		_, _, code, _ := acsassert.SubprocessOutput("grep", "-rln", name,
			filepath.Join(goDir, "internal"), filepath.Join(goDir, "cmd"))
		if code == 0 {
			t.Errorf("production Go code under go/internal or go/cmd references %q — "+
				"the fleet block must be policy.json-only, never a new env flag", name)
		}
	}
}

func TestC464_006_RuntimeReferenceFleetKeyTable(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	cmd := "awk '/fleet/,0' " + doc + " | grep -c -E 'plan_source|concurrency|count'"
	out, _, code, _ := acsassert.SubprocessOutput("bash", "-c", cmd)
	if code != 0 {
		t.Errorf("fleet key-table grep is red (exit=%d, likely zero matches — the fleet section is absent)\n%s", code, out)
		return
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		t.Errorf("fleet key-table match count %q did not parse: %v", out, err)
		return
	}
	if n < 3 {
		t.Errorf("fleet key-table match count = %d, want >= 3 (count/concurrency/plan_source all present, not just a stub \"fleet\" mention)", n)
	}
}

func TestC464_007_ControlFlagsClosedVocabDocumented(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	cmd := "grep -E 'plan_source' " + doc + " | grep -E 'triage' | grep -c -E 'manual'"
	out, _, code, _ := acsassert.SubprocessOutput("bash", "-c", cmd)
	if code != 0 {
		t.Errorf("control-flags.md plan_source closed-vocab documentation is red (exit=%d, expected >= 1 line naming both triage and manual)\n%s", code, out)
	}
}

func TestC464_008_SequentialDefaultStated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	cmd := "grep -rn -i 'sequential' " + doc + " | grep -c -i 'fleet\\|count'"
	out, _, code, _ := acsassert.SubprocessOutput("bash", "-c", cmd)
	if code != 0 {
		t.Errorf("sequential-default statement is red (exit=%d, expected >= 1 line pairing \"sequential\" with fleet/count)\n%s", code, out)
	}
}

// acs-predicate: config-check
func TestC464_009_NoNewFleetEnvFlagsInDocs(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docsDir := filepath.Join(root, "docs")
	for _, name := range []string{"EVOLVE_FLEET_COUNT", "EVOLVE_FLEET_CONCURRENCY", "EVOLVE_FLEET_PLAN"} {
		_, _, code, _ := acsassert.SubprocessOutput("grep", "-rln", name, docsDir)
		if code == 0 {
			t.Errorf("docs reference %q — the fleet block must be policy.json-only, never a new env flag", name)
		}
	}
}
