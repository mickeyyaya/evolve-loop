//go:build acs

// Package cycle1287 materialises the cycle-1287 acceptance criteria for the two
// fleet-scoped tasks pinned to this lane (inbox item `continuation-defect-ledger`):
//
//   - land-continuation-defect-ledger          → land the ledger/inbox/closure work
//     WITHOUT regressing the two main-side fixes the branch point predates
//   - batch-integrity-review-doc-closure-crossref → the governed docs must pass the
//     closure-citation gate they describe
//
// What is actually at risk. The ledger mechanism itself is already present and
// green in this worktree (predicates 003/004 pin that it stays so). The landing
// RISK is base drift: this lane forked at 9b129565, and `main` has since moved to
// a57e9ec4 carrying two fixes this tree does not have —
//
//	go/internal/phases/retro/retro.go       stale-worktree guard (gobridge.IsDir)
//	                                        + retro_stale_worktree_test.go
//	go/internal/router/router.go            MintSpec.Description/WhenToUse (ADR-0038)
//	                                        + mintspec_metadata_test.go
//
// A landing that takes this tree's version of those regions verbatim silently
// reintroduces the 1255-D1 stale-worktree CRITICAL and drops the ADR-0038 SELECT
// metadata wire contract, deleting both regression tests on the way. Predicates
// 001/002 are the RED that only a correctly-resolved sync can green.
//
// Predicate strategy — every predicate EXERCISES the system (runs the production
// regression test, round-trips the production type through encoding/json, builds
// the tree, or drives the production gate function); none asserts "source file
// contains string X" as its load-bearing check (the cycle-85 degenerate-predicate
// ban). Subprocess predicates are narrowed to ONE named package with an explicit
// -run expression, per the flaky-predicate-shape rules, and each guards against
// go test's exit-0-on-no-matching-test trap.
package cycle1287

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goTestRun runs ONE named package under an explicit -run expression built from
// the exact test names given, and requires EVERY named test to have executed and
// PASSED.
//
// Per-name accounting, not exit code alone. `go test -run TestThatDoesNotExist
// ./pkg` exits 0 with "testing: warning: no tests to run", and an alternation
// where only one of four names exists exits 0 with no warning at all — so an
// exit-code predicate greens on a landing that deleted the very regression tests
// it exists to protect. Asserting a `--- PASS: <name>` line per name closes both
// holes: a deleted test is as red as a failing one.
//
// `go -C <dir>` anchors the invocation to the worktree under test rather than the
// process working directory, which differs between the main tree, a worktree, and
// each fleet lane.
func goTestRun(t *testing.T, root, pkg string, names ...string) {
	t.Helper()
	anchored := make([]string, 0, len(names))
	for _, n := range names {
		anchored = append(anchored, "^"+n+"$")
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-v", "-run", strings.Join(anchored, "|"), pkg)
	combined := stdout + stderr
	for _, n := range names {
		if !strings.Contains(combined, "--- PASS: "+n) {
			t.Errorf("%s: %s did not run-and-pass — it is missing from this tree or failing, so the behaviour it pins is unprotected", pkg, n)
		}
	}
	if err != nil || code != 0 {
		t.Errorf("go test %s exited %d (err=%v)\n%s", pkg, code, err, combined)
	}
}

// TestC1287_001_RetroStaleWorktreeCriticalSurvivesTheLanding pins main's
// stale-worktree fix (commit 43a802d3, the 1255-D1 CRITICAL) through this
// landing. RED while this tree still carries the pre-fix `req.Worktree != "" ||
// !fleetMode(req)` guard and lacks retro_stale_worktree_test.go entirely.
//
// The assertion is the main-side regression test EXECUTING and passing here —
// not the presence of a string in retro.go — so neither re-adding the file
// without the fix nor re-adding the fix without the file can green it.
func TestC1287_001_RetroStaleWorktreeCriticalSurvivesTheLanding(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goTestRun(t, root, "./internal/phases/retro",
		"TestRetroWorktree_StaleNonExistentPathFallsBackToScratchCwd",
		"TestRetroWorktree_FleetNeverEmitsANonExistentPath",
		"TestRetroWorktree_FleetProvisionedWorktreePassesThroughVerbatim",
		"TestRetroWorktree_NonFleetStalePathPassesThroughVerbatim")
}

// TestC1287_002_MintSpecSelectMetadataSurvivesTheLanding pins main's ADR-0038
// SELECT metadata contract (commit 6c4e8068) through this landing: MintSpec must
// carry Description/WhenToUse on the `description` / `when_to_use` json keys, and
// omit them when empty.
//
// Reflection rather than a struct literal deliberately: a literal referencing an
// absent field fails to COMPILE, which takes the whole predicate package down and
// hides every other predicate's verdict. Reflection keeps the failure scoped to
// this criterion. The values are driven through the real encoding/json codec in
// both directions, so this is a wire-contract exercise, not a field-name check.
func TestC1287_002_MintSpecSelectMetadataSurvivesTheLanding(t *testing.T) {
	root := acsassert.RepoRoot(t)

	var spec router.MintSpec
	if err := json.Unmarshal([]byte(`{"prompt":"p","tier":"deep","description":"D","when_to_use":"W"}`), &spec); err != nil {
		t.Fatalf("unmarshal MintSpec: %v", err)
	}
	v := reflect.ValueOf(spec)
	for field, want := range map[string]string{"Description": "D", "WhenToUse": "W"} {
		f := v.FieldByName(field)
		if !f.IsValid() {
			t.Errorf("router.MintSpec has no field %s — ADR-0038 SELECT metadata was dropped by the landing (base drift, not a real change from this lane)", field)
			continue
		}
		if got := f.String(); got != want {
			t.Errorf("MintSpec.%s = %q after decoding the advisor wire form, want %q", field, got, want)
		}
	}

	// omitempty: an advisor that emits no metadata must mint exactly as before.
	bare, err := json.Marshal(router.MintSpec{Prompt: "p"})
	if err != nil {
		t.Fatalf("marshal bare MintSpec: %v", err)
	}
	for _, key := range []string{`"description"`, `"when_to_use"`} {
		if strings.Contains(string(bare), key) {
			t.Errorf("empty MintSpec marshalled %s (%s) — the metadata keys must be omitempty", key, bare)
		}
	}

	// The main-side regression tests must survive the merge too, not just the
	// struct fields they cover.
	goTestRun(t, root, "./internal/router",
		"TestMintSpec_CarriesSelectMetadata",
		"TestMintSpec_MetadataOmitEmpty")
}

// TestC1287_003_DefectLedgerReconcileIsLiveAtTheClassifySeam runs the six
// ledger/reconcile locks scout-report.md names, against the production
// hooks.Classify seam. Expected GREEN on arrival (the mechanism is already in
// this tree); it is here so a merge conflict resolved in main's favour cannot
// silently drop the thing this cycle exists to land.
func TestC1287_003_DefectLedgerReconcileIsLiveAtTheClassifySeam(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goTestRun(t, root, "./internal/phases/audit",
		"TestClassify_RejectingAuditEmitsDefectLedger",
		"TestClassify_ContinuationCannotPassWithUnaccountedDefect",
		"TestClassify_ContinuationWithNoDispositionArtifactCannotPass",
		"TestClassify_UnresolvableEvidenceDoesNotCloseADefect",
		"TestClassify_ContinuationLedgerRetainsEveryEntry",
		"TestClassify_PassingAuditWritesNoLedger")
}

// TestC1287_004_RetroRemediationReachesTheInboxTransactionally runs the two
// transactional-inbox locks (F1 clause 2): remediation items land beside the
// retrospective, and a failed inbox write leaves no retrospective behind.
// Expected GREEN on arrival, same rationale as 003.
func TestC1287_004_RetroRemediationReachesTheInboxTransactionally(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goTestRun(t, root, "./internal/faillearn",
		"TestWriteArtifacts_InboxItemsLandBesideRetrospective",
		"TestWriteArtifacts_InboxFailureLeavesNoRetrospective")
}

// TestC1287_005_TreeBuildsAfterTheDriftResolution compiles the whole module. The
// router hunk has an off-branch consumer (core/phase_advisor.go reads
// MintSpec.Description/WhenToUse), so a hunk resolved by deleting the fields
// breaks a package this cycle never touches. `go build` — not a `go test ./...`
// sweep, which the flaky-shape rules ban as a predicate.
func TestC1287_005_TreeBuildsAfterTheDriftResolution(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "build", "./...")
	if err != nil || code != 0 {
		t.Errorf("go build ./... exited %d (err=%v)\n%s%s", code, err, stdout, stderr)
	}
}

// TestC1287_006_GovernedDocsPassTheClosureCitationGate is the Task 2 crux: the
// two documents that narrate the closure-citation gate must SATISFY it. Driven
// through the production closureClaimOffenders via the in-package test (the gate
// helper is unexported, and exporting it just to test it would widen the API for
// the test's convenience). RED until the "Not closed here" section and the F1
// accounting prose are rewritten as citations.
func TestC1287_006_GovernedDocsPassTheClosureCitationGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goTestRun(t, root, "./internal/phases/audit", "TestC1287_DocsPassClosureCitationGate")
}

// TestC1287_007_ClosureGateStillRejectsUncitedClaims is the anti-gaming twin of
// 006. The cheapest way to green a docs gate is to weaken the gate, so the
// rejection behaviour is pinned independently: an uncited "verified closed" must
// still FAIL, a cited one must pass, and ordinary prose about a closed file
// handle must stay invisible. Expected GREEN on arrival and required to STAY
// green — 006 greening while 007 reds means the gate was neutered, not the docs
// fixed.
func TestC1287_007_ClosureGateStillRejectsUncitedClaims(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goTestRun(t, root, "./internal/phases/audit", "TestC1287_ClosureGateRejectsUncitedClaim")
}
