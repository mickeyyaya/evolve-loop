# Comment history: `acs/cycle1332`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1332/predicates_test.go:3` — above `package cycle1332`

```text
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
```

### `go/acs/cycle1332/predicates_test.go:128` — above `func TestC1332_003_evaluate_blocker_breaker_excludes_acked_fingerprint(t *testing.T) {`

```text
// AC3: the literal incident reproduction — acked fingerprint excluded from
// the identical-fingerprint count.
```
