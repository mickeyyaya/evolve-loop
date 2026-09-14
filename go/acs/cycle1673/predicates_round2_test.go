//go:build acs

// predicates_round2_test.go — cycle-1673 AUDIT-REPAIR predicates (round 2).
//
// Round 1 shipped five green predicates (predicates_test.go) and the audit
// still returned FAIL. The reason is the whole point of this file: every
// round-1 fixture passed ONE directory as both `ProjectRoot` and `Worktree`,
// so the gate's *root* argument was structurally unfalsifiable. The selection
// was widened to `./...` and proven; the tree it is pointed at was never
// asserted on.
//
// Audit round 2 findings encoded here (audit-report.md ## Issues):
//
//	H1 (HIGH)   the widened repo-contract gate scans the project root, not the
//	            lane worktree. ship.go:188 hands runRepoContractGate
//	            `req.ProjectRoot`; cyclerun_dispatch.go:138,140 populates
//	            ProjectRoot and Worktree as two DIFFERENT values for a fleet
//	            lane. Live proof: ship-repocontract-scan.log:1 reads
//	            `(module …/runtime/go)` while active_worktree is
//	            `.evolve/worktrees/cycle-cd3ae73e-1673`. The gate therefore
//	            tests the tree the lane merges INTO, never the tree it merges
//	            FROM — so cycles 1657/1659 reproduce unchanged, now at the
//	            cost of a full repo suite per ship.        → 006, 007, 008, 009
//	M1 (MEDIUM) build-report.md and repocontract.go:14,:82,:416 assert the pack
//	            runs "in the lane worktree". Closing H1 is what makes that
//	            prose true; the narrative half is a manual+checklist item
//	            addressed to the Auditor (test-report.md ## AC-Materialization).
//	M2 (MEDIUM) the staged explanation document was a different revision from
//	            the sealed one (index ce3c834e… vs seal ec743949…).      → 010
//
// AC map (1:1 with test-report.md ## AC-Materialization, continuing AC1–AC5):
//
//	AC6  the gate scans the LANE WORKTREE: a red that exists only in the
//	     worktree blocks the ship with the named code                    → 006
//	AC7  (negative) a red that exists only in the project root does NOT
//	     block the lane — a pre-merge main red is the wave boundary's job
//	     (AC2), never a false RED charged to this lane                   → 007
//	AC8  (edge) an EMPTY Worktree still falls back to ProjectRoot, and the
//	     routing is pinned by an in-repo regression test                 → 008
//	AC9  the added-test backstop reads the same lane tree — it discovers
//	     the worktree's STAGED added tests, not the project root's       → 009
//	AC10 the explanation document staged in the index is byte-identical to
//	     the sealed working-tree copy                                    → 010
//
// Adversarial axes (skills/adversarial-testing §6). NEGATIVE: 007 is the
// false-RED direction and is the reason 006 cannot be satisfied by scanning
// BOTH trees — a both-trees "fix" passes 006 and fails 007, and only
// preferring the worktree passes both. EDGE/OOD: 008 drives the empty-worktree
// sequential path (a fix that unconditionally uses req.Worktree would scan
// "/go" and silently stop gating); 009 drives a staged ADDITION rather than a
// modification, the one input class the backstop — not the repo-wide pack —
// owns. SEMANTIC: worktree routing (006), no-false-RED (007), fallback (008),
// backstop routing (009), artifact identity (010) — five distinct behaviours.
//
// Why no predicate can be greened by a magic string: 006/007/008/009 each
// construct two SEPARATE hermetic git trees and drive the production
// `ship.New → (*Phase).Run → runRepoContractGate` chain across them. The only
// way to change the verdict is to change which tree the gate executes in.
//
// Flaky-shape contract: no `go test ./...` in this package's own argv (the one
// broad compile is the PRODUCTION gate's, inside a two-package hermetic module
// in t.TempDir()); every in-repo binding is -run narrowed to ONE named
// package; no wall-clock bounds; no literal PIDs; every git call is -C
// anchored (gitC); no unbounded load generators.
//
// Reachability probe (cycle-644 rule): this file pins only ALREADY-EXISTING
// exported API (ship.New, ship.Config, (*ship.Phase).Run, core.PhaseRequest,
// core.AsShipError, shiperr.CodeRepoContractGate/Infra) plus the unexported
// helpers of this same test package. No new production import edge is
// committed, so the cycle-644 unsatisfiable-pin shape cannot arise. The
// remediation H1 asks for (thread req.Worktree into runRepoContractGate) is
// intra-package in internal/phases/ship — zero new imports, no cycle.
package cycle1673

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// supersededContractTest is the name of the fixture's untouched-package test —
// internal/deliverable's ADR-0100 declared-effects e2e, the exact test that
// went red on main from 4db205a8 until hotfix #590.
const supersededContractTest = "TestUntouchedPackageAssertsSupersededContract"

// addedRedGuardTest is the fixture's newly ADDED (never tracked before) test —
// the input class only the added-test backstop selects.
const addedRedGuardTest = "TestAddedRedGuard"

// ---------------------------------------------------------------------------
// Two-tree harness
// ---------------------------------------------------------------------------

func contractSrc(verdict string) string {
	return "package contractpkg\n\n// Verdict is the cross-package contract value.\nfunc Verdict() string { return \"" + verdict + "\" }\n"
}

// deliverableTestSrc is the UNTOUCHED package's already-committed test. It
// asserts the SUPERSEDED contract ("old"), so any tree whose contractpkg says
// "new" is repo-wide RED — and a package-scoped or fixed-pack selection that
// never reaches internal/deliverable cannot see it.
const deliverableTestSrc = "package deliverable\n\nimport (\n\t\"testing\"\n\n\t\"fixture/internal/contractpkg\"\n)\n\n" +
	"func " + supersededContractTest + "(t *testing.T) {\n" +
	"\tif got := contractpkg.Verdict(); got != \"old\" {\n" +
	"\t\tt.Fatalf(\"declared-effects contract changed under an untouched package: %q\", got)\n\t}\n}\n"

// laneTree builds one hermetic, self-contained git project root in the shape of
// the cycle-1657/1659 incident. committed is the contract value baked into the
// baseline commit; staged, when non-empty, is a tracked MODIFICATION left in
// the index — the way a lane stages its work before the pre-ship gate runs.
//
// Repo-wide GREEN iff the tree's effective contract value is "old".
func laneTree(t *testing.T, committed, staged string) string {
	t.Helper()
	root := t.TempDir()

	mustWrite(t, root, "go/go.mod", "module fixture\n\ngo 1.23\n")
	mustWrite(t, root, "go/internal/contractpkg/contract.go", contractSrc(committed))
	mustWrite(t, root, "go/internal/deliverable/deliverable.go",
		"package deliverable\n\n// Kind names the deliverable.\nfunc Kind() string { return \"code\" }\n")
	mustWrite(t, root, "go/internal/deliverable/declared_effects_e2e_test.go", deliverableTestSrc)

	gitC(t, root, "init", "-q", "-b", "main")
	gitC(t, root, "config", "user.email", "acs@example.invalid")
	gitC(t, root, "config", "user.name", "acs cycle1673")
	gitC(t, root, "add", "-A")
	gitC(t, root, "commit", "-q", "-m", "fixture: baseline (the untouched package's test is TRACKED, not added)")

	if staged != "" {
		mustWrite(t, root, "go/internal/contractpkg/contract.go", contractSrc(staged))
		gitC(t, root, "add", "-A")
	}
	return root
}

// stageAddedRedTest adds a NEW package carrying a NEW failing test and stages
// it. `git diff --cached --diff-filter=A` — the added-test backstop's only
// input — sees exactly this, and only in the tree it is pointed at.
func stageAddedRedTest(t *testing.T, root string) {
	t.Helper()
	mustWrite(t, root, "go/internal/newpkg/newpkg.go",
		"package newpkg\n\n// Name is this package's only symbol.\nfunc Name() string { return \"newpkg\" }\n")
	mustWrite(t, root, "go/internal/newpkg/added_red_test.go",
		"package newpkg\n\nimport \"testing\"\n\nfunc "+addedRedGuardTest+"(t *testing.T) {\n"+
			"\tt.Fatalf(\"newly added test is RED in %s\", Name())\n}\n")
	gitC(t, root, "add", "-A")
}

// runShipGateAcrossRoots drives the PRODUCTION ship phase — ship.New +
// (*Phase).Run, the construction the orchestrator and `evolve phase ship` both
// use — with ProjectRoot and Worktree set to DIFFERENT trees, exactly as
// cyclerun_dispatch.go:138,140 populates them for a fleet lane. Returns the
// phase error and the gate's own scan-log side effect.
//
// This is the wiring proof (house rule 2): runRepoContractGate is unexported
// and reachable only through Run, and the ProjectRoot/Worktree split is the
// production condition round 1's single-directory fixture could not express.
func runShipGateAcrossRoots(t *testing.T, projectRoot, worktree string) (err error, scanLog string) {
	t.Helper()
	workspace := t.TempDir()
	phase := ship.New(ship.Config{
		Runner:           sysexec.DefaultRunner,
		RepoContractGate: "enforce",
	})
	_, err = phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       1673,
		ProjectRoot: projectRoot,
		Workspace:   workspace,
		Worktree:    worktree,
		Context:     map[string]string{"commit_message": "acs cycle1673 round-2 fixture ship"},
		Env:         map[string]string{"EVOLVE_PROJECT_ROOT": projectRoot, "EVOLVE_PLUGIN_ROOT": projectRoot},
	})
	if raw, rerr := os.ReadFile(filepath.Join(workspace, "ship-repocontract-scan.log")); rerr == nil {
		scanLog = string(raw)
	}
	return err, scanLog
}

// moduleDirOf is the module path the gate prints in its scan-log header
// (`… (module %s)`), for the tree at root.
func moduleDirOf(root string) string { return filepath.Join(root, "go") }

// ---------------------------------------------------------------------------
// AC6 — the gate scans the LANE WORKTREE (H1, the HIGH finding).
// ---------------------------------------------------------------------------

// TestC1673_006_RepoContractGateScansTheLaneWorktreeNotTheProjectRoot — AC6.
//
// The incident, reproduced at production wiring level: the lane's contract
// change lives in its WORKTREE (staged, as a lane stages it); the project root
// is pre-merge main, where the change does not exist and every test is green.
// Today the gate runs `go test ./...` in projectRoot/go, sees green, and lets
// the lane land — main goes red on the next commit. Only a gate rooted at the
// worktree reaches internal/deliverable's superseded-contract test.
//
// Un-gameable: the two trees are separate directories on disk and only one of
// them contains the regression. No comment, constant, or magic string can move
// the verdict; only the argument handed to the gate can.
func TestC1673_006_RepoContractGateScansTheLaneWorktreeNotTheProjectRoot(t *testing.T) {
	projectRoot := laneTree(t, "old", "") // pre-merge main: green, no lane change
	worktree := laneTree(t, "old", "new") // the lane: contract change staged, deliverable red

	err, scanLog := runShipGateAcrossRoots(t, projectRoot, worktree)
	if err == nil {
		t.Fatalf("RED (H1): the ship PASSED a lane whose worktree is repo-wide RED — the gate scanned %s "+
			"(pre-merge main) instead of the lane worktree %s. This is cycles 1657/1659 verbatim.",
			moduleDirOf(projectRoot), moduleDirOf(worktree))
	}
	if code := gateCode(err); code != shiperr.CodeRepoContractGate {
		t.Fatalf("RED (H1): the repo-contract gate did NOT block this lane — the ship ran on and failed later with %q. "+
			"The lane's red lives ONLY in its worktree (%s); the gate is pointed at %s. want %q. (err: %v)",
			code, moduleDirOf(worktree), moduleDirOf(projectRoot), shiperr.CodeRepoContractGate, err)
	}
	if !strings.Contains(err.Error(), supersededContractTest) {
		t.Errorf("RED: the %s message does not name the failing untouched-package test %s: %v",
			shiperr.CodeRepoContractGate, supersededContractTest, err)
	}

	// The forensic artifact must corroborate the routing, not just the verdict.
	// This is the assertion the audit made against production
	// (ship-repocontract-scan.log:1 named runtime/go, not the worktree).
	if want := "(module " + moduleDirOf(worktree) + ")"; !strings.Contains(scanLog, want) {
		t.Errorf("RED (H1): ship-repocontract-scan.log never names the lane worktree module dir %q — "+
			"the scanner ran somewhere else.\nscan log:\n%s", want, scanLog)
	}
	if unwanted := "(module " + moduleDirOf(projectRoot) + ")"; strings.Contains(scanLog, unwanted) {
		t.Errorf("RED (H1): ship-repocontract-scan.log names the PROJECT ROOT module dir %q — the gate must test "+
			"the tree the lane merges FROM, not the one it merges INTO (scanning both charges this lane for a "+
			"pre-merge main red; that is the wave boundary's job, AC2).\nscan log:\n%s", unwanted, scanLog)
	}
}

// ---------------------------------------------------------------------------
// AC7 — NEGATIVE: a project-root-only red must not be charged to this lane.
// ---------------------------------------------------------------------------

// TestC1673_007_ProjectRootOnlyRedDoesNotFalseBlockTheLane — AC7, the false-RED
// direction and the anti-no-op half of AC6.
//
// The lane's worktree is green; the project root (pre-merge main) is already
// red for a reason this lane did not cause. A gate rooted at the project root
// blocks — today's behaviour, and a defect: the lane is failed for someone
// else's regression, and the operator reads a lane FAIL where the truth is a
// red main (ADR-0072 evidence the wave boundary owns, AC2). A gate that
// "fixes" AC6 by scanning BOTH trees also fails here. Only preferring the
// worktree passes AC6 and AC7 together.
func TestC1673_007_ProjectRootOnlyRedDoesNotFalseBlockTheLane(t *testing.T) {
	projectRoot := laneTree(t, "new", "") // pre-merge main is ALREADY red
	worktree := laneTree(t, "old", "")    // the lane itself is green

	err, scanLog := runShipGateAcrossRoots(t, projectRoot, worktree)
	switch code := gateCode(err); code {
	case shiperr.CodeRepoContractGate:
		t.Fatalf("RED (H1, false-RED direction): the gate blocked a GREEN lane worktree (%s) because the "+
			"PROJECT ROOT (%s) is red. A pre-merge main red is not this lane's regression — it must halt the "+
			"wave (AC2), never fail the lane. (err: %v)", moduleDirOf(worktree), moduleDirOf(projectRoot), err)
	case shiperr.CodeRepoContractInfra:
		t.Fatalf("RED: the gate returned %q for a green lane worktree — an infra classification here is still a "+
			"block charged to this lane. (err: %v)", code, err)
	}
	// The ship proceeds past the gate and fails later on a fixture precondition
	// (no attested audit binding exists in a t.TempDir workspace). That is the
	// expected shape: what AC7 forbids is a REPO-CONTRACT block, not an error.
	if err != nil {
		t.Logf("gate passed; ship failed downstream as expected for a hermetic fixture: %v", err)
	}
	if want := "(module " + moduleDirOf(worktree) + ")"; !strings.Contains(scanLog, want) {
		t.Errorf("RED (H1): ship-repocontract-scan.log never names the lane worktree module dir %q — "+
			"the green verdict was reached by scanning the wrong tree, which would be right for the wrong "+
			"reason.\nscan log:\n%s", want, scanLog)
	}
}

// ---------------------------------------------------------------------------
// AC8 — EDGE: an empty Worktree falls back to ProjectRoot, pinned in-repo.
// ---------------------------------------------------------------------------

// TestC1673_008_EmptyWorktreeFallsBackToProjectRoot — AC8.
//
// Not every ship is a fleet lane: the sequential loop and `evolve phase ship`
// leave PhaseRequest.Worktree empty. A remediation that unconditionally swaps
// in req.Worktree would root the gate at "/go" and silently stop gating those
// ships — a worse regression than H1, and invisible because the gate would
// simply never find a red. The fallback half is pre-existing GREEN; the
// in-repo pins below are the RED half, so a later cycle cannot revert the
// routing without the ship package's own suite going red.
func TestC1673_008_EmptyWorktreeFallsBackToProjectRoot(t *testing.T) {
	projectRoot := laneTree(t, "new", "") // red, and it is the ONLY tree there is

	err, scanLog := runShipGateAcrossRoots(t, projectRoot, "")
	if code := gateCode(err); code != shiperr.CodeRepoContractGate {
		t.Fatalf("RED: with an EMPTY Worktree the gate must still scan the project root (%s) and block its red; "+
			"got %q instead. An unconditional worktree swap roots the gate at \"/go\" and stops gating every "+
			"non-fleet ship. (err: %v)", moduleDirOf(projectRoot), code, err)
	}
	if want := "(module " + moduleDirOf(projectRoot) + ")"; !strings.Contains(scanLog, want) {
		t.Errorf("RED: ship-repocontract-scan.log does not name the project-root module dir %q on the "+
			"empty-worktree path.\nscan log:\n%s", want, scanLog)
	}

	// In-repo regression pin (AC3's discipline applied to the ROOT, which is
	// what round 1 left unpinned): the routing must be asserted by the ship
	// package's own suite, so reverting it reds CI rather than only this cycle.
	// One name per branch of the root resolution, so no branch can be reverted
	// silently: worktree preferred, project-root fallback, and the false-RED
	// direction AC7 owns.
	assertSuiteTestsPass(t, shipPkg,
		"TestRepoContractGate_ScansLaneWorktreeNotProjectRoot",
		"TestRepoContractGate_FallsBackToProjectRootWhenWorktreeEmpty",
		"TestRepoContractGate_ProjectRootRedDoesNotBlockAGreenWorktree",
	)

	// NEGATIVE / no-weakening: the guards round 1 delivered must not be traded
	// away to buy the root fix.
	assertSuiteTestsPass(t, shipPkg,
		"TestRepoContractGate_UsesRepoWideSuite",
		"TestRepoContractGate_RepoWideSuiteBlocksUntouchedPackageRegression",
		"TestRepoContractGate_NewlyAddedFailingTestBlocksShip",
		"TestRepoContractGate_EnforceRedFailsWithDedicatedCode",
	)
}

// ---------------------------------------------------------------------------
// AC9 — the added-test backstop reads the same lane tree.
// ---------------------------------------------------------------------------

// TestC1673_009_AddedTestBackstopDiscoversTheWorktreesStagedAdditions — AC9.
//
// addedTestPackageGroups runs `git -C <root> diff --cached --diff-filter=A`
// (repocontract.go:287) against the SAME misrouted root as the scanner pack.
// For a fleet lane the project root's index is empty — the lane's staged files
// live in its worktree — so the backstop silently discovers nothing and its
// specific attribution is lost on every lane ship. One parameter fixes both
// halves; this predicate is what makes the second half falsifiable, since a
// remediation that changes only `moduleDir` passes AC6 and fails here.
func TestC1673_009_AddedTestBackstopDiscoversTheWorktreesStagedAdditions(t *testing.T) {
	projectRoot := laneTree(t, "old", "") // pre-merge main: green, empty index
	worktree := laneTree(t, "old", "")    // the lane, green except for…
	stageAddedRedTest(t, worktree)        // …a newly ADDED, staged, failing test

	err, scanLog := runShipGateAcrossRoots(t, projectRoot, worktree)
	if code := gateCode(err); code != shiperr.CodeRepoContractGate {
		t.Fatalf("RED (H1, backstop half): the lane staged a newly ADDED failing test (%s in ./internal/newpkg) "+
			"in its worktree %s and the ship was not blocked (%q). The backstop read the project root's index "+
			"(%s), which is empty for every fleet lane. (err: %v)",
			addedRedGuardTest, worktree, code, projectRoot, err)
	}
	if !strings.Contains(err.Error(), "added-test backstop") {
		t.Errorf("RED: the block came from the repo-wide pack, not the added-test backstop — the backstop's "+
			"specific attribution (contractRed's detail, repocontract.go:411) is what an operator needs to see "+
			"for a newly added red. err: %v", err)
	}
	if !strings.Contains(err.Error(), addedRedGuardTest) {
		t.Errorf("RED: the block does not name the added test %s: %v", addedRedGuardTest, err)
	}
	if !strings.Contains(scanLog, "added-test backstop: go test") || !strings.Contains(scanLog, "./internal/newpkg") {
		t.Errorf("RED: ship-repocontract-scan.log has no added-test backstop invocation naming ./internal/newpkg — "+
			"the backstop never discovered the worktree's staged addition.\nscan log:\n%s", scanLog)
	}
}

// ---------------------------------------------------------------------------
// AC10 — M2: the staged explanation document is the sealed one.
// ---------------------------------------------------------------------------

// TestC1673_010_StagedExplanationDocumentIsTheSealedOne — AC10 (audit M2).
//
// At audit round 2 the document was `AM`: the index held a pre-reset revision
// (ce3c834e…, `Base SHA: 292cc033…`) while the working tree held the sealed
// one (ec743949…, `Base SHA: c6bb682c…`). Ship's `git add -A` happens to
// re-stage the right bytes, so the shipped artifact was salvaged by accident —
// but any reader of the index (a reviewer, a `git show :path`, an audit
// re-verification) sees a document describing a different base. This pins the
// invariant rather than the accident.
//
// Behavioural: it shells git against the real repo and compares real bytes;
// there is no string an implementer can add to satisfy it.
func TestC1673_010_StagedExplanationDocumentIsTheSealedOne(t *testing.T) {
	root := acsassert.RepoRoot(t)

	pattern := filepath.Join(root, "docs", "explain", "builds", "cycle-1673-*.md")
	docs, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %s: %v", pattern, err)
	}
	if len(docs) == 0 {
		t.Fatalf("RED: no cycle-1673 explanation document under docs/explain/builds/ — "+
			"the handoff marks explanation_status=required (glob %s)", pattern)
	}

	base := laneBase(t, root)
	for _, doc := range docs {
		rel, relErr := filepath.Rel(root, doc)
		if relErr != nil {
			t.Fatalf("rel %s: %v", doc, relErr)
		}
		rel = filepath.ToSlash(rel)

		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
			t.Errorf("RED: %s is neither tracked nor staged — an unstaged explanation document is dropped at ship", rel)
			continue
		}

		staged, _, code, sErr := acsassert.SubprocessOutput("git", "-C", root, "show", ":"+rel)
		if code != 0 {
			t.Errorf("RED: `git show :%s` failed (code=%d, %v) — the document is not in the index", rel, code, sErr)
			continue
		}
		onDisk, readErr := os.ReadFile(doc)
		if readErr != nil {
			t.Fatalf("read %s: %v", doc, readErr)
		}
		if staged != string(onDisk) {
			t.Errorf("RED (M2): the INDEX copy of %s is a different revision from the sealed working-tree copy "+
				"(%d staged bytes vs %d on disk) — the audit read `AM` here and the staged copy still carried a "+
				"stale Base SHA. Re-stage the sealed document.", rel, len(staged), len(onDisk))
		}

		// The document's own binding must name the tree it explains. A stale
		// revision is exactly a document whose Base SHA is not the lane's base.
		if want := "Base SHA: " + base; base != "HEAD" && !strings.Contains(string(onDisk), want) {
			t.Errorf("RED (M2): %s does not carry %q — it describes a different base than this lane's "+
				"(the audit found the staged copy pinned to a superseded base).", rel, want)
		}
	}
}
