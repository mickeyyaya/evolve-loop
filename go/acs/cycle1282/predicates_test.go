//go:build acs

// Package cycle1282 encodes the cycle-1282 acceptance criteria for
// `continuation-defect-ledger`. Cycle 1282 is the CONTINUATION of cycle 1279:
// the anti-laundering ledger landed there and its own audit rejected it with
// seven defects (.evolve/runs/cycle-1279/audit-report.md, D1–D7). These
// predicates grade the disposition of that defect list — which is, fittingly,
// exactly the property the mechanism exists to enforce.
//
// Every predicate drives the REAL production seam through the owning package's
// behavioral tests — `hooks.Classify` (the audit verdict path),
// `Orchestrator.writeDeterministicLearning` (the failure floor the production
// path calls at failure_learning.go:366/372), and `faillearn.WriteArtifacts`.
// Nothing here greps source for a magic string: a predicate that passed on a
// string would pass on dead code, which is the failure mode this cycle exists
// to close.
//
// Shape discipline (flaky-predicate lint): every subprocess is ONE named
// package with a narrowed `-run` (./internal/core is a known 40s+ suite and is
// never swept whole), `cmd.Dir` is set explicitly rather than inherited from a
// fleet lane's cwd, and the bound is a process-reaping timeout, not a
// performance assertion.
package cycle1282

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goTestTimeout bounds one narrowed single-package run. Reaping bound only:
// the runs below are sub-second, so no contention level makes this a false red.
const goTestTimeout = 4 * time.Minute

// runNamedTests executes `go test -count=1 -v -run <pattern> <pkg>` from the
// worktree's go/ directory. -count=1 defeats the test cache: a cached PASS from
// a prior attempt must never green a predicate for a tree that no longer holds
// the fix.
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

// requirePassing asserts the run succeeded AND that each named test actually
// EXECUTED and passed. The second half is load-bearing: `go test -run` against
// a pattern matching nothing exits 0, so an exit-code-only predicate would
// green on a tree where the test does not exist at all.
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

// tail caps reported output so a predicate failure stays readable in the cycle
// log without discarding the assertion messages go test prints last.
func tail(s string) string {
	const max = 4000
	if len(s) <= max {
		return s
	}
	return "…\n" + s[len(s)-max:]
}

// TestC1282_001_LedgerNeverShrinksAndIDsAreStable — D1 (CRITICAL). reconcile
// rebuilt `merged` from ancestor.Entries alone and truncate-wrote it, so an
// ordinary audit retry erased the entries emit had appended — the module's own
// declared-forbidden state (defect_ledger.go:34), reachable with no adversary.
// Position-derived ids ("d"+len+1) then re-bind an id to different defect text
// across a chain, so a disposition closes something other than what it claims.
func TestC1282_001_LedgerNeverShrinksAndIDsAreStable(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_(ContinuationRetryDoesNotEraseOwnEntries|LedgerIDsAreContentDerived)$",
		"TestClassify_ContinuationRetryDoesNotEraseOwnEntries",
		"TestClassify_LedgerIDsAreContentDerived",
	)
}

// TestC1282_002_MissingAncestorLedgerIsDiagnosed — D2 (HIGH). The ancestor
// ledger lives outside the workspace and the role guard matches Edit|Write
// only, so one Bash `rm` disarmed the whole gate with zero diagnostics. The
// negative half keeps the new diagnostic off the ordinary non-continuation
// path, where warning on every cycle would train operators to ignore it.
func TestC1282_002_MissingAncestorLedgerIsDiagnosed(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_(ContinuationWithNoAncestorLedgerIsDiagnosed|NonContinuationEmitsNoLedgerDiagnostic)$",
		"TestClassify_ContinuationWithNoAncestorLedgerIsDiagnosed",
		"TestClassify_NonContinuationEmitsNoLedgerDiagnostic",
	)
}

// TestC1282_003_ClosureEvidenceMustResolve — D3 (HIGH). Evidence was validated
// for non-emptiness after trim only, so `evidence:"x"` transitioned an
// inherited CRITICAL to FIXED with no diagnostic — the unverifiable closure
// claim F1 indicts. The positive half is the anti-overcorrection lock: a rule
// that rejected every claim would block every continuation forever.
func TestC1282_003_ClosureEvidenceMustResolve(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_(UnresolvableEvidenceDoesNotCloseADefect|ResolvableEvidenceClosesADefect)$",
		"TestClassify_UnresolvableEvidenceDoesNotCloseADefect",
		"TestClassify_ResolvableEvidenceClosesADefect",
	)
}

// TestC1282_004_EveryDispositionArmIsExercised — D4 (HIGH). The coverage gate
// FAILed at 76.2% with three of the five disposition arms — the acceptance
// criterion's headline rule — unexecuted, plus the non-OPEN carry-forward that
// is the literal multi-hop shape (1255→1268→1270→1272) being defended. The
// 1279 retention test rides along as the regression lock.
func TestC1282_004_EveryDispositionArmIsExercised(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_(DispositionArms|CarriesForwardAlreadyDispositionedAncestorEntry|ContinuationLedgerRetainsEveryEntry)$",
		"TestClassify_DispositionArms",
		"TestClassify_CarriesForwardAlreadyDispositionedAncestorEntry",
		"TestClassify_ContinuationLedgerRetainsEveryEntry",
	)
}

// TestC1282_005_EmitCoversWarnNotFailAlone — D6 (MEDIUM). emit was gated on
// verdict == FAIL while scout Task 1 and F1 specify FAIL/WARN, so a
// WARN-shipped cycle carrying structured defects left the next continuation
// nothing to inherit. The negative half stops the widening from minting a
// ledger on every warned cycle, which would make every later cycle look like a
// continuation and render the reconcile gate vacuous.
func TestC1282_005_EmitCoversWarnNotFailAlone(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_Warn(WithStructuredDefectsEmitsLedger|WithoutStructuredDefectsMintsNothing)$",
		"TestClassify_WarnWithStructuredDefectsEmitsLedger",
		"TestClassify_WarnWithoutStructuredDefectsMintsNothing",
	)
}

// TestC1282_006_DegenerateEchoIsNotFiledToInbox — D5 (MEDIUM).
// failure_learning.go:442-448 claims the synthesized summary echo is filtered,
// but the guard is `structured != nil` alone and ReadFailureBlock returns a
// block whenever Class != "" — so a classed-but-defectless failure filed the
// echo as a priority-H inbox bug. Narrowed `-run` against ./internal/core (a
// known 40s+ suite that must never be swept whole from a predicate).
//
// Calls runNamedTests DIRECTLY rather than through requirePassing: the
// flaky-shape lint follows ONE hop into a same-package helper, so the `-run`
// narrowing that makes this invocation safe must be visible at that depth for
// the known-slow-suite finding to resolve correctly.
func TestC1282_006_DegenerateEchoIsNotFiledToInbox(t *testing.T) {
	const pattern = "^TestWriteDeterministicLearning_(ClassedButDefectlessBlockFilesNothing|EchoDefectListFilesNothing|StructuredDefectsAreFiled)$"
	out, ok := runNamedTests(t, "./internal/core", pattern)
	if !ok {
		t.Errorf("./internal/core: `go test -run %s` failed:\n%s", pattern, tail(out))
		return
	}
	for _, n := range []string{
		"TestWriteDeterministicLearning_ClassedButDefectlessBlockFilesNothing",
		"TestWriteDeterministicLearning_EchoDefectListFilesNothing",
		"TestWriteDeterministicLearning_StructuredDefectsAreFiled",
	} {
		if !strings.Contains(out, "--- PASS: "+n) {
			t.Errorf("./internal/core: %s did not run to a PASS (a -run pattern that matches nothing also exits 0):\n%s", n, tail(out))
		}
	}
}

// TestC1282_007_InboxIDCannotEscapeTheInboxDirectory — D7 (LOW). WithInbox
// concatenated `it.ID + ".json"` into a path with no sanitisation on a NEWLY
// EXPORTED API; the current caller is safe, the next one is the one that falls
// in. Runs the whole faillearn package (one named package, sub-second) so the
// guard cannot regress the existing F1(ii) transactional-write contract.
func TestC1282_007_InboxIDCannotEscapeTheInboxDirectory(t *testing.T) {
	requirePassing(t,
		"./internal/faillearn",
		"^Test",
		"TestWriteArtifacts_InboxRejectsPathEscapingID",
		"TestWriteArtifacts_InboxAcceptsOrdinaryID",
		"TestWriteArtifacts_InboxItemsLandBesideRetrospective",
		"TestWriteArtifacts_WithoutInboxOptionIsUnchanged",
	)
}
