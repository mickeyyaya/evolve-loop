//go:build acs

// Package cycle1547 materialises the cycle-1547 acceptance criteria for the one
// task triage committed to this lane:
//
//   - continuation-create-reuse-snapshot-base-guard  (ADR-0076 slice C, architect finding #3)
//
// The other lane-scope id, worktree-provisioning-retry, is `## deferred` in
// scout-report.md (all three named production call sites — core, swarm, and the
// operator CLI — already consume gitexec.AddWorktreeWithRetry) and therefore
// carries ZERO predicates here — R9.3: predicates bind only to triage-committed
// work, and a predicate gating deferred work starves the committed task
// (cycle-280).
//
// PRE-EXISTING GREEN. This exact defect and fix already landed at
// go/internal/core/worktree_base.go + worktree_base_test.go, shipped in
// commit e3c99d77 "fix(core): salvage the snapshot-base guard from the
// 1539-1546 absorbing-FAIL chain (#486)", which is already an ancestor of this
// cycle's worktree (`git log --oneline -1 -- go/internal/core/worktree_base.go`
// resolves to e3c99d77; see also the go/acs/cycle1546/predicates_test.go
// original for the same claim). Scout re-selected the slug from lane scope
// before that landing was reconciled against the inbox. Per this agent's Step 4
// RED-verification rule ("Unexpected pass: log as pre-existing GREEN, mark in
// handoff") these predicates are authored, run, and confirmed GREEN rather than
// invented as false RED — there is no production code left for Builder to
// write, and Builder's job this cycle is a no-op confirmation, not a fix.
//
// Predicate strategy — every predicate exercises the SYSTEM, never a source
// grep of production code (the cycle-85 degenerate-predicate ban). The seams
// under test (gitWorktree.Create, ensureCleanWorktree, the runCycle base
// capture, normalizeWorktreeToBase) are all UNEXPORTED, so they are
// unreachable from a leaf acs package. Each predicate therefore drives them
// through the sanctioned behavioural-via-subprocess shape (the
// cycle-987/997/1532/1546 precedent): a `-run`-narrowed, single-named-package
// `go test -v` that must print `--- PASS: <name>` for every binding test. Since
// the binding tests already exist and already pass, this predicate package
// re-confirms the live wiring rather than gating a pending fix — a regression
// here means the guard was silently removed or broken, not merely unbuilt.
//
// Every invocation is `-run`-narrowed against ONE named package, never a `/...`
// sweep and never the bare 40s+ ./internal/core suite, so a concurrent lane's
// contamination in an untouched package can never red this cycle
// (flaky-predicate-shape / scope-lint contract).
package cycle1547

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// corePkg owns every binding test for this lane — the committed task's fix
// lives in go/internal/core/worktree_base.go + cyclerun.go, so `core` is the
// only package in this cycle's touched scope.
const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

// assertDefaultSuiteTestsPass shells `go test -run '^(names)$' -count=1 -v pkg` in
// the DEFAULT build suite (no -tags) and requires EVERY name to print a
// `--- PASS: <name>` line. -count=1 defeats the test cache, so a stale cached
// result from an earlier phase can never stand in for a live run.
func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", "-v", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("binding test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag). exit=%d\ncombined go-test output:\n%s",
				name, pkg, code, out)
		}
	}
}

// TestC1547_001_SalvageSnapshotHEADNeverBecomesTheNormalizeBase — AC1, the core
// claim. On a Create REUSE whose HEAD is a salvage-snapshot commit, the
// recorded worktree base must be the snapshot's first NON-salvage ancestor, not
// the snapshot. Two binding tests, because the base value alone is only half
// the claim:
//
//   - ...ResolvesToFirstNonSalvageAncestor — a real git worktree whose HEAD is
//     a salvage snapshot over an ordinary base commit resolves to that ORDINARY
//     commit exactly, not merely `!= snapshotSHA` (which would pass on garbage).
//   - ...SalvagedWorkIsPendingInTheReviewDiffAfterNormalize — the CONSEQUENCE.
//     After normalizeWorktreeToBase runs against the recorded base, the file the
//     salvage snapshot carried must appear as a PENDING change (the audit's
//     `git diff HEAD` surface).
func TestC1547_001_SalvageSnapshotHEADNeverBecomesTheNormalizeBase(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_SalvageSnapshotHEADResolvesToFirstNonSalvageAncestor",
		"TestWorktreeReuseBase_SalvagedWorkIsPendingInTheReviewDiffAfterNormalize",
	)
}

// TestC1547_002_StackedSalvageSnapshotsResolvePastAllOfThem — AC2, the EDGE
// case. Two FAILed attempts on one preserved worktree stack TWO salvage
// snapshots. A single-hop `HEAD^` implementation lands on the inner salvage
// snapshot and re-creates the defect one commit lower. The binding test stacks
// >= 2 salvage commits and asserts the base is the ordinary commit BENEATH all
// of them.
func TestC1547_002_StackedSalvageSnapshotsResolvePastAllOfThem(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_StackedSalvageSnapshotsResolveToTheCommitBeneathAll",
	)
}

// TestC1547_003_OrdinaryReusedHEADIsRecordedUnchanged — AC3, the NEGATIVE, and
// the strongest anti-over-reach signal in this set. The guard is allowed to
// fire ONLY on a salvage-snapshot HEAD. Two binding tests:
//
//   - ...OrdinaryHEADIsRecordedVerbatim — plain reuse, base == HEAD.
//   - ...CommitMentioningSalvageInBodyIsNotTreatedAsASnapshot — a legitimate
//     commit whose MESSAGE BODY quotes the marker string must not classify as a
//     snapshot; classification keys on the SUBJECT line only.
func TestC1547_003_OrdinaryReusedHEADIsRecordedUnchanged(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_OrdinaryHEADIsRecordedVerbatim",
		"TestWorktreeReuseBase_CommitMentioningSalvageInBodyIsNotTreatedAsASnapshot",
	)
}

// TestC1547_004_UnresolvableAncestorDegradesLoudlyNeverSilently — AC4, the
// fail-loud half of the inbox spec. When no non-salvage ancestor exists, the
// guard cannot resolve a base; that degrade must be ANNOUNCED via a stderr WARN
// naming the snapshot, never silently falling back to the snapshot itself.
func TestC1547_004_UnresolvableAncestorDegradesLoudlyNeverSilently(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_UnresolvableAncestorWarnsLoudlyAndDoesNotDegradeSilently",
	)
}

// TestC1547_005_GuardIsReachedFromTheProductionProvisioningPath — AC5, the
// WIRING PROOF. A seam whose only caller is a test is dead code. The binding
// test drives the REAL provisioning path (the orchestrator's worktree.Create +
// base capture in cyclerun.go, not the helper directly) against a worktree
// whose HEAD is a salvage snapshot, and asserts the resulting
// CycleState.WorktreeBaseSHA is the guarded value.
func TestC1547_005_GuardIsReachedFromTheProductionProvisioningPath(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_ProvisioningPathRecordsGuardedBaseInCycleState",
	)
}
