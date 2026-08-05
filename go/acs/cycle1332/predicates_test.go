//go:build acs

// Package cycle1332 materializes the cycle-1332 acceptance criteria for this
// fleet lane's sole assigned inbox item, pipeline-defect-pipeline-blocker
// (per R9.3 no predicates bind to any other lane's items).
//
// Incident: cycle-1329's identical-fingerprint pipeline-blocker halt
// (ship|unknown|76d0f4fca190) was diagnosed and its root cause fixed+merged
// by #415, and the P0 was consumed with a narrative TWICE — yet the SAME
// fingerprint re-tripped the boot breaker on every relaunch, because (a) the
// breaker has no memory of "this fingerprint was already diagnosed and
// consumed" (core.EvaluateBlockerBreaker / core.CollectBatchFailureDigests
// re-scan disk fresh every call) and (b) lastCycleNumber does not advance on
// a halted/FAILed cycle, so the offending digest stays inside the batch
// window forever. Sibling P1 item
// .evolve/inbox/2026-08-05T09-40-00Z-recurrence-ack-for-consumed-p0.json
// (filed live during the incident) specifies the fix literally: an ack
// ledger consulted by the breaker, with an operator-driven CLI path to write
// it (its "fix" field's second, --fingerprint, branch — the OR clause this
// cycle implements; scout deferred the transactional-inbox-consumption-write
// branch, whose production caller does not exist as code today).
//
// AC map (1:1, from scout-report.md Task 1 selected task +
// "Acceptance Criteria Summary" + the P1 item's explicit wiring-proof ask):
//
//	AC1 core.LoadResolvedFingerprints(evolveDir) parses the ack ledger
//	    (.evolve/resolved-fingerprints.json) and returns a set containing
//	    every recorded fingerprint.
//	    → C1332_001 runs TestLoadResolvedFingerprints_ReadsLedgerRecords as
//	      a subprocess and requires its verbose "--- PASS:" marker.
//	AC2 Edge: a missing ledger file returns an EMPTY set with NO error
//	    (fail-open, mirrors CollectBatchFailureDigests' own tolerance for a
//	    healthy batch that never wrote a digest).
//	    → C1332_002 same subprocess pattern over
//	      TestLoadResolvedFingerprints_MissingFileReturnsEmptyNoError.
//	AC3 core.EvaluateBlockerBreaker excludes an acked fingerprint from the
//	    identical-fingerprint (Rule B) count — the literal reproduction of
//	    this cycle's incident: 3x identical-fingerprint digests + one ack
//	    record for that fingerprint → Halt=false.
//	    → C1332_003 runs TestEvaluateBlockerBreaker_ExcludesAckedFingerprint
//	      (exact name from scout-report.md verifiableBy).
//	AC4 Negative/regression: the SAME 3x identical-fingerprint digests with
//	    NO ack for that fingerprint still halt — proves the ack is
//	    fingerprint-scoped, never a blanket disable of Rule B (the ADR-0072
//	    floor this breaker extends must not be weakened).
//	    → C1332_004 runs
//	      TestEvaluateBlockerBreaker_UnackedIdenticalFingerprintStillHalts.
//	AC5 An operator-driven write path appends a ledger record
//	    (fingerprint + resolved_at + resolved_by) atomically — the
//	    P0-response primitive downstream callers (CLI, and eventually inbox
//	    consumption) both need.
//	    → C1332_005 runs TestAppendResolvedFingerprint_WritesRecord.
//	AC6 Caller proof: `evolve loop --reset --fingerprint <fp>` — driven from
//	    the REAL production entrypoint (runLoop, cmd/evolve) — appends the
//	    ledger record. A predicate that only calls AppendResolvedFingerprint
//	    directly would prove nothing about the operator-facing flag actually
//	    being wired (house rule: a wiring proof is a reachability test).
//	    → C1332_006 runs TestRunLoop_FingerprintAck_AppendsLedgerRecord.
//	AC7 Wiring-proof fixture (P1 item's explicit ask, Beyond-the-Ask
//	    hypothesis 2): blockerBreakerHalt — the actual loop-boot call site —
//	    replaying this cycle's exact incident (3 identical-fingerprint
//	    digests on disk under the batch window) does NOT halt when the ack
//	    ledger carries a matching record, and DOES halt (unchanged ADR-0072
//	    behavior) when the ledger is absent/non-matching.
//	    → C1332_007 runs
//	      TestBlockerBreakerHalt_AckedFingerprintDoesNotReHalt.
//
// Adversarial axes: negative (AC4 — unacked identical fingerprint must still
// halt; AC2 — missing ledger must not error), edge (AC2 empty/absent file),
// semantic (AC1 read vs AC5 write vs AC3 exclude vs AC7 end-to-end wiring are
// four distinct behaviors, not one restated). No source-grep predicates
// (cycle-85 rule): every predicate here runs the system under test as a
// subprocess (named unit test) or the SSOT eval quality checker — none
// asserts on source-file text alone.
package cycle1332

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evalSlug = "blocker-breaker-fingerprint-ack"
	corePkg  = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	cmdPkg   = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

// requireNamedPass runs `go test -run <pattern> <pkg>` as a subprocess and
// requires the exact named test(s) to report a verbose PASS marker. This is
// the RED mechanism: today none of these test names exist in the target
// package, so `-run` matches nothing, stdout carries no "--- PASS: <name>"
// line, and the loop below fails every predicate — a clean, well-diagnosed
// RED rather than a bare compile error (matches the cycle1313 precedent for
// ACS predicates that pin not-yet-written unit tests by exact name).
func requireNamedPass(t *testing.T, pkg, pattern string, names []string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", pattern, pkg)
	if code != 0 || err != nil {
		t.Logf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, pkg, code, err, stdout, stderr)
	}
	for _, name := range names {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("test %s did not report PASS in %s (missing, renamed, or not implemented yet)", name, pkg)
		}
	}
}

// AC1: the ledger loader parses recorded fingerprints.
func TestC1332_001_load_resolved_fingerprints_reads_ledger(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestLoadResolvedFingerprints_ReadsLedgerRecords$",
		[]string{"TestLoadResolvedFingerprints_ReadsLedgerRecords"})
}

// AC2: a missing ledger file is a tolerated empty set, not an error.
func TestC1332_002_load_resolved_fingerprints_missing_file_is_empty(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestLoadResolvedFingerprints_MissingFileReturnsEmptyNoError$",
		[]string{"TestLoadResolvedFingerprints_MissingFileReturnsEmptyNoError"})
}

// AC3: the literal incident reproduction — acked fingerprint excluded from
// the identical-fingerprint count.
func TestC1332_003_evaluate_blocker_breaker_excludes_acked_fingerprint(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestEvaluateBlockerBreaker_ExcludesAckedFingerprint$",
		[]string{"TestEvaluateBlockerBreaker_ExcludesAckedFingerprint"})
}

// AC4 (negative): an unacked identical fingerprint must still halt — the ack
// is fingerprint-scoped, never a blanket Rule B disable.
func TestC1332_004_evaluate_blocker_breaker_unacked_still_halts(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestEvaluateBlockerBreaker_UnackedIdenticalFingerprintStillHalts$",
		[]string{"TestEvaluateBlockerBreaker_UnackedIdenticalFingerprintStillHalts"})
}

// AC5: the ledger writer appends a record.
func TestC1332_005_append_resolved_fingerprint_writes_record(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestAppendResolvedFingerprint_WritesRecord$",
		[]string{"TestAppendResolvedFingerprint_WritesRecord"})
}

// AC6: caller proof — the operator-facing --fingerprint flag, driven through
// the real runLoop entrypoint, reaches the ledger writer.
func TestC1332_006_run_loop_fingerprint_flag_appends_ledger(t *testing.T) {
	requireNamedPass(t, cmdPkg,
		"^TestRunLoop_FingerprintAck_AppendsLedgerRecord$",
		[]string{"TestRunLoop_FingerprintAck_AppendsLedgerRecord"})
}

// AC7: end-to-end wiring proof at the actual loop-boot call site
// (blockerBreakerHalt) — the P1 item's explicit fixture ask.
func TestC1332_007_blocker_breaker_halt_acked_fingerprint_does_not_rehalt(t *testing.T) {
	requireNamedPass(t, cmdPkg,
		"^TestBlockerBreakerHalt_AckedFingerprintDoesNotReHalt$",
		[]string{"TestBlockerBreakerHalt_AckedFingerprintDoesNotReHalt"})
}

// AC-support: the permanent eval entry (.evolve/evals/<slug>.md) exists and
// passes the SSOT quality checker with a non-empty, real-command evidence
// set (closes the vacuous-empty-eval hole — Step 6b of the TDD contract).
func TestC1332_008_eval_file_passes_quality_check(t *testing.T) {
	evalPath := filepath.Join(acsassert.RepoRoot(t), ".evolve", "evals", evalSlug+".md")
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: evalPath})
	if err != nil {
		t.Fatalf("eval quality-check %s: %v", evalPath, err)
	}
	if res.Overall != evalqualitycheck.LevelPass {
		for _, c := range res.Commands {
			if c.Level != evalqualitycheck.LevelPass {
				t.Errorf("eval command %q classified level %d: %s", c.Line, c.Level, c.Reason)
			}
		}
		t.Fatalf("eval %s overall level %d, want PASS(0)", evalPath, res.Overall)
	}
	if len(res.Commands) < 2 {
		t.Fatalf("eval %s has %d classifiable command(s), want >=2 (vacuous-empty-eval guard)", evalPath, len(res.Commands))
	}
}
