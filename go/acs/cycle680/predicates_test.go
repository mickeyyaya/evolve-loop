//go:build acs

// Package cycle680 materializes the cycle-680 acceptance criteria for the sole
// committed top_n task selfcheck-breaker-fail-loud (triage-report.md ## top_n;
// the scout's chronicle-s2-digest-writer pick was DEFERRED by triage under the
// fleet_scope constraint — its red tests are preserved in the cycle workspace
// under deferred-chronicle-red-tests/ for the future cycle that commits it).
//
// AC map (1:1):
//
//	AC1 selfcheck MkdirAll/write failure WARNs to stderr  → C680_001/002
//	AC2 selfcheck healthy write stays silent + roundtrips → C680_003/004
//	AC3 breaker persist failure WARNs, prior state kept   → C680_005/006
//	AC4 breaker healthy write stays silent                → C680_007
//
// Each predicate shells `go test -run '^<name>$' -v` over the unit-test
// contract, which EXERCISES the SUT (writeBuildSelfCheckArtifact over real
// temp worktrees; the contract-gate breaker writer over real temp paths) —
// behavioral via subprocess, no source-grep predicates (cycle-85 rule). The
// `-v` + "--- PASS:" guard rejects a rename/no-tests-matched silent green.
package cycle680

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const modRoot = "github.com/mickeyyaya/evolve-loop/go/internal/"

// runGoTest executes the named unit test in pkg and requires an explicit
// verbose PASS marker so the predicate fails on: compile failure, test
// failure, OR the test not existing (rename gaming).
func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", "^"+name+"$", modRoot+pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test %s reported no PASS for %s (renamed or not run?)\nstdout:\n%s", pkg, name, stdout)
	}
}

func TestC680_001_SelfCheckWriteFailureSurfacesWARN(t *testing.T) {
	runGoTest(t, "core", "TestWriteBuildSelfCheck_WriteFailureSurfaces")
}

func TestC680_002_SelfCheckMkdirFailureWARNsNoPanic(t *testing.T) {
	runGoTest(t, "core", "TestWriteBuildSelfCheckArtifact_MkdirAllFailure_WARNsAndDoesNotPanic")
}

func TestC680_003_SelfCheckHealthyWriteIsSilent(t *testing.T) {
	runGoTest(t, "core", "TestWriteBuildSelfCheck_HealthyWriteIsSilent")
}

func TestC680_004_SelfCheckHealthyRoundTrip(t *testing.T) {
	runGoTest(t, "core", "TestWriteBuildSelfCheckArtifact_HealthyRoundTrip_LargeAndUnicodeContent")
}

func TestC680_005_BreakerPersistFailureWARNs(t *testing.T) {
	runGoTest(t, "deliverable", "TestBreakerWriteFailureLogged")
}

func TestC680_006_BreakerTmpWriteFailureKeepsPriorState(t *testing.T) {
	runGoTest(t, "deliverable", "TestWriteBreaker_TmpWriteFailure_WARNsAndLeavesPriorStateUnchanged")
}

func TestC680_007_BreakerHealthyWriteIsSilent(t *testing.T) {
	runGoTest(t, "deliverable", "TestBreakerWriteSuccess_NoStderrNoise")
}
