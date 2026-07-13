//go:build acs

// Package cycle806 materializes the cycle-806 acceptance criteria for this
// fleet lane's sole committed defect, fix-fleet-soak-red-ci (triage top_n:
// sweep-tombstone-attribution, soak-invariants-reconcile, ciparity-integration-
// tier). Per R9.3 no predicate binds to any deferred/dropped item
// (ciparity-jobmatrix-superset-pin is out of scope).
//
// Every predicate EXECUTES the system under test as a subprocess (`go test` of
// a named behavioral unit/integration test) and requires an explicit
// `--- PASS: <name>` marker — exit 0 alone would also cover the "0 tests
// matched" case (a renamed/removed test), which must fail the predicate, not
// pass it. No source-grep predicates over logic files (cycle-85 rule).
//
// AC map (1:1, from scout-report.md Selected Tasks + Acceptance Criteria):
//
//	Task sweep-tombstone-attribution
//	  AC1.1 tombstone-aware resolver reads a reaped registry
//	        → C806_001 sessionrecord.TestReadAllResolving_ReadsTombstoneAfterReap
//	  AC1.2 (negative) missing live+tombstone → zero records, no fabrication
//	        → C806_002 sessionrecord.TestReadAllResolving_MissingBothIsZeroNoFabrication
//	  AC1.3 (edge) live+tombstone same session → no double-count (union dedup)
//	        → C806_003 sessionrecord.TestReadAllResolving_LiveAndTombstoneNoDoubleCount
//	  AC1.4 attribution survives a real ReapOrphans tombstone end-to-end
//	        → C806_004 sessionreaper.TestReapOrphans_AttributionDiscoverableAfterTombstone
//	Task soak-invariants-reconcile
//	  AC2.1 the integration-tier soak test is GREEN under the new contract
//	        (N tombstones + idempotent second reap + attribution via resolver;
//	        UNKNOWN/cross-run branch kept) → C806_005 runs the real
//	        TestFleetSoak_AllFourInvariants with -tags integration
//	Task ciparity-integration-tier
//	  AC3.1 integration-tier gate offenders FAIL audit
//	        → C806_006 audit.TestRun_IntegrationTierGate_Offenders_FAILsAudit
//	  AC3.2 (edge) default gate no-ops without a go module
//	        → C806_007 audit.TestIntegrationTierCheckDefault_NoOpWithoutGoModule
//	  AC3.3 (anti-drift) NewDefault wires a gate that truly runs -tags integration
//	        → C806_008 audit.TestNewDefault_WiresIntegrationTierGate
//
// Adversarial axes: negative (AC1.2 no-fabrication), edge (AC1.3 dedup, AC3.2
// no-op), semantic (resolver read vs end-to-end reaper attribution vs soak
// invariants vs gate FAIL vs tag membership are distinct behaviors, not one
// restated).
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

// runNamedTest runs one named Test in pkg under -race and requires its verbose
// PASS marker. tag, when non-empty, is passed as a single -tags value. An exit 0
// with no PASS marker (test missing/renamed/0-matched) fails the predicate.
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

// AC1.1 — tombstone-aware resolver reads a reaped registry.
func TestC806_001_resolver_reads_tombstone_after_reap(t *testing.T) {
	runNamedTest(t, sessionrecordPkg, "TestReadAllResolving_ReadsTombstoneAfterReap", "")
}

// AC1.2 (negative) — missing live + tombstone resolves to zero, no fabrication.
func TestC806_002_resolver_missing_both_is_zero(t *testing.T) {
	runNamedTest(t, sessionrecordPkg, "TestReadAllResolving_MissingBothIsZeroNoFabrication", "")
}

// AC1.3 (edge) — live + tombstone with same session must not double-count.
func TestC806_003_resolver_no_double_count(t *testing.T) {
	runNamedTest(t, sessionrecordPkg, "TestReadAllResolving_LiveAndTombstoneNoDoubleCount", "")
}

// AC1.4 — attribution survives a real ReapOrphans tombstone end-to-end.
func TestC806_004_attribution_survives_tombstone(t *testing.T) {
	runNamedTest(t, sessionreaperPkg, "TestReapOrphans_AttributionDiscoverableAfterTombstone", "")
}

// AC2.1 — the integration-tier soak test is GREEN under the reconciled contract.
// This is the exact CI job that was red (the -tags integration tier) — the
// predicate runs it the way go.yml does.
func TestC806_005_soak_all_invariants_green_under_integration(t *testing.T) {
	runNamedTest(t, cmdEvolvePkg, "TestFleetSoak_AllFourInvariants", "integration")
}

// AC3.1 — integration-tier gate offenders FAIL audit.
func TestC806_006_integration_gate_offenders_fail_audit(t *testing.T) {
	runNamedTest(t, auditPkg, "TestRun_IntegrationTierGate_Offenders_FAILsAudit", "")
}

// AC3.2 (edge) — default gate no-ops without a go module.
func TestC806_007_integration_gate_noop_without_go_module(t *testing.T) {
	runNamedTest(t, auditPkg, "TestIntegrationTierCheckDefault_NoOpWithoutGoModule", "")
}

// AC3.3 (anti-drift) — NewDefault wires a gate that truly runs -tags integration.
func TestC806_008_newdefault_wires_integration_tier(t *testing.T) {
	runNamedTest(t, auditPkg, "TestNewDefault_WiresIntegrationTierGate", "")
}
