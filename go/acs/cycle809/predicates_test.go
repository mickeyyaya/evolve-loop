//go:build acs

// Package cycle809 materializes the cycle-809 acceptance criteria for this fleet
// lane's sole committed defect, ciparity-integration-tier-race-parity (triage
// top_n). Per R9.3 no predicate binds to any deferred/dropped item
// (integration-tier-timeout-headroom-watch, integration-tier-coverprofile-parity
// are out of scope).
//
// Defect: integrationTierCheckDefault (go/internal/phases/audit/ciparity.go:205)
// runs `go test -count=1 -tags integration <pkgs>`, but the CI step it mirrors
// (.github/workflows/go.yml:59) runs `go test -race -count=1 -tags integration`.
// `-race` is present in CI, absent from the gate → a real data race in a touched
// package passes audit clean and goes CI-red on the very step this gate exists to
// pre-empt (warnship_apicover_ci_gap disease, one flag short of parity).
//
// Every predicate EXECUTES the system under test as a subprocess (`go test` of a
// named behavioral unit test) and requires an explicit `--- PASS: <name>` marker
// — exit 0 alone would also cover the "0 tests matched" case (a renamed/removed
// test), which must fail the predicate, not pass it. No source-grep predicate
// over logic files, and specifically no grep for the literal string "-race" (the
// cycle-85 degenerate-predicate failure mode, which a no-op would satisfy).
//
// AC map (1:1, from scout-report.md Acceptance Criteria Summary):
//
//	AC1 gate's `go test` invocation includes -race (proven by EFFECT: it now
//	    catches a real data race, invisible without -race)
//	      → C809_001 audit.TestIntegrationTierGate_Race
//	AC2 the regression test proves detection of a REAL race, not flag-string
//	    presence (the same fixture PASSES under plain -tags integration)
//	      → C809_002 audit.TestIntegrationTierGate_RaceFixtureIsRaceOnly
//	AC3 the pre-existing integration-tier gate tests remain GREEN (no drift)
//	      → C809_003 runs the three cycle-806 gate tests, each must PASS
//	AC4 go vet / -race / apicover -enforce clean on the touched package
//	      → manual+checklist (Auditor; the audit phase's own CI-parity gates run
//	        exactly these — see test-report.md)
//
// Adversarial axes: positive (AC1 gate catches the race under -race), negative
// (AC2 fixture passes WITHOUT -race, proving race-only detection), semantic (AC3
// existing offenders-FAIL / no-op / tag-membership behaviors are distinct, not
// one restated).
package cycle809

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const auditPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"

// runNamedTest runs one named Test in pkg under -race and requires its verbose
// PASS marker. An exit 0 with no PASS marker (test missing/renamed/0-matched)
// fails the predicate — the anti-gaming guard against a deleted behavioral test
// silently greening the suite.
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

// AC1 — the gate includes -race, proven by effect: it catches a genuine data
// race that is invisible to a plain `-tags integration` run. RED until Builder
// adds -race to integrationTierCheckDefault (ciparity.go:205).
func TestC809_001_gate_catches_real_data_race(t *testing.T) {
	runNamedTest(t, auditPkg, "TestIntegrationTierGate_Race")
}

// AC2 (negative / anti-gaming) — the race fixture PASSES under plain
// `-tags integration` (no -race), proving C809_001's detection is a genuine race
// caught by -race, not a compile/logic failure a flag-string flip would surface.
func TestC809_002_race_fixture_is_race_only(t *testing.T) {
	runNamedTest(t, auditPkg, "TestIntegrationTierGate_RaceFixtureIsRaceOnly")
}

// AC3 (regression / no-drift) — the three cycle-806 integration-tier gate tests
// must stay GREEN after -race is added: offenders still FAIL audit, the default
// gate still no-ops without a go module, and NewDefault still wires a gate that
// truly runs `-tags integration`.
func TestC809_003_existing_gate_tests_remain_green(t *testing.T) {
	for _, name := range []string{
		"TestRun_IntegrationTierGate_Offenders_FAILsAudit",
		"TestIntegrationTierCheckDefault_NoOpWithoutGoModule",
		"TestNewDefault_WiresIntegrationTierGate",
	} {
		runNamedTest(t, auditPkg, name)
	}
}
