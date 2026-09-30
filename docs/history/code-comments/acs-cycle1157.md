# Comment history: `acs/cycle1157`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1157/predicates_test.go:3` — above `package cycle1157`

```text
// Package cycle1157 materialises the acceptance criteria for the single task
// triage COMMITTED to this fleet lane (triage-report.md `## top_n`):
//
//   - inboxmover-promote-mkdir-fail-loud → 001-005
//
// No other id was assigned to this lane and nothing was deferred, so there is no
// deferred-floor predicate here (R9.3 floor-binding: predicates bind only to
// triage-committed work — cycle-280).
//
// # Continuation context (READ THIS FIRST, Builder)
//
// Cycle 1157 continues cycle 1156 under ADR-0076: this branch carries 1156's
// salvage snapshot (9effecb2), which already inverted the PRODUCER half of the
// contract — Promote (inboxmover.go:314-326) now returns ErrMvFailed on a
// destination mkdir failure instead of the (NoOp=true, nil) ship.sh-compat lie.
// Predicates 001, 002, 004 and 005 therefore start GREEN and are REGRESSION
// LOCKS on salvaged work: their job is to make an accidental revert loud, and
// the test-report records them as pre-existing GREEN rather than claiming a RED
// this cycle did not produce.
//
// Predicate 003 is the genuine RED. Making Promote fail loud only helps where a
// caller actually READS the error, and one caller still throws it on the floor:
//
//	// inboxmover.go:697 (releaseCycleProcessing, ADR-0072 S5 quarantine path)
//	if pr, pErr := Promote(opts, taskID, "quarantine", ...); pErr == nil && !pr.NoOp {
//	    ... quarantine bookkeeping ...
//	}
//
// pErr is bound and never inspected. When quarantine's destination mkdir fails,
// the item silently falls through to the ordinary release: it returns to the
// inbox root, the next triage re-picks the exact poison task the S5 ceiling
// exists to park, and NOTHING anywhere — stderr, ledger, or result — says the
// quarantine was attempted and failed. That is the same swallow the task names,
// one call site downstream of the fix, and it is the last one in the package
// (outcome.go:74 and ReconcileSuperseded both propagate correctly).
//
// The contract 003 pins is loud-but-fail-open, deliberately: the drain must
// still release the item (a quarantine mkdir failure that ALSO strands the file
// in processing/ would be a worse defect than the silent one), but the failure
// must reach the cycle's stderr naming the task and the quarantine attempt.
// Predicate 004 is its negative twin — a Builder who unconditionally logs an
// error to satisfy 003 fails 004.
//
// # Predicate quality (cycle-85 ban)
//
// Every predicate below CALLS the production function and asserts on its
// returned error, the diagnostic it emitted, or where the item physically
// landed on disk; 005 runs the real `evolve` binary and asserts its exit code.
// None is a source-grep for a magic string, so none can be satisfied by adding
// a comment or a literal.
```

### `go/acs/cycle1157/predicates_test.go:263` — above `func TestC1157_003_quarantine_promote_failure_is_surfaced(t *testing.T) {`

```text
// AC3 (RED — the remaining swallow): releaseCycleProcessing's ADR-0072 S5
// quarantine path binds Promote's error and never inspects it
// (inboxmover.go:697, `pErr == nil && !pr.NoOp`). When the quarantine mkdir
// fails, the poison item silently falls back to the ordinary release and the
// next triage re-picks the exact task the ceiling exists to park.
//
// Contract: loud, but still fail-open. The drain MUST keep releasing the item
// (stranding it in processing/ would be a worse defect), and the failed
// quarantine MUST reach the cycle-visible stderr naming the task id and the
// quarantine attempt.
```
