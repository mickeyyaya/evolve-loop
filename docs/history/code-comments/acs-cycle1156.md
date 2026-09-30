# Comment history: `acs/cycle1156`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1156/predicates_test.go:3` — above `package cycle1156`

```text
// Package cycle1156 materialises the acceptance criteria for the three tasks
// triage COMMITTED to this fleet lane (triage-report.md `## top_n`):
//
//   - inboxmover-promote-mkdir-fail-loud  → 001, 002, 003
//   - wave-lane-task-quarantine-dead      → 004, 005, 006
//   - menu-pass-promotes-committed-ids    → 007, 008
//
// The fourth lane-scope id (workspace-hygiene-s5-wiring-shadow-default) was
// DEFERRED by triage and therefore gets ZERO predicates here (R9.3 floor-binding:
// predicates bind only to triage-committed work; a deferred-floor predicate
// starves the committed tasks — cycle-280).
//
// # Why these three are one lifecycle contract
//
// The inbox item for wave-lane-task-quarantine-dead states it explicitly: "one
// lifecycle seam handling PASS-promote + FAIL-bump covers both". Today there are
// two half-lifecycles and a hole in the middle:
//
//   - PASS side (menu-pass-promotes-committed-ids): promotion is agent-driven, so
//     cycle-1147 shipped three menu items in one commit and promoted NONE of them
//     — processed/cycle-1147/ is empty and all three re-entered the backlog.
//   - FAIL side (wave-lane-task-quarantine-dead): the failure drain is the only
//     bumpFailureCount caller and it walks ONLY
//     processing/cycle-N/. Wave lanes never claim their ids into processing/, so
//     the ADR-0072 S5 retry ceiling is structurally unreachable for fleet work
//     (batch-14: four FAILs, failure_count never incremented).
//   - And when the underlying move fails, Promote (inboxmover.go:305-318) reports
//     the infrastructure failure as NoOp=true / nil error — the ship.sh "already
//     done" compat contract reused for a genuine non-delivery.
//
// # Contract pinned by these predicates (Builder: implement these exact signatures)
//
// Per Core Rule 5 (deterministic work belongs in code, not agent instructions),
// predicates 004-008 pin ONE exported lifecycle seam in package inboxmover rather
// than two parallel models (never_duplicate_centralize):
//
//	type CycleOutcome struct {
//	    Cycle        int      // cycle number
//	    Passed       bool     // true = PASS (promote), false = FAIL (bump/quarantine)
//	    CommittedIDs []string // triage-decision.json `## top_n` — the worked set
//	    CommitSHA    string   // ship SHA, PASS only ("" = no SHA prefix)
//	    Reason       string   // ledger reason ("" = default)
//	    Ceiling      int      // FailureThresholds.TaskRetryCeiling (FAIL only)
//	    SystemLevel  bool     // S3 system failure: NEVER quarantines (AC4)
//	}
//	func ApplyCycleOutcome(opts Options, oc CycleOutcome) (OutcomeResult, error)
//	func ClaimLaneScope(opts Options, cycle int, ids []string) ([]string, error)
//
// The predicates deliberately assert on the FILESYSTEM end state (where the item
// physically lands, and what its durable failure_count says) rather than on the
// returned OutcomeResult — the on-disk lifecycle IS the contract, and the Builder
// keeps freedom over the result struct's shape.
//
// # Predicate quality (cycle-85 ban)
//
// Every predicate below CALLS the production function and asserts on its return
// value, its error, or the artifact it moved on disk; 003 runs the real `evolve`
// binary as a subprocess and asserts on its exit code. None of them is a
// source-grep for a magic string, so none can be satisfied by adding a comment.
```

### `go/acs/cycle1156/predicates_test.go:122` — above `func testOpts(root string, stderr io.Writer) inboxmover.Options {`

```text
// testOpts returns inboxmover Options rooted at root with the landing gate
// stubbed to "landed". The real gate shells out to `git merge-base` and is
// fail-open on a non-git dir; stubbing it keeps the promote predicates asserting
// the LIFECYCLE rather than incidental git behaviour of a temp dir.
```

### `go/acs/cycle1156/predicates_test.go:383` — above `func TestC1156_006_committed_id_quarantines_at_ceiling(t *testing.T) {`

```text
// AC (RED, ceiling + AC4 edge): "at task_retry_ceiling the item moves to
// quarantine and is not re-seeded", and a SYSTEM-level failure never quarantines
// (ADR-0072 S3 precedence).
```

### `go/acs/cycle1156/predicates_test.go:438` — above `func TestC1156_007_passing_cycle_promotes_exactly_committed_ids(t *testing.T) {`

```text
// AC (RED): "a PASSing cycle whose triage committed N ids leaves
// processed/cycle-<N>/ holding exactly those N items; uncommitted menu ids stay
// in inbox root."
//
// Cycle-1147's shape verbatim: a menu ships several items in ONE commit, so the
// promote must be driven by the committed-id set in code — not by an agent that
// promoted nothing and left all three items to be re-offered by the very next
// triage (the verified-stale-drop burn that cost cycles 1131 and 1134). The
// "exactly N" count assertion is what rejects a promote-the-whole-menu shortcut.
```
