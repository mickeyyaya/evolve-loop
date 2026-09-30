# Comment history: `acs/cycle1180`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1180/predicates_test.go:3` — above `package cycle1180`

```text
// Package cycle1180 materialises the cycle-1180 acceptance criteria for the two
// triage-COMMITTED (## top_n) fleet-scoped tasks pinned to this lane:
//
//   - wave-lane-task-quarantine-dead   → a FAILED WAVE LANE must move its
//     triage-committed ids through the ADR-0072 S5 failure lifecycle
//     (failure_count bump → quarantine at ceiling), and a quarantined id must
//     then read as CONSUMED at dispatch so it stops being re-picked.
//   - wave-planner-pass-scope-prune    → the wave SEED must drop carried-over
//     committed ids that the inbox lifecycle has already consumed.
//
// (workspace-hygiene-s5-wiring-shadow-default was DROPPED by triage as
// already-landed — no predicate here, per R9.3 predicates-bind-to-committed-work.)
//
// What is actually broken today (verified in this worktree, not assumed):
//
//  1. cmd_loop.go's WAVE branch (cmd_loop.go:596-604) only COUNTS failed lanes
//     and logs "N/M lanes ok". The sequential branch (cmd_loop.go:722-732) is
//     the ONLY caller of inboxmover.ApplyCycleOutcome's FAIL path, and
//     fleet.Result{Index,ExitCode,Err} carries neither the lane's cycle number
//     nor its workspace — so nothing can apply a lane's verdict from there.
//     Fleet-dispatched work therefore never bumps failure_count and the S5
//     retry ceiling is structurally unreachable (batch-14: cycles 1137/1139/
//     1142/1143 all FAILed on the same ids with failure_count stuck at 0).
//     The PASS half already lives INSIDE the cycle process
//     (internal/phases/ship/postship.go:188) — the FAIL half must be its
//     symmetric, equally importable sibling. Predicates 002/003 pin that seam.
//  2. inboxmover.ResolveDispatchState (dispatchstate.go:41-58) classifies
//     inbox root / processing / processed / rejected / retry — but NOT
//     quarantine/. A quarantined id falls through to StateUnknown, which the
//     dispatch freshness gate fails OPEN on (cmd_loop_wave.go:323-324) — so
//     quarantine, even once reachable, would not stop re-dispatch. Predicate
//     001 pins it.
//  3. triagecap.WidenTopNToFleetWidth (topn_width.go:96-97) copies the
//     carried-over `committed` slice through VERBATIM, and SelectWaveSeedMenus
//     (lane_menu.go:103-109) hands it straight to the wave plan. An id already
//     consumed at an earlier wave is re-pinned into a later lane-scope.json —
//     the cycle-1116 re-pin of tdd-topn-binding-gate (consumed at cycle-1113).
//     Predicate 004 pins it.
//
// Predicate strategy — every predicate DRIVES the system under test over an
// isolated temp tree and asserts on the resulting on-disk lifecycle state or
// return value. There is not one source-grep assertion in this file (the
// cycle-85 degenerate-predicate ban): adding a magic string to a source file
// greens nothing here.
//
// Diversity: 001 asserts a state classification, 002 the positive bump +
// quarantine transition, 003 the NEGATIVE half (uncommitted menu ids and
// system-level failures must stay inert — the anti-over-quarantine guard), 004
// the prune plus its fail-open edge (an id with no lifecycle evidence at all
// must survive).
```

### `go/acs/cycle1180/predicates_test.go:231` — above `func TestC1180_002_LaneFailureBumpsAndQuarantines(t *testing.T) {`

```text
// TestC1180_002_LaneFailureBumpsAndQuarantines is the cycle-1180 CRUX for
// wave-lane-task-quarantine-dead.
//
// It drives the importable failure-closeout seam — the symmetric sibling of the
// PASS half already living inside the cycle process (phases/ship/postship.go) —
// over a temp project, three times in a row against a ceiling of 3, and asserts
// the durable lifecycle actually moves:
//
//	FAIL #1 → failure_count 1, released to the inbox root (still re-pickable)
//	FAIL #2 → failure_count 2, still at the root
//	FAIL #3 → at the ceiling ⇒ the item is in .evolve/inbox/quarantine/, GONE
//	          from the root, and reported in OutcomeResult.Quarantined
//
// A seam that merely releases (today's whole-batch behavior) fails at FAIL #1;
// one that bumps but never routes to quarantine fails at FAIL #3. Nothing here
// passes on an unfixed tree, and no source string can green it.
```

### `go/acs/cycle1180/predicates_test.go:294` — above `func TestC1180_003_UncommittedMenuAndSystemFailuresStayInert(t *testing.T) {`

```text
// TestC1180_003_UncommittedMenuAndSystemFailuresStayInert is the anti-over-
// quarantine half of wave-lane-task-quarantine-dead (its menu-semantics guard).
//
// Since PR #366 a wave lane CLAIMS a whole menu but triage commits only a subset.
// Bumping the whole menu on FAIL would walk healthy backlog toward quarantine on
// failures of an unrelated task — the exact inverse defect. Two negatives:
//
//	(a) an id present in the lane's inbox but ABSENT from triage-decision.json's
//	    top_n must end with failure_count 0 and stay at the inbox root;
//	(b) a SYSTEM-level failure (ADR-0072 S3 — quota storm, forged verdict) must
//	    bump NOTHING, not even the committed id: it is not the task's fault.
```

### `go/acs/cycle1180/predicates_test.go:367` — above `func TestC1180_004_WaveSeedPrunesConsumedCommittedIds(t *testing.T) {`

```text
// TestC1180_004_WaveSeedPrunesConsumedCommittedIds pins
// wave-planner-pass-scope-prune against the real seed seam.
//
// SelectWaveSeedMenus takes the prior decision's `committed` candidates and
// (via WidenTopNToFleetWidth) copies them into the wave plan verbatim. An id
// consumed at an earlier wave is therefore re-pinned into a later
// lane-scope.json — cycle-1116 re-pinned tdd-topn-binding-gate after cycle-1113
// consumed it. The criterion: the SEED itself must drop candidates the inbox
// lifecycle has already consumed (processed/, and quarantine/ once 001 lands),
// so the plan artifact is honest rather than relying on the launch-time gate.
//
// Fail-open edge, asserted in the same predicate: a carried-over id with NO
// lifecycle evidence at all (not inbox-backed — a synthetic or externally
// sourced card) must SURVIVE the prune. A prune that drops everything it cannot
// resolve would starve every wave, so this negative is load-bearing.
```
