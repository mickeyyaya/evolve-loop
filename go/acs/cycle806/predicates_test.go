//go:build acs

package cycle806

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	sessionrecordPkg = "github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
	sessionreaperPkg = "github.com/mickeyyaya/evolve-loop/go/internal/sessionreaper"
	auditPkg         = "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
	cmdEvolvePkg     = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

func runNamedTest(t *testing.T, pkg, name, tag string) {
	t.Helper()
	args := []string{"test", "-race", "-count=1", "-v"}
	if tag != "" {
		args = append(args, "-tags", tag)
	}
	args = append(args, "-run", "^"+name+"$", pkg)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			name, pkg, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Errorf("test %s in %s did not report PASS (missing, renamed, or 0 matched)", name, pkg)
	}
}

func TestC806_001_resolver_reads_tombstone_after_reap(t *testing.T) {
	runNamedTest(t, sessionrecordPkg, "TestReadAllResolving_ReadsTombstoneAfterReap", "")
}

func TestC806_002_resolver_missing_both_is_zero(t *testing.T) {
	runNamedTest(t, sessionrecordPkg, "TestReadAllResolving_MissingBothIsZeroNoFabrication", "")
}

func TestC806_003_resolver_no_double_count(t *testing.T) {
	runNamedTest(t, sessionrecordPkg, "TestReadAllResolving_LiveAndTombstoneNoDoubleCount", "")
}

func TestC806_004_attribution_survives_tombstone(t *testing.T) {
	runNamedTest(t, sessionreaperPkg, "TestReapOrphans_AttributionDiscoverableAfterTombstone", "")
}

func TestC806_005_soak_all_invariants_green_under_integration(t *testing.T) {
	runNamedTest(t, cmdEvolvePkg, "TestFleetSoak_AllFourInvariants", "integration")
}

func TestC806_006_integration_gate_offenders_fail_audit(t *testing.T) {
	runNamedTest(t, auditPkg, "TestRun_IntegrationTierGate_Offenders_FAILsAudit", "")
}

func TestC806_007_integration_gate_noop_without_go_module(t *testing.T) {
	runNamedTest(t, auditPkg, "TestIntegrationTierCheckDefault_NoOpWithoutGoModule", "")
}

func TestC806_008_newdefault_wires_integration_tier(t *testing.T) {
	runNamedTest(t, auditPkg, "TestNewDefault_WiresIntegrationTierGate", "")
}
