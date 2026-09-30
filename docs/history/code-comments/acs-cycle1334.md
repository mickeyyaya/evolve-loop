# Comment history: `acs/cycle1334`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1334/predicates_test.go:3` — above `package cycle1334`

```text
// Package cycle1334 materializes the cycle-1334 acceptance criteria for this
// fleet lane's sole assigned todo-id, pipeline-repair-consumption-ack-wiring
// (per R9.3 no predicates bind to any other lane's items).
//
// Incident recap: cycle-1332 wired the MANUAL ack branch (`evolve loop
// --reset --fingerprint <fp>` — cmd_loop_fingerprint_ack_test.go) but
// explicitly deferred the transactional-consumption branch, whose
// production caller did not exist as code. The sibling P1 inbox item
// (2026-08-05T09-40-00Z-recurrence-ack-for-consumed-p0.json) names that gap
// literally: a pipeline-defect P0 item's `consumed_by` narrative (or, before
// a narrative is written, its auto-filed `notes` field) already carries the
// failure fingerprint as free text, but nothing parses it back out to write
// .evolve/resolved-fingerprints.json — the operator has to hand-retype it
// into --reset --fingerprint, "exactly the toil-and-tamper-surface the
// sanctioned flows exist to avoid" (the item's own words).
//
// AC map (1:1, from scout-report.md Task 1 "recurrence-ack-consumption-
// wiring" + its "Acceptance Criteria Summary" + the P1 item's explicit
// wiring-proof ask):
//
//	AC1 core.ParseConsumptionFingerprint(text) extracts the fingerprint
//	    triplet from the unquoted `fingerprint X` shape a consumed_by
//	    narrative uses.
//	    → C1334_001.
//	AC2 Semantic: the SAME parser also extracts from the quoted
//	    `fingerprint "X"` shape the auto-filed notes field uses — a
//	    distinct text shape, not the same behavior restated.
//	    → C1334_002.
//	AC3 Negative: free text with no `fingerprint` token returns ok=false —
//	    fails closed, never invents a fingerprint.
//	    → C1334_003.
//	AC4 Negative/precision: a pipe-delimited substring with no literal
//	    `fingerprint` keyword must not false-match (anchors on the token,
//	    not "any triplet-shaped text").
//	    → C1334_004.
//	AC5 core.ConsumePipelineDefectFingerprint writes an ack-ledger record
//	    (via AppendResolvedFingerprint) atomically from a consumed_by
//	    narrative — the literal fix this cycle's task selects.
//	    → C1334_005.
//	AC6 Semantic: the SAME helper falls back to the notes field when
//	    consumed_by is empty (an item consumed before a narrative exists) —
//	    a second, distinct extraction path.
//	    → C1334_006.
//	AC7 Negative: neither field carries a fingerprint → error, and NO
//	    ledger record written (never silently swallows an undiagnosed P0).
//	    → C1334_007.
//	AC8 Rule B integration — the wiring-proof fixture named verbatim in the
//	    inbox item's `fix` field: 3x identical-fingerprint failure records
//	    + a resolved-fingerprints.json ack write via the new consumption
//	    helper → Rule B does NOT halt; the SAME fixture without the
//	    consumption call → Rule B DOES halt (ceiling unweakened).
//	    → C1334_008.
//	AC9 Caller proof: `evolve inbox ack-fingerprint <item-path>` — driven
//	    from the REAL production entrypoint (runInbox, cmd/evolve) —
//	    reaches ConsumePipelineDefectFingerprint and writes the ledger. A
//	    predicate that only calls the core helper directly proves nothing
//	    about the CLI surface actually being wired (house rule: a wiring
//	    proof is a reachability test).
//	    → C1334_009.
//	AC10 Semantic: the SAME CLI subcommand also falls back to notes,
//	    exercising AC6's path from the real entrypoint.
//	    → C1334_010.
//	AC11 Negative: a missing item path returns a nonzero exit and writes NO
//	    ledger file.
//	    → C1334_011.
//	AC12 Negative: a real item with no parseable fingerprint in either field
//	    returns a nonzero exit and writes NO ledger file.
//	    → C1334_012.
//
// "Both: existing --reset --fingerprint manual path remains byte-identical"
// (scout-report.md Acceptance Criteria Summary) is covered by the EXISTING
// cycle-1332 regression tests (TestEvaluateBlockerBreaker_*,
// TestAppendResolvedFingerprint_WritesRecord,
// TestRunLoop_FingerprintAck_AppendsLedgerRecord) — this cycle adds a NEW
// writer path beside them without touching their code paths, so no new
// predicate is needed; disposition = pre-existing GREEN, verified
// unmodified (see test-report.md Coverage Map).
//
// Adversarial axes: negative (AC3/AC4/AC7/AC11/AC12), edge (AC3 empty
// match, AC4 near-miss text), semantic (AC1 vs AC2 two distinct text
// shapes; AC5 vs AC6 two distinct extraction paths; AC9/AC10 mirror AC5/AC6
// from the real CLI entrypoint — not one behavior restated four times). No
// source-grep predicates (cycle-85 rule): every predicate here runs the
// system under test as a subprocess (named unit test) or the SSOT eval
// quality checker — none asserts on source-file text alone.
```
