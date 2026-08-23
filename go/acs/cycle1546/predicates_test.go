//go:build acs

// Package cycle1546 materialises the cycle-1546 acceptance criteria for the one
// task triage committed to this lane:
//
//   - continuation-create-reuse-snapshot-base-guard  (ADR-0076 slice C, architect finding #3)
//
// The other two ids in the lane scope (lost-ship-closeout-universal-landing-witness,
// lost-ship-dossier-evidence) are `## deferred` in triage-report.md and therefore
// carry ZERO predicates here — R9.3: predicates bind only to triage-committed work,
// and a predicate gating deferred work starves the committed task (cycle-280).
//
// The defect. Slice C changed preserved-worktree state from DIRTY to COMMITTED: a
// FAILed cycle's work is now snapshot-committed onto the cycle branch with the
// subject `salvage snapshot (ADR-0076 continuation-on-fail)`. The adoption path
// (CreateFrom) handles that shape. The REUSE path does not: when gitWorktree.Create
// reuses an existing worktree for the same cycle number (resume/reset edge), the
// tree is now CLEAN — so ensureCleanWorktree no-ops — and cyclerun's base capture
// (`rev-parse HEAD`, cyclerun.go:497) records the SALVAGE SNAPSHOT as
// CycleState.WorktreeBaseSHA. normalizeWorktreeToBase then soft-resets to that very
// snapshot, so the salvaged work sits AT the base, produces an empty review diff,
// and is never re-exposed for audit. Silent, and the failure looks like "the builder
// did nothing".
//
// Predicate strategy — every predicate exercises the SYSTEM, never a source grep of
// production code (the cycle-85 degenerate-predicate ban). The seams under test
// (gitWorktree.Create, ensureCleanWorktree, the runCycle base capture,
// normalizeWorktreeToBase) are all UNEXPORTED, so they are unreachable from a leaf
// acs package. Each predicate therefore drives them through the sanctioned
// behavioural-via-subprocess shape (the cycle-987/997/1532 precedent): a
// `-run`-narrowed, single-named-package `go test -v` that must print
// `--- PASS: <name>` for every binding test Builder authors. Asserting on the PASS
// LINE — never on exit 0 — is load-bearing: `go test -run` on a pattern that matches
// NO test exits 0 with "no tests to run", so a still-missing binding test would
// false-GREEN.
//
// Every invocation is `-run`-narrowed against ONE named package, never a `/...`
// sweep and never the bare 40s+ ./internal/core suite, so a concurrent lane's
// contamination in an untouched package can never red this cycle
// (flaky-predicate-shape / scope-lint contract).
package cycle1546

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// corePkg owns every binding test for this lane — triage's committed file set is
// go/internal/core/worktree.go + go/internal/core/cyclerun.go, so `core` is the
// only package in this cycle's touched scope.
const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

// assertDefaultSuiteTestsPass shells `go test -run '^(names)$' -count=1 -v pkg` in
// the DEFAULT build suite (no -tags) and requires EVERY name to print a
// `--- PASS: <name>` line. -count=1 defeats the test cache, so a stale cached result
// from an earlier phase can never stand in for a live run.
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

// TestC1546_001_SalvageSnapshotHEADNeverBecomesTheNormalizeBase — AC1, the core
// claim. On a Create REUSE whose HEAD is a salvage-snapshot commit, the recorded
// worktree base must be the snapshot's first NON-salvage ancestor, not the snapshot.
// Two binding tests, because the base value alone is only half the claim:
//
//   - ...ResolvesToFirstNonSalvageAncestor — builds a real git worktree whose HEAD is
//     a salvage snapshot over an ordinary base commit, provisions through the reuse
//     path, and asserts the recorded base == the ORDINARY commit. Asserting
//     `!= snapshotSHA` alone would pass on any garbage value; the test must pin the
//     exact expected ancestor SHA.
//   - ...SalvagedWorkIsPendingInTheReviewDiffAfterNormalize — the CONSEQUENCE, and
//     the reason the AC exists at all. After normalizeWorktreeToBase runs against the
//     recorded base, the file the salvage snapshot carried must appear as a PENDING
//     change (the audit's `git diff HEAD` surface). A fix that corrects the stored
//     SHA but leaves the salvaged work invisible to audit has fixed nothing.
func TestC1546_001_SalvageSnapshotHEADNeverBecomesTheNormalizeBase(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_SalvageSnapshotHEADResolvesToFirstNonSalvageAncestor",
		"TestWorktreeReuseBase_SalvagedWorkIsPendingInTheReviewDiffAfterNormalize",
	)
}

// TestC1546_002_StackedSalvageSnapshotsResolvePastAllOfThem — AC2, the EDGE case.
// Two FAILed attempts on one preserved worktree stack TWO salvage snapshots (the
// snapshot commit is idempotent only for a CLEAN tree; a second dirty attempt
// commits again). A single-hop `HEAD^` implementation — the obvious and wrong
// shortcut — lands on the inner salvage snapshot and re-creates the exact defect one
// commit lower, with the outer attempt's work still invisible. The binding test must
// stack >= 2 salvage commits and assert the base is the ordinary commit BENEATH all
// of them, and that BOTH attempts' files are pending afterwards.
func TestC1546_002_StackedSalvageSnapshotsResolvePastAllOfThem(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_StackedSalvageSnapshotsResolveToTheCommitBeneathAll",
	)
}

// TestC1546_003_OrdinaryReusedHEADIsRecordedUnCHANGED — AC3, the NEGATIVE, and the
// strongest anti-over-reach signal in this set. The guard is allowed to fire ONLY on
// a salvage-snapshot HEAD. A reused worktree sitting on an ordinary commit must
// record that commit VERBATIM as the base; walking back an ancestor there would
// re-expose already-reviewed history as this cycle's diff and corrupt every normal
// resume. Two binding tests:
//
//   - ...OrdinaryHEADIsRecordedVerbatim — plain reuse, base == HEAD, byte-for-byte.
//   - ...CommitMentioningSalvageInBodyIsNotTreatedAsASnapshot — the OOD input: a
//     legitimate commit whose MESSAGE BODY quotes the marker string (a fix commit
//     describing this very defect would). Classification must key on the snapshot's
//     own SUBJECT line, not a substring match anywhere in the message, or landing
//     this task's own commit could poison a later cycle's base.
func TestC1546_003_OrdinaryReusedHEADIsRecordedUnchanged(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_OrdinaryHEADIsRecordedVerbatim",
		"TestWorktreeReuseBase_CommitMentioningSalvageInBodyIsNotTreatedAsASnapshot",
	)
}

// TestC1546_004_UnresolvableAncestorDegradesLoudlyNeverSilently — AC4, the fail-loud
// half of the inbox spec ("...or WARN loudly instead of silently degrading
// WorktreeBaseSHA"). When no non-salvage ancestor exists — the whole history is
// salvage snapshots, e.g. the root commit is itself one — the guard cannot resolve a
// base. That is a legitimate degrade, but it must be ANNOUNCED: the binding test must
// capture the orchestrator's stderr and assert a WARN naming the snapshot, so an
// operator can tell "no salvage snapshot here" from "gave up and used the snapshot".
// A silent fallback is the defect wearing a fix's clothes, and is exactly what this
// predicate exists to reject.
func TestC1546_004_UnresolvableAncestorDegradesLoudlyNeverSilently(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_UnresolvableAncestorWarnsLoudlyAndDoesNotDegradeSilently",
	)
}

// TestC1546_005_GuardIsReachedFromTheProductionProvisioningPath — AC5, the WIRING
// PROOF. A seam whose only caller is a test is dead code, and the four predicates
// above would all pass on a guard helper that production never invokes — the failure
// mode #373 is named for. The binding test must drive the REAL provisioning path
// (the orchestrator's worktree.Create + base capture in cyclerun.go, not the helper
// directly) against a worktree whose HEAD is a salvage snapshot, and assert the
// resulting CycleState.WorktreeBaseSHA is the guarded value. Builder must name that
// production caller as `file:line` in build-report.md.
func TestC1546_005_GuardIsReachedFromTheProductionProvisioningPath(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_ProvisioningPathRecordsGuardedBaseInCycleState",
	)
}
