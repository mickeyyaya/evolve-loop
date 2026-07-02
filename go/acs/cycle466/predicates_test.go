//go:build acs

// Package cycle466 materialises the cycle-466 acceptance criteria for the
// single triage-committed task (## top_n only, operator priority override):
//
//	s2-wave-salvage-fix-d1 (go/internal/fleet/triageplan.go,
//	go/cmd/evolve/cmd_loop_wave.go) → C466_001..006
//
// Salvages cycle 465's preserved worktree wave-semantics work and fixes the
// D1 defect that failed cycle 465's audit: dispatchIteration did not guard
// len(specs)==0, so an empty adapted triage plan invoked launcher.Run with a
// zero-lane spec list and returned ran=true — silently consuming a
// --max-cycles iteration doing zero work (livelock). Also fixes
// productionWavePlanFn's hardcoded cardPackages=nil by threading real
// top_n[].id card ids through PlanFromTriage, since real triage-decision.json
// artifacts (e.g. .evolve/runs/cycle-464/triage-decision.json) commonly carry
// NO committed_floors field at all — the floorless+cardless livelock is the
// COMMON path, not an edge case.
//
// 1:1 AC-materialization: 6 predicates + 0 manual+checklist + 0 removed = 6
// ACs total (see .evolve/evals/s2-wave-salvage-fix-d1.md), none
// double-counted.
//
// RED strategy (verified in test-report.md "RED Run Output"): C466_001-003
// and C466_006 fail because go/cmd/evolve/cmd_loop_wave_test.go and
// go/internal/fleet/triageplan_test.go reference dispatchIteration/
// shouldRunWave/PlanFromTriage, which do not exist yet in this worktree —
// go/cmd/evolve and go/internal/fleet both fail to COMPILE. C466_004
// (repo-gates regression) and C466_005 (apicover -enforce) are red for the
// same two compile failures.
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C466_001 (an empty adapted plan must fall through to
//	            sequential, never claim a do-nothing wave — kills a guard
//	            that still returns ran=true with empty results),
//	            C466_003 (malformed triage-decision.json must be REJECTED,
//	            not silently guessed into an unscoped launch)
//	Edge/OOD:   C466_006 (a single top_n card at count=4 yields exactly ONE
//	            spec, not four — never pad unused lanes to fc.Count)
//	Semantic:   C466_001 vs C466_002 (the empty-plan GUARD is a distinct
//	            requirement from the top_n CARD-FALLBACK path — a fix that
//	            only adds the guard still livelocks on every real
//	            cycle-464-shaped decision; both must hold independently)
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

// requireTestsRan closes the degenerate-predicate trap: `go test -run X` with
// no matching test exits 0 with "no tests to run", which would green a
// predicate on unwritten (or renamed) work.
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

// TestC466_001_DispatchIterationEmptyPlanNeverClaimsAWave (AC1, D1
// regression): EXACTLY the eval's `-run 'TestDispatchIteration'` grader —
// the full dispatchIteration contract, including the empty-plan guard
// (TestDispatchIteration_EmptyPlanNeverClaimsAWave), must be green.
func TestC466_001_DispatchIterationEmptyPlanNeverClaimsAWave(t *testing.T) {
	out, code := runGoTest(t, "TestDispatchIteration", true, cmdPkg)
	requireTestsRan(t, out, 5)
	if code != 0 {
		t.Errorf("dispatchIteration contract (incl. D1 empty-plan guard) is red (exit=%d)\n%s", code, out)
	}
}

// TestC466_002_PlanFromTriageProductionFixtureThreadsRealCards (AC2): a
// triage-decision.json shaped like the REAL cycle-464 artifact (top_n[].id
// cards, no committed_floors) with cardPackages=nil must plan >=1 lane.
// Shells the named RED unit test written this cycle.
func TestC466_002_PlanFromTriageProductionFixtureThreadsRealCards(t *testing.T) {
	out, code := runGoTest(t, "TestPlanFromTriage_ProductionFixtureTopNOnlyFallback", true, fleetPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("PlanFromTriage production-fixture top_n fallback contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC466_003_NegativeAndEmptyRejectedSequentialFallback (AC3, negative):
// malformed decision JSON must reject with zero specs (fleet package), and a
// wave-eligible dispatch driven by a malformed plan must surface the error
// and never invoke the launcher (cmd/evolve package).
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

// TestC466_004_RepoGatesRaceCleanFullSuite (AC4, CI-parity): full -race
// package regression on cmd/evolve, internal/fleet, internal/policy, and
// internal/triagecap, plus the absent-fleet-block golden re-verified.
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

// TestC466_005_VetAndApicoverEnforceCleanOnTouchedPackages (AC5,
// CI-parity): mirrors .github/workflows/go.yml's "api-coverage enforce"
// step scoped to internal/fleet — PlanFromTriage (the new exported symbol)
// must be named by a test AST AND show >0% executed coverage. Kills the
// cycle-413 gaming class (a new exported symbol shipped without a naming
// test breaks main CI's repo-wide apicover -enforce).
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

// TestC466_006_SingleTopNCardCountFourYieldsOneLane (AC6, edge/OOD): a
// triage-decision.json with exactly one top_n card and fc.Count=4 must
// produce EXACTLY 1 lane spec — never pad unused lanes to fc.Count. EXACTLY
// the eval's `-run 'TestPlanFromTriage'` grader for the edge case.
func TestC466_006_SingleTopNCardCountFourYieldsOneLane(t *testing.T) {
	out, code := runGoTest(t, "TestPlanFromTriage_SingleTopNCardCountFourYieldsOneLane", true, fleetPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("PlanFromTriage single-card count=4 no-padding contract is red (exit=%d)\n%s", code, out)
	}
}
