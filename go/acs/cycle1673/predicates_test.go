//go:build acs

// Package cycle1673 materializes the acceptance criteria of the ONE inbox item
// this fleet lane committed (lane-scope.json todo_ids ∩ triage-report.md
// ## top_n) — and nothing else (R9.3):
//
//	lane-ship-gate-lands-red-main-package-scoped-tests (P1, weight 0.95, bug)
//
// The defect. A lane's pre-ship repo-contract gate runs a FIXED four-package
// pack (internal/phases/ship/repocontract.go repoContractPackages), so a
// landing that breaks the test of a package the lane never touched sails
// through: cycles 1657/1659 landed 4db205a8, internal/deliverable's ADR-0100
// e2e test asserted the superseded contract, and main's go CI stayed red from
// 4db205a8 until hotfix #590 — while the loop kept dispatching lanes onto it,
// because nothing in the wave boundary ever reads main's CI.
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 the lane ship gate runs the CI-equivalent suite (repo-wide, or at
//	    minimum the reverse-dependency closure of the changed packages)
//	    before a lane ship may land; a red run blocks with a named
//	    ship.error code                                                  → 001
//	AC2 the loop reads main's CI status at each wave boundary and HALTs
//	    with a named LOOP_HALT reason when main is red, instead of
//	    stacking lane ships on a red main                                → 002
//	AC3 a regression test pins that a change which breaks an UNTOUCHED
//	    package's test cannot pass the lane gate                         → 003
//	AC4 green checks proceed, while API/transport unavailability is
//	    explicitly warned/classified — never mistaken for green or red   → 004
//	AC5 (house floor) any NEW go/internal/<pkg> this change creates is
//	    enrolled in go/.apicover-enforce AND names every export in
//	    apicover_named_test.go, in the same diff                         → 005
//
// Adversarial axes (skills/adversarial-testing §6). NEGATIVE: 001/003 drive
// the REAL production ship phase over a fixture whose only red package is one
// the lane never touched AND which is already committed (so the staged
// added-test backstop cannot see it) — the fixed four-package pack is GREEN
// there, so the pre-fix binary fails this predicate and no magic string can
// green it; 004 pins that an unavailable check-run API does NOT produce the
// red halt (the false-RED direction); 005 fails a new package that is
// enrolled-but-unnamed as loudly as an unenrolled one. EDGE/OOD: a red
// package outside the fixed pack, a tracked MODIFICATION (never an addition),
// a check-run query that errors rather than answers, a repo with no new
// packages at all. SEMANTIC: gate selection (001), dispatch halt (002),
// regression pin (003), outcome classification (004), graduation (005) — five
// distinct behaviours, not one restated.
//
// Flaky-shape contract: ONE named package per `go test` invocation, always
// -run narrowed (cmd/evolve is a known-slow suite), no wall-clock bounds, no
// literal PIDs, every git call is -C anchored, no unbounded load generators.
// The one broad compile the fixture drives is the PRODUCTION gate's own, on a
// six-package hermetic module in t.TempDir() — not this package's argv.
//
// Reachability probe (cycle-644 rule): this package is a leaf — nothing
// imports go/acs/*. Its frozen pins are all on ALREADY-EXISTING exported API
// (ship.New, ship.Config.RepoContractGate, (*ship.Phase).Run, core.PhaseRequest,
// core.AsShipError, shiperr.CodeRepoContractGate), so no new production import
// edge is committed here and the cycle-644 unsatisfiable-pin shape cannot arise.
package cycle1673

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	shipPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	cmdPkg  = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

	// windowFile holds prepareIteration — the ONE boundary every wave crosses
	// before a launcher can exist (cmd_loop_window.go:30-76; the first dispatch
	// construction is at 117-119).
	windowFile = "go/cmd/evolve/cmd_loop_window.go"

	// mainCIRedStopReason is the named halt this cycle owes. It must be
	// distinct from the existing plane_diverged_halt: "main's CI is red" and
	// "the plane's history diverged" prescribe different operator remedies,
	// and collapsing them is how a red-main halt would read as a git problem.
	mainCIRedStopReason = "main_ci_red_halt"

	// planeDivergedStopReason is the pre-existing halt at the same boundary.
	// It must survive — the red-main halt is an ADDITION, never a rename.
	planeDivergedStopReason = "plane_diverged_halt"
)

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

// assertSuiteTestsPass shells `go test -run '^(names)$' -count=1 -v pkg`
// against ONE named package and requires EVERY name to print a
// `--- PASS: <name>` line. Asserting on the PASS line, never on exit 0, is
// load-bearing: a -run pattern matching NO test exits 0 with "no tests to
// run", so a still-missing binding would false-GREEN.
func assertSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", "-v", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: binding test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag). exit=%d\ncombined go-test output:\n%s",
				name, pkg, code, out)
		}
	}
}

// gitC runs git anchored to dir (never the process cwd — this test runs from
// the acs package dir, a lane worktree, and a fleet lane alike).
func gitC(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git -C %s %s: %v\n%s", dir, strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// mustWrite writes rel (relative to root), creating parents.
func mustWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// untouchedRedFixture builds a hermetic project root reproducing the
// cycle-1657/1659 incident EXACTLY:
//
//   - the four historical guard packages the fixed pack selects all exist and
//     are GREEN — so a fixed-pack selection sees nothing wrong;
//   - internal/contractpkg is the package the lane edits, and the edit is a
//     tracked MODIFICATION that is staged (git's --diff-filter=A added-test
//     backstop is blind to it, exactly as it was to 4db205a8);
//   - internal/deliverable is the UNTOUCHED package whose already-committed
//     test asserts the superseded contract and therefore goes red.
//
// Only a CI-equivalent selection — repo-wide, or the reverse-dependency
// closure of the changed package (deliverable imports contractpkg) — reaches
// it. Both sanctioned implementations go red here; the pre-fix binary does not.
func untouchedRedFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	mustWrite(t, root, "go/go.mod", "module fixture\n\ngo 1.23\n")
	for _, pkg := range []string{"phasespec", "profiles", "phasecoherence", "routingtest"} {
		mustWrite(t, root, "go/internal/"+pkg+"/guard.go",
			"package "+pkg+"\n\n// Guard is this guard suite's only symbol.\nfunc Guard() string { return \"green\" }\n")
		mustWrite(t, root, "go/internal/"+pkg+"/guard_test.go",
			"package "+pkg+"\n\nimport \"testing\"\n\nfunc TestGuardIsGreen(t *testing.T) {\n\tif Guard() != \"green\" {\n\t\tt.Fatal(\"guard suite red\")\n\t}\n}\n")
	}
	mustWrite(t, root, "go/internal/contractpkg/contract.go",
		"package contractpkg\n\n// Verdict is the contract value the lane rewrites.\nfunc Verdict() string { return \"old\" }\n")
	mustWrite(t, root, "go/internal/deliverable/deliverable.go",
		"package deliverable\n\n// Kind names the deliverable.\nfunc Kind() string { return \"code\" }\n")
	mustWrite(t, root, "go/internal/deliverable/declared_effects_e2e_test.go",
		"package deliverable\n\nimport (\n\t\"testing\"\n\n\t\"fixture/internal/contractpkg\"\n)\n\n"+
			"func TestUntouchedPackageAssertsSupersededContract(t *testing.T) {\n"+
			"\tif got := contractpkg.Verdict(); got != \"old\" {\n"+
			"\t\tt.Fatalf(\"declared-effects contract changed under an untouched package: %q\", got)\n\t}\n}\n")

	gitC(t, root, "init", "-q", "-b", "main")
	gitC(t, root, "config", "user.email", "acs@example.invalid")
	gitC(t, root, "config", "user.name", "acs cycle1673")
	gitC(t, root, "add", "-A")
	gitC(t, root, "commit", "-q", "-m", "fixture: green baseline (the untouched package's test is TRACKED, not added)")

	// The lane's change: a tracked MODIFICATION, staged the way a lane stages
	// its work before the pre-ship gate runs. --diff-filter=A never sees it.
	mustWrite(t, root, "go/internal/contractpkg/contract.go",
		"package contractpkg\n\n// Verdict is the contract value the lane rewrites.\nfunc Verdict() string { return \"new\" }\n")
	gitC(t, root, "add", "-A")
	return root
}

// runProductionShipGate drives the PRODUCTION ship phase — ship.New +
// (*Phase).Run, the same construction the orchestrator and `evolve phase ship`
// use — against root with the repo-contract gate at enforce, and returns the
// resulting error plus the gate's own scan-log side effect.
//
// This is the wiring proof (house rule 2): runRepoContractGate is unexported
// and its ONE production caller is (*Phase).runNative, reached only through
// Run. A predicate that called a helper directly would pass on dead code.
func runProductionShipGate(t *testing.T, root string) (err error, scanLog string) {
	t.Helper()
	workspace := t.TempDir()
	phase := ship.New(ship.Config{
		Runner:           sysexec.DefaultRunner,
		RepoContractGate: "enforce",
	})
	_, err = phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       1673,
		ProjectRoot: root,
		Workspace:   workspace,
		Worktree:    root,
		Context:     map[string]string{"commit_message": "acs cycle1673 fixture ship"},
		Env:         map[string]string{"EVOLVE_PROJECT_ROOT": root, "EVOLVE_PLUGIN_ROOT": root},
	})
	if raw, rerr := os.ReadFile(filepath.Join(workspace, "ship-repocontract-scan.log")); rerr == nil {
		scanLog = string(raw)
	}
	return err, scanLog
}

// gateCode unwraps the structured ship error the gate is required to return.
func gateCode(err error) shiperr.ShipErrorCode {
	if se, ok := core.AsShipError(err); ok {
		return se.Code
	}
	return ""
}

// ---------------------------------------------------------------------------
// AC1 — the lane ship gate is CI-equivalent, and a red run blocks with a
// named ship.error code.
// ---------------------------------------------------------------------------

// TestC1673_001_LaneShipGateRunsCIEquivalentSuiteAndNamesShipError — AC1.
// Behavioural and un-gameable: the production ship phase runs over a fixture
// whose ONLY red package is one the lane never touched and whose test is
// already committed. The fixed four-package pack is green there and the
// staged added-test backstop is blind to a modification, so the pre-fix
// binary returns no gate error at all; only a CI-equivalent selection blocks.
// The assertion is on the STRUCTURED code, not on "some error happened":
// REPO_CONTRACT_INFRA (the toolchain died) and a downstream ship error must
// not be able to green this.
func TestC1673_001_LaneShipGateRunsCIEquivalentSuiteAndNamesShipError(t *testing.T) {
	err, scanLog := runProductionShipGate(t, untouchedRedFixture(t))
	if err == nil {
		t.Fatalf("RED: the lane ship gate PASSED a tree whose untouched package internal/deliverable is red — " +
			"a fixed-pack selection cannot see it, which is exactly how 4db205a8 landed a red main")
	}
	if code := gateCode(err); code != shiperr.CodeRepoContractGate {
		t.Fatalf("RED: the repo-contract gate did NOT block this tree — the ship ran on and failed later with %q. "+
			"A fixed-pack selection never reaches internal/deliverable, which is exactly how 4db205a8 landed a red main; "+
			"want the named %q. (err: %v)",
			code, shiperr.CodeRepoContractGate, err)
	}
	// The named code must carry WHICH test failed — ship-error.json is what the
	// operator and the failure adapter read.
	if !strings.Contains(err.Error(), "TestUntouchedPackageAssertsSupersededContract") {
		t.Errorf("RED: the %s message does not name the failing untouched-package test: %v",
			shiperr.CodeRepoContractGate, err)
	}
	// Side effect: the forensic scan log must record the untouched package's
	// red, not just the four historical guard suites.
	if !strings.Contains(scanLog, "internal/deliverable") {
		t.Errorf("RED: ship-repocontract-scan.log never mentions internal/deliverable — the scanner never reached it.\nscan log:\n%s", scanLog)
	}
}

// ---------------------------------------------------------------------------
// AC2 — the wave boundary reads main's CI and halts before any dispatch.
// ---------------------------------------------------------------------------

// TestC1673_002_WaveBoundaryHaltsOnRedMainBeforeAnyLaneDispatch — AC2. The
// bindings drive the real boundary with an injected completed-failing
// check-run and must prove BOTH halves: a named halt, and ZERO lanes
// launched. The anti-gaming half is the second one — a check-run helper that
// is called but is not a dispatch gate still permits red-main stacking
// (Scout's beyond-the-ask hypothesis 2, confidence 0.94). The consumption
// point is pinned function-scoped to prepareIteration, the ONE boundary
// before fleet configuration and dispatchFleetIteration can build a launcher.
func TestC1673_002_WaveBoundaryHaltsOnRedMainBeforeAnyLaneDispatch(t *testing.T) {
	assertSuiteTestsPass(t, cmdPkg,
		"TestMainCIAtWaveBoundary_RedCheckRunHalts",
		"TestMainCIAtWaveBoundary_RedHaltsBeforeAnyLaneDispatch",
	)

	root := acsassert.RepoRoot(t)
	window := filepath.Join(root, filepath.FromSlash(windowFile))

	red, err := acsassert.CountInGoFunc(window, "prepareIteration", mainCIRedStopReason)
	if err != nil {
		t.Fatalf("prepareIteration is not readable in %s (renamed or moved?): %v", windowFile, err)
	}
	if red == 0 {
		t.Errorf("RED: prepareIteration never sets the named red-main stop reason %q — "+
			"a halt emitted anywhere else is emitted after a launcher can already exist (%s:117-119)",
			mainCIRedStopReason, windowFile)
	}
	// NEGATIVE: the pre-existing divergence halt must survive as its OWN
	// reason. Renaming it to serve the new halt would satisfy the line above
	// while deleting a live guard.
	diverged, err := acsassert.CountInGoFunc(window, "prepareIteration", planeDivergedStopReason)
	if err != nil {
		t.Fatalf("prepareIteration unreadable: %v", err)
	}
	if diverged == 0 {
		t.Errorf("RED: prepareIteration no longer sets %q — the red-main halt is an ADDITION, not a rename of the divergence halt", planeDivergedStopReason)
	}

	// Caller proof: the check-run query must exist in PRODUCTION source. A
	// seam whose only caller is a test is dead code.
	if callers := productionFilesContaining(t, filepath.Join(root, "go", "cmd", "evolve"), "check-runs"); len(callers) == 0 {
		t.Errorf("RED: no production (non-test) file under go/cmd/evolve queries check-runs — nothing reads main's CI status")
	} else {
		t.Logf("production check-run query site(s): %s", strings.Join(callers, ", "))
	}

	// The signal registry and its projected doc must stay in lockstep: a new
	// named halt registered but never projected is drift the loop ships
	// silently. This is the repo's own `signals codes check` guard.
	assertSuiteTestsPass(t, cmdPkg, "TestSignalsCodes_RepoDocIsInSyncWithTheLinkedRegistry")
}

// productionFilesContaining returns the repo-relative non-test .go files under
// dir whose source contains needle.
func productionFilesContaining(t *testing.T, dir, needle string) []string {
	t.Helper()
	var hits []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(dir, name))
		if rerr == nil && strings.Contains(string(raw), needle) {
			hits = append(hits, name)
		}
	}
	return hits
}

// ---------------------------------------------------------------------------
// AC3 — a regression test pins the untouched-package case in-repo.
// ---------------------------------------------------------------------------

// TestC1673_003_InRepoRegressionPinsUntouchedPackageRed — AC3. 001 proves the
// behaviour once, from outside; AC3 asks for the pin to live IN the repo so a
// later cycle that reverts the selection to the fixed pack goes red in the
// ship package's own suite. Both named regressions must pass, and the two
// pre-existing backstop guards must still pass beside them — the new
// selection must not be bought by weakening the added-test backstop.
func TestC1673_003_InRepoRegressionPinsUntouchedPackageRed(t *testing.T) {
	assertSuiteTestsPass(t, shipPkg,
		"TestRepoContractGate_UsesRepoWideSuite",
		"TestRepoContractGate_RepoWideSuiteBlocksUntouchedPackageRegression",
	)
	// NEGATIVE / no-weakening: the guards this cycle must not trade away.
	assertSuiteTestsPass(t, shipPkg,
		"TestRepoContractGate_NewlyAddedFailingTestBlocksShip",
		"TestRepoContractGate_EnforceRedFailsWithDedicatedCode",
		"TestRepoContractGate_PersistentAmbiguityIsInfraClassedExactlyTwoRuns",
		"TestRepoContractGate_TransientFailureRetriesOnceThenShips",
	)

	// The pin must be TRACKED, not merely present on disk: a gitignored
	// regression file is dropped at ship (cycle-92/93).
	root := acsassert.RepoRoot(t)
	rel := "go/internal/phases/ship/repocontract_test.go"
	if !acsassert.FileExists(t, filepath.Join(root, rel)) {
		t.Fatalf("RED: %s missing on disk", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s is untracked — it would be dropped at ship", rel)
	}
}

// ---------------------------------------------------------------------------
// AC4 — green proceeds; unavailable is warned, never mistaken for green or red.
// ---------------------------------------------------------------------------

// TestC1673_004_GreenProceedsAndUnavailableIsWarnedNotRed — AC4. This is the
// false-RED direction, and it is the one a "halt whenever the query fails"
// implementation gets wrong: GitHub being unreachable is not evidence main is
// red, and must not stop the batch, nor may it be silently read as green. The
// bindings pin all three outcomes; the function-scoped pin here forbids the
// cheap collapse of routing the unavailable case into the red stop reason.
func TestC1673_004_GreenProceedsAndUnavailableIsWarnedNotRed(t *testing.T) {
	assertSuiteTestsPass(t, cmdPkg,
		"TestMainCIAtWaveBoundary_GreenProceeds",
		"TestMainCIAtWaveBoundary_UnavailableWarnsWithoutFalseRed",
	)

	root := acsassert.RepoRoot(t)
	window := filepath.Join(root, filepath.FromSlash(windowFile))
	// Exactly one consumption site for the red stop reason. Two would mean the
	// unavailable branch reuses it — the false-RED shape this AC forbids.
	red, err := acsassert.CountInGoFunc(window, "prepareIteration", mainCIRedStopReason)
	if err != nil {
		t.Fatalf("prepareIteration unreadable: %v", err)
	}
	if red > 1 {
		t.Errorf("RED: prepareIteration sets %q on %d lines — the unavailable/transport outcome must have its OWN named handling, never the red halt",
			mainCIRedStopReason, red)
	}
}

// ---------------------------------------------------------------------------
// AC5 — house floor: new-package graduation (ADR-0069's repo-wide apicover gate)
// ---------------------------------------------------------------------------

// TestC1673_005_AnyNewInternalPackageIsGraduated — AC5. If this change creates
// a new go/internal/<pkg> (a plausible home for the check-run client), BOTH
// halves must land in the SAME diff: the pattern appended to
// go/.apicover-enforce, and an apicover_named_test.go naming every export.
// Enrolled-but-unnamed fails the repo-wide gate; unenrolled aborts the build
// phase. Three lanes, one halt, same cause (cycle-1218). A change that adds no
// new package satisfies this vacuously and says so.
func TestC1673_005_AnyNewInternalPackageIsGraduated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	base := laneBase(t, root)
	added := gitC(t, root, "diff", "--diff-filter=A", "--name-only", base, "--", "go/internal")

	pkgs := map[string]bool{}
	for _, path := range strings.Fields(added) {
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(path)) // go/internal/<pkg>[/...]
		// Only a directory with NO pre-existing tracked .go file is new.
		if out, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-tree", "--name-only", base+":"+dir); code == 0 && strings.TrimSpace(out) != "" {
			continue
		}
		pkgs[dir] = true
	}
	if len(pkgs) == 0 {
		t.Log("no new go/internal package on this lane — graduation obligation is vacuous")
		return
	}

	enrollRaw, err := os.ReadFile(filepath.Join(root, "go", ".apicover-enforce"))
	if err != nil {
		t.Fatalf("read go/.apicover-enforce: %v", err)
	}
	enroll := string(enrollRaw)
	for dir := range pkgs {
		pattern := "./" + strings.TrimPrefix(dir, "go/")
		t.Logf("new package on this lane: %s (pattern %s)", dir, pattern)
		if !acsassert.LineContainsAll(filepath.Join(root, "go", ".apicover-enforce"), pattern) {
			t.Errorf("RED: new package %s is NOT enrolled in go/.apicover-enforce (want a line %q) — an unenrolled new package aborts the build phase.\nenrolled list head:\n%.400s", dir, pattern, enroll)
		}
		named := filepath.Join(root, filepath.FromSlash(dir), "apicover_named_test.go")
		if !acsassert.FileExists(t, named) {
			t.Errorf("RED: new package %s has no apicover_named_test.go — enrolled-but-unnamed fails the repo-wide gate as hard as unenrolled", dir)
		}
	}
}

// laneBase is the commit this lane's worktree forked from main at (falls back
// to HEAD when main is unreachable, which makes the diff checks trivially
// green — logged, never silent).
func laneBase(t *testing.T, root string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "merge-base", "main", "HEAD").Output()
	base := strings.TrimSpace(string(out))
	if err != nil || base == "" {
		t.Logf("merge-base main HEAD unavailable (%v); comparing against HEAD", err)
		return "HEAD"
	}
	return base
}
