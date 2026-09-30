//go:build acs

package cycle466

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	fleetPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	cmdPkg    = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	policyPkg = "github.com/mickeyyaya/evolve-loop/go/internal/policy"
	tricapPkg = "github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

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

func TestC466_001_DispatchIterationEmptyPlanNeverClaimsAWave(t *testing.T) {
	out, code := runGoTest(t, "TestDispatchIteration", true, cmdPkg)
	requireTestsRan(t, out, 5)
	if code != 0 {
		t.Errorf("dispatchIteration contract (incl. D1 empty-plan guard) is red (exit=%d)\n%s", code, out)
	}
}

func TestC466_002_PlanFromTriageProductionFixtureThreadsRealCards(t *testing.T) {
	out, code := runGoTest(t, "TestPlanFromTriage_ProductionFixtureTopNOnlyFallback", true, fleetPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("PlanFromTriage production-fixture top_n fallback contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC466_003_NegativeAndEmptyRejectedSequentialFallback(t *testing.T) {
	fleetOut, fleetCode := runGoTest(t, "TestPlanFromTriage_MalformedDecisionJSON_RejectsNotGuesses|TestPlanFromTriage_EmptyInputsNeverOverSchedule", true, fleetPkg)
	requireTestsRan(t, fleetOut, 2)
	if fleetCode != 0 {
		t.Errorf("PlanFromTriage malformed/empty rejection contract is red (exit=%d)\n%s", fleetCode, fleetOut)
	}
	cmdOut, cmdCode := runGoTest(t, "TestDispatchIteration_MalformedTriagePlanFallsBackSequential", true, cmdPkg)
	requireTestsRan(t, cmdOut, 1)
	if cmdCode != 0 {
		t.Errorf("dispatchIteration malformed-plan sequential-fallback contract is red (exit=%d)\n%s", cmdCode, cmdOut)
	}
}

func TestC466_004_RepoGatesRaceCleanFullSuite(t *testing.T) {
	out, code := runGoTest(t, "", true, cmdPkg, fleetPkg, policyPkg, tricapPkg)
	if code != 0 {
		t.Errorf("full-package -race regression on cmd/evolve, internal/fleet, internal/policy, internal/triagecap is red (exit=%d)\n%s", code, out)
	}
	goldenOut, goldenCode := runGoTest(t, "TestShouldRunWave", true, cmdPkg)
	requireTestsRan(t, goldenOut, 1)
	if goldenCode != 0 {
		t.Errorf("absent-fleet-block shouldRunWave golden is red (exit=%d)\n%s", goldenCode, goldenOut)
	}
}

func TestC466_005_VetAndApicoverEnforceCleanOnTouchedPackages(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	vetOut, _, vetCode, _ := acsassert.SubprocessOutput("bash", "-c", "cd "+goDir+" && go vet ./cmd/evolve/... ./internal/fleet/...")
	if vetCode != 0 {
		t.Errorf("go vet ./cmd/evolve/... ./internal/fleet/... is red (exit=%d)\n%s", vetCode, vetOut)
	}
	cmd := "cd " + goDir + " && " +
		"go build -o bin/apicover ./cmd/apicover && " +
		"go test -coverprofile=coverage.wavesalvage466.txt ./internal/fleet/ >/dev/null && " +
		"go tool cover -func=coverage.wavesalvage466.txt > coverage.wavesalvage466.func.txt && " +
		"bin/apicover -enforce -cover coverage.wavesalvage466.func.txt $(go list -f '{{.Dir}}' ./internal/fleet)"
	out, _, code, _ := acsassert.SubprocessOutput("bash", "-c", cmd)
	if code != 0 {
		t.Errorf("apicover -enforce over internal/fleet is red (exit=%d)\n%s", code, out)
	}
}

func TestC466_006_SingleTopNCardCountFourYieldsOneLane(t *testing.T) {
	out, code := runGoTest(t, "TestPlanFromTriage_SingleTopNCardCountFourYieldsOneLane", true, fleetPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("PlanFromTriage single-card count=4 no-padding contract is red (exit=%d)\n%s", code, out)
	}
}
