//go:build acs

// Package cycle1279 encodes the cycle-1279 acceptance criteria for
// `continuation-defect-ledger` (batch-integrity-review-2026-08-04.md F1 —
// defect laundering across salvage/continuation chains).
//
// Each predicate drives the REAL production seam through the package's own
// behavioral tests: `hooks.Classify` (the audit verdict path) for the ledger
// emit + disposition diff, and `faillearn.WriteArtifacts` (the function
// core.writeDeterministicLearning and cmd_loop_outcome already call) for the
// transactional inbox write. Nothing here greps source for a magic string — a
// predicate that passed on a string would pass on dead code, which is the
// exact failure mode this cycle exists to close.
//
// Shape discipline (flaky-predicate lint): every subprocess is ONE named
// package with a narrowed `-run`, `cmd.Dir` is set explicitly (never a bare
// cwd-relative git/go invocation, which resolves differently in a fleet lane),
// and the context bound comes from the test deadline rather than a literal
// wall-clock budget.
package cycle1279

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goTestTimeout bounds one narrowed single-package run. It is a process
// reaping bound, not a performance assertion: the runs below are sub-second,
// so no contention level makes this a false red.
const goTestTimeout = 4 * time.Minute

// runNamedTests executes `go test -v -count=1 -run <pattern> <pkg>` from the
// worktree's go/ directory and returns the combined output plus success.
//
// -count=1 defeats the test cache: a cached PASS from a prior attempt must
// never green a predicate for a tree that no longer contains the fix.
func runNamedTests(t *testing.T, pkg, pattern string) (string, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), goTestTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "-run", pattern, pkg)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go") // explicit: never the process cwd
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err == nil
}

// requirePassing asserts the package run succeeded AND that each named test
// actually EXECUTED and passed. The second half is load-bearing: `go test -run`
// against a pattern matching nothing exits 0, so an exit-code-only predicate
// would green on a tree where the test does not exist at all.
func requirePassing(t *testing.T, pkg, pattern string, names ...string) {
	t.Helper()
	out, ok := runNamedTests(t, pkg, pattern)
	if !ok {
		t.Errorf("%s: `go test -run %s` failed:\n%s", pkg, pattern, tail(out))
		return
	}
	for _, n := range names {
		if !strings.Contains(out, "--- PASS: "+n) {
			t.Errorf("%s: %s did not run to a PASS (a -run pattern that matches nothing also exits 0):\n%s", pkg, n, tail(out))
		}
	}
}

// tail caps the reported output so a predicate failure stays readable in the
// cycle log without discarding the actual assertion messages (which go test
// prints last).
func tail(s string) string {
	const max = 4000
	if len(s) <= max {
		return s
	}
	return "…\n" + s[len(s)-max:]
}

// TestC1279_001_RejectingAuditEmitsDefectLedger — F1(i) part one. A rejecting
// audit must persist an addressable, id-bearing defect ledger (status OPEN,
// verbatim defect text) via the real Classify path, and a clean PASS must NOT
// mint one. The negative half is what keeps the diff gate below from being
// satisfiable by minting an empty ledger on every cycle.
func TestC1279_001_RejectingAuditEmitsDefectLedger(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_(RejectingAuditEmitsDefectLedger|PassingAuditWritesNoLedger)$",
		"TestClassify_RejectingAuditEmitsDefectLedger",
		"TestClassify_PassingAuditWritesNoLedger",
	)
}

// TestC1279_002_ContinuationCannotLaunderADefect — F1(i) part two, the crux of
// the finding. A continuation lane that fixes two of three inherited defects,
// narrates PASS, and shows a green EGPS gate must NOT be able to PASS, and the
// unaccounted defect must be named by id. The no-disposition-artifact variant
// closes the cheapest bypass (emit nothing, hope the step degrades open).
func TestC1279_002_ContinuationCannotLaunderADefect(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_Continuation(CannotPassWithUnaccountedDefect|WithNoDispositionArtifactCannotPass)$",
		"TestClassify_ContinuationCannotPassWithUnaccountedDefect",
		"TestClassify_ContinuationWithNoDispositionArtifactCannotPass",
	)
}

// TestC1279_003_DispositionsAreVisibleAndEntriesSurvive — F1(i) part three.
// The disposition must live in the audit's OWN written-back artifact ("not
// just inferred from a diff a human must run"), every entry must survive as a
// status transition (never a deletion — a ledger that shrinks is a ledger that
// launders), a FIXED claim must carry evidence, and a DEFERRED claim a reason.
func TestC1279_003_DispositionsAreVisibleAndEntriesSurvive(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_ContinuationLedgerRetainsEveryEntry$",
		"TestClassify_ContinuationLedgerRetainsEveryEntry",
	)
}

// TestC1279_004_NonContinuationPassPathUnperturbed — the regression lock. Most
// cycles are not continuations; the new diff step must be a no-op for them.
// Without this, the fix for F1 is free to break every ordinary green cycle.
func TestC1279_004_NonContinuationPassPathUnperturbed(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_NonContinuationPassPathUnchanged$",
		"TestClassify_NonContinuationPassPathUnchanged",
	)
}

// TestC1279_005_RetroRemediationReachesTheInbox — F1(ii). Remediation items
// must be addressable FROM `.evolve/inbox` (with inboxbatch.Item wire-tag
// parity), not only from the retrospective body — the literal 1255 defect. The
// transactional half asserts the invariant directly: when the inbox write
// fails, no retrospective may be left behind claiming the remediation was
// recorded.
func TestC1279_005_RetroRemediationReachesTheInbox(t *testing.T) {
	requirePassing(t,
		"./internal/faillearn",
		"^TestWriteArtifacts_(InboxItemsLandBesideRetrospective|InboxFailureLeavesNoRetrospective)$",
		"TestWriteArtifacts_InboxItemsLandBesideRetrospective",
		"TestWriteArtifacts_InboxFailureLeavesNoRetrospective",
	)
}

// TestC1279_006_ExistingFaillearnCallersUnchanged — the back-compat lock for
// the three production WriteArtifacts call sites that pass no options
// (core/failure_learning.go, core/reset.go, cmd/evolve/cmd_loop_outcome.go),
// plus the zero-items edge. Runs the WHOLE faillearn package (one named
// package, sub-second) so the option refactor cannot regress any existing
// floor behavior — the byte-identical-artifact contract the package documents.
func TestC1279_006_ExistingFaillearnCallersUnchanged(t *testing.T) {
	requirePassing(t,
		"./internal/faillearn",
		"^Test",
		"TestWriteArtifacts_WithoutInboxOptionIsUnchanged",
		"TestWriteArtifacts_EmptyInboxItemsMintsNoFiles",
	)
}
