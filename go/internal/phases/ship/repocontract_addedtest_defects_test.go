package ship

// repocontract_addedtest_defects_test.go — cycle-1566 RED contract for the
// added-test backstop's audited defects.
//
// The backstop (repocontract.go, addedTestPackages + the "added-test backstop"
// runClassifiedPack call) closes the red-first-deliverable-reds-main incident
// class: three landings pushed a genuinely failing newly-added `*_test.go`
// with no ship-time consumer and redded main. Cycle-1559's audit reproduced
// four defects in that backstop against the real gate; they are OPEN in the
// tree today (.evolve/runs/cycle-1566/defect-dispositions.json). Each test
// below drives the REAL gate (no seam swap except where the fixed pack itself
// is the subject) against a REAL temporary git repo, so none of them can be
// satisfied by a source-level string.
//
//	H1 — a build-tag-guarded added test that genuinely FAILS is compiled out of
//	     the untagged backstop run; its package reports green and the failure
//	     ships silently. This is pinned incident 25040cea's exact shape.
//	H2 — a LONE tag-guarded added test package (the `//go:build acs` predicate
//	     file every cycle mints, including this one) has no files under the
//	     default tag set, which classifyPackEvents grades a genuine RED. That
//	     hard-blocks the lane's own honest ship with a false CodeRepoContractGate.
//	M1 — a `git diff --cached` error disables the whole backstop and writes
//	     nothing anywhere: the ship proceeds believing it was scanned.
//	M2 — the shared red message dropped the four fixed-suite names and cannot
//	     tell an operator whether the fixed pack or an added test went red.
//
// The gate-level skip case is pinned here too: the runNative half already
// exists (TestRunNative_AddedSkippedTestDoesNotBlockShip), but scout's Task 1
// acceptance criterion is stated at the gate, and that half had no test.

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

// newLaneRepo builds a temporary lane worktree that is GREEN for the four
// fixed guard suites, so anything a test observes afterwards is attributable
// to the added-test backstop alone.
func newLaneRepo(t *testing.T) string {
	t.Helper()
	repo := makeRepo(t)
	goDir := filepath.Join(repo, "go")
	mustWrite(t, filepath.Join(goDir, "go.mod"), "module example.com/lane\n\ngo 1.24\n")
	for _, pkg := range []string{"phasespec", "profiles", "phasecoherence", "routingtest"} {
		mustWrite(t, filepath.Join(goDir, "internal", pkg, "pass_test.go"),
			"package "+pkg+"\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {}\n")
	}
	runGit(t, repo, "add", "go")
	runGit(t, repo, "commit", "-qm", "baseline: four green guard suites")
	return repo
}

// readScanLog returns the run-dir scanner artifact, failing the test when it
// is absent — every gate run, green or red, owes one.
func readScanLog(t *testing.T, workspace string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(workspace, scanLogName))
	if err != nil {
		t.Fatalf("scan log must be written on every gate run: %v", err)
	}
	return string(body)
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

// TestRepoContractGate_NewlyAddedSkippedTestDoesNotBlockShip is scout Task 1's
// negative criterion at the gate itself: a deliberately red-first reproducer
// that lands `t.Skip`-ped is the SAFE intermediate state and must ship. A gate
// that graded skip as failure would make the documented land-with-skip
// convention unusable and push lanes back to landing bare reds.
func TestRepoContractGate_NewlyAddedSkippedTestDoesNotBlockShip(t *testing.T) {
	repo := newLaneRepo(t)
	const path = "go/internal/reproduction/skip_test.go"
	mustWrite(t, filepath.Join(repo, path),
		"package reproduction\n\nimport \"testing\"\n\nfunc TestNewlyAddedSkipped(t *testing.T) { t.Skip(\"pending fix: tracked red-first reproducer\") }\n")
	runGit(t, repo, "add", path)

	ws := t.TempDir()
	if err := runRepoContractGate(context.Background(), "enforce", repo, ws, io.Discard); err != nil {
		t.Fatalf("a t.Skip-ped newly added reproducer must not block the ship, got %v", err)
	}
	if log := readScanLog(t, ws); strings.Contains(log, "--- FAIL: TestNewlyAddedSkipped") {
		t.Fatalf("a skipped reproducer must not be recorded as a failure, got:\n%s", log)
	}
}

// TestRepoContractGate_NewlyAddedTaggedFailingTestIsNotSilentlyGreen pins H1
// — incident 25040cea's shape. The added file carries `//go:build integration`
// and fails; its package also holds an untagged green test, so an untagged
// backstop run reports the package `ok` and the ship sails with a red test in
// its diff. The backstop must run each added candidate under the build tags
// that file actually declares, and fail closed naming the failing test.
func TestRepoContractGate_NewlyAddedTaggedFailingTestIsNotSilentlyGreen(t *testing.T) {
	repo := newLaneRepo(t)
	pkgDir := filepath.Join(repo, "go", "internal", "reproduction")
	mustWrite(t, filepath.Join(pkgDir, "green_test.go"),
		"package reproduction\n\nimport \"testing\"\n\nfunc TestUntaggedGreen(t *testing.T) {}\n")
	const failingTest = "TestNewlyAddedIntegrationRed"
	mustWrite(t, filepath.Join(pkgDir, "integration_test.go"),
		"//go:build integration\n\npackage reproduction\n\nimport \"testing\"\n\nfunc "+failingTest+"(t *testing.T) { t.Fatal(\"deliberate red behind a build tag\") }\n")
	runGit(t, repo, "add", "go/internal/reproduction")

	err := runRepoContractGate(context.Background(), "enforce", repo, t.TempDir(), io.Discard)
	if err == nil {
		t.Fatalf("a newly added FAILING test behind //go:build integration must block the ship; the untagged run reporting the package green is exactly incident 25040cea")
	}
	se, ok := shiperr.AsShipError(err)
	if !ok || se.Code != shiperr.CodeRepoContractGate {
		t.Fatalf("error = %v, want structured REPO_CONTRACT_GATE", err)
	}
	if !strings.Contains(err.Error(), failingTest) {
		t.Fatalf("gate error must name the tag-guarded failing test %s, got %q", failingTest, err)
	}
}

// TestRepoContractGate_NewlyAddedTagGuardedGreenPackageIsNotFalseRed pins H2.
// A lone `//go:build acs` predicate package — which EVERY cycle mints into its
// own shipping diff, this one included — has zero files under the default tag
// set. `go test` reports that as a build/setup failure, which the fixed pack's
// classifier grades a genuine RED, so the backstop would hard-block the very
// ships it exists to protect. The candidate is healthy under its own tag and
// must not red the gate; the scan log must say what was done with it rather
// than dropping it silently.
func TestRepoContractGate_NewlyAddedTagGuardedGreenPackageIsNotFalseRed(t *testing.T) {
	repo := newLaneRepo(t)
	const path = "go/acs/cycle9999/predicates_test.go"
	mustWrite(t, filepath.Join(repo, path),
		"//go:build acs\n\npackage cycle9999\n\nimport \"testing\"\n\nfunc TestC9999_001_Healthy(t *testing.T) {}\n")
	runGit(t, repo, "add", path)

	ws := t.TempDir()
	if err := runRepoContractGate(context.Background(), "enforce", repo, ws, io.Discard); err != nil {
		t.Fatalf("a tag-guarded added test package that is GREEN under its own tag must not red the gate (this shape is minted by every cycle, including this one), got %v", err)
	}
	log := readScanLog(t, ws)
	if !strings.Contains(log, "acs/cycle9999") {
		t.Fatalf("scan log must record how the tag-guarded candidate was handled (rerun under its tag, or an explicit exclusion) — silent omission is the false-green half of the same defect; got:\n%s", log)
	}
}

// TestRepoContractGate_AddedTestDiscoveryFailureIsRecorded pins M1. When the
// staged-file query fails, addedTestPackages returns (nil, nil) and the gate
// returns green having scanned nothing, with no trace in the artifact that is
// supposed to be the record of what ran. A backstop that can disable itself
// invisibly is worse than no backstop: the ship report claims coverage it
// never had. Driven with a projectRoot that is a real Go module but NOT a git
// repository, which is precisely how the underlying git query fails.
func TestRepoContractGate_AddedTestDiscoveryFailureIsRecorded(t *testing.T) {
	root := t.TempDir()
	goDir := filepath.Join(root, "go")
	mustWrite(t, filepath.Join(goDir, "go.mod"), "module example.com/lane\n\ngo 1.24\n")
	for _, pkg := range []string{"phasespec", "profiles", "phasecoherence", "routingtest"} {
		mustWrite(t, filepath.Join(goDir, "internal", pkg, "pass_test.go"),
			"package "+pkg+"\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {}\n")
	}

	ws := t.TempDir()
	if err := runRepoContractGate(context.Background(), "enforce", root, ws, io.Discard); err != nil {
		t.Fatalf("an undiscoverable diff is an infrastructure gap, not a contract violation — it must not fail the ship closed, got %v", err)
	}
	log := readScanLog(t, ws)
	if !strings.Contains(log, "added-test") {
		t.Fatalf("scan log must name the added-test backstop when its discovery step fails, got:\n%s", log)
	}
	if !containsAny(log, "unavailable", "failed", "disabled", "skipped") {
		t.Fatalf("scan log must state that the added-test backstop did NOT run (unavailable/failed/disabled), never leave the reader to assume it did; got:\n%s", log)
	}
}

// TestRepoContractGate_RedMessagesDistinguishFixedPackFromAddedTests pins M2.
// Both packs funnel through one message, so the operator cannot tell which
// scan went red, and the addition dropped the four fixed-suite names that told
// them where to look. Both halves are asserted against the real error text.
func TestRepoContractGate_RedMessagesDistinguishFixedPackFromAddedTests(t *testing.T) {
	t.Run("fixed pack RED still names the four guard suites", func(t *testing.T) {
		swapRepoContractTest(t, redPack("internal/phasespec.TestCatalogParity"))
		err := runRepoContractGate(context.Background(), "enforce", t.TempDir(), t.TempDir(), io.Discard)
		if err == nil {
			t.Fatal("fixed-pack RED must fail the ship")
		}
		for _, suite := range []string{"phasespec", "profiles", "phasecoherence", "routingtest"} {
			if !strings.Contains(err.Error(), suite) {
				t.Errorf("fixed-pack RED must keep naming the guard suite %q so the operator knows where to look, got %q", suite, err)
			}
		}
	})

	t.Run("added-test RED identifies itself as an added-test failure", func(t *testing.T) {
		repo := newLaneRepo(t)
		const path = "go/internal/reproduction/red_test.go"
		const failingTest = "TestNewlyAddedRedNeedsAttribution"
		mustWrite(t, filepath.Join(repo, path),
			"package reproduction\n\nimport \"testing\"\n\nfunc "+failingTest+"(t *testing.T) { t.Fatal(\"deliberate red\") }\n")
		runGit(t, repo, "add", path)

		err := runRepoContractGate(context.Background(), "enforce", repo, t.TempDir(), io.Discard)
		if err == nil {
			t.Fatal("added-test RED must fail the ship")
		}
		if !containsAny(err.Error(), "added-test", "newly added") {
			t.Errorf("added-test RED must say the NEWLY ADDED test is what went red, not report it as the fixed scanner pack, got %q", err)
		}
		if !strings.Contains(err.Error(), failingTest) {
			t.Errorf("added-test RED must name %s, got %q", failingTest, err)
		}
	})
}
