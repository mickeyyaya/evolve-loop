//go:build acs

package cycle809

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const auditPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"

func runNamedTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			name, pkg, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Errorf("test %s in %s did not report PASS (missing, renamed, or 0 matched)", name, pkg)
	}
}

func TestC809_001_gate_catches_real_data_race(t *testing.T) {
	runNamedTest(t, auditPkg, "TestIntegrationTierGate_Race")
}

func TestC809_002_race_fixture_is_race_only(t *testing.T) {
	runNamedTest(t, auditPkg, "TestIntegrationTierGate_RaceFixtureIsRaceOnly")
}

func TestC809_003_existing_gate_tests_remain_green(t *testing.T) {
	for _, name := range []string{
		"TestRun_IntegrationTierGate_Offenders_FAILsAudit",
		"TestIntegrationTierCheckDefault_NoOpWithoutGoModule",
		"TestNewDefault_WiresIntegrationTierGate",
	} {
		runNamedTest(t, auditPkg, name)
	}
}
