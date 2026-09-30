# Comment history: `acs/cycle1335`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1335/predicates_test.go:3` — above `package cycle1335`

```text
// Package cycle1335 materializes the cycle-1335 acceptance criteria for this
// fleet lane's two assigned todo-ids, recurrence-ack-consumption-wiring and
// batch-window-fail-advance (per R9.3 no predicates bind to any other lane's
// items, and none to `## deferred`).
//
// FRAMING CORRECTION — read before Builder starts. Premise-challenge returned
// FAIL/BLOCK on the framing handed down from scout+triage, and fault
// localization then localized the defects that actually exist in the tree.
// The ACs below encode the CORRECTED framing. Two corrections are load-bearing
// and must not be re-derived:
//
//	(a) go/internal/core/reset.go is EXONERATED and must not be edited.
//	    Nothing in the tree rewinds lastCycleNumber — reset.go:322 ADVANCES it
//	    under the comment "number never reused", and every other writer
//	    assigns forward. The real Defect B is the OPPOSITE: aborted cycles
//	    never advance the counter at all.
//	(b) The `kind: "pipeline-defect"` discriminator must NOT be implemented.
//	    Enumerating every live inbox item, that value matches ZERO items; the
//	    incident's own P0 and the driving item are both kind:"pipeline-repair".
//	    A kind-gated implementation passes every fixture test and never fires
//	    in production — the unit-green/live-green trap that produced this
//	    incident. Gate on core.ParseConsumptionFingerprint returning ok.
//
// The live incident (P0 re-tripped the boot breaker on THREE relaunches after
// the fix merged, #415) is two compounding defects:
//
//	Defect A — the ack ledger has no automated writer.
//	  .evolve/resolved-fingerprints.json does not exist on the live tree,
//	  while the P0 naming the fingerprint sits consumed in
//	  .evolve/inbox/consumed/ with a consumed_by narrative that
//	  core.ParseConsumptionFingerprint parses correctly today. The extraction
//	  logic works; nothing non-interactive calls it.
//	Defect B — aborted cycles pin their digests inside the breaker window.
//	  Cycles 1326/1328/1329 all exited through abnormalEpilogue (phase:
//	  "aborted"), which writes the failure digest but never advances
//	  state.LastCycleNumber (every loopAbort returns from RunCycle before
//	  finalizeCycle). The breaker window is derived from that same COMPLETION
//	  counter, so all three digests were re-collected on every relaunch and
//	  Rule B tripped at i==0, before any cycle ran.
//
// AC map (1:1 with the corrected task set; see test-report.md
// "## AC-Materialization" for the disposition table):
//
//	B1  readBatchWindowFloor anchors the breaker window on
//	    max(LastCycleNumber, LastAllocatedCycleNumber) — the monotone
//	    allocation lease tracks cycles DISPATCHED, which is what a time
//	    boundary needs; LastCycleNumber tracks cycles COMPLETED.
//	    → C1335_001.
//	B2  Edge: a legacy state with lastAllocatedCycleNumber=0 falls back to
//	    the completion counter. A bare swap would zero the window and
//	    re-collect the entire runs/ history.
//	    → C1335_002.
//	B3  Regression: readLastCycleNumber keeps reporting the COMPLETION
//	    counter — unfinishedCycle's stuck-cycle detection depends on it. Two
//	    counters, two named readers.
//	    → C1335_003.
//	B4  Caller proof: the incident replayed through the REAL entrypoint
//	    (runLoop) with the live counter shape must NOT halt.
//	    → C1335_004.
//	B5  Negative: digests minted INSIDE the batch must still halt — the
//	    window fix must not weaken Rule B's sensitivity.
//	    → C1335_005.
//	A1  Caller proof / live-state self-heal: blockerBreakerHalt reconciles
//	    .evolve/inbox/consumed/ into the ledger before loading it, so an
//	    already-consumed P0 (the CURRENT live state) stops re-halting — and
//	    the ledger is MATERIALIZED, making it a projection of the consumed
//	    corpus rather than a hand-maintained store (ADR-0047).
//	    → C1335_006.
//	A2  Semantic/anti-drift: an item with NO kind field whose fingerprint
//	    lives in `notes` is reconciled too — the gate is parse-success, not
//	    a `kind` vocabulary that can drift (correction (b) above).
//	    → C1335_007.
//	A3  Negative: a consumed item acking a DIFFERENT fingerprint leaves the
//	    halting one untouched — per-fingerprint, never a blanket disable.
//	    → C1335_008.
//	A4  Negative/robustness: a corrupt file in consumed/ WARNs by name and
//	    the sweep continues. A reconciler defect must never become a new
//	    boot blocker — the exact failure class this cycle removes.
//	    → C1335_009.
//	A5  Edge: no consumed/ directory ⇒ reconcile to nothing, silently,
//	    pre-fix behavior byte-identical.
//	    → C1335_010.
//	A6  Caller proof: `evolve inbox consume <item-path>`, driven from the
//	    real runInbox entrypoint, moves the item to consumed/ AND acks its
//	    fingerprint in ONE transaction — no separate ack-fingerprint step.
//	    → C1335_011.
//	A7  Semantic: an item with no parseable fingerprint consumes normally
//	    and writes NO ledger record — non-defect items no-op naturally.
//	    → C1335_012.
//	A8  Negative: a missing item path exits non-zero, names the item (not
//	    subcommand usage), and writes no ledger.
//	    → C1335_013.
//	A9  Negative/edge: `consume` with no argument exits non-zero with usage
//	    naming the subcommand.
//	    → C1335_014.
//	A10/B6 Permanent regression entries: both eval files pass the SSOT
//	    quality checker with a non-empty, real-command evidence set.
//	    → C1335_015, C1335_016.
//
// Adversarial axes: negative (C1335_005/008/009/013/014), edge (C1335_002/010),
// semantic (the window derivation vs the ledger projection are two distinct
// subsystems; A1 vs A2 are two distinct extraction shapes; A6 vs A1 are two
// distinct consumption routes — not one behavior restated). No source-grep
// predicates (cycle-85 rule): every predicate runs the system under test as a
// subprocess named test, or the SSOT eval quality checker.
```

### `go/acs/cycle1335/predicates_test.go:182` — above `func TestC1335_004_run_loop_aborted_digests_outside_window(t *testing.T) {`

```text
// B4 (caller proof): the incident replayed through runLoop must not halt.
```
