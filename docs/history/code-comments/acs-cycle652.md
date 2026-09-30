# Comment history: `acs/cycle652`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle652/predicates_test.go:3` — above `package cycle652`

```text
// Package cycle652 encodes the acceptance criteria for
// builder-task-binding-topn-gate (inbox weight 0.97, 8th recurrence of the
// wrong-task-build defect: cycles 282, 310, 522, 575, 577, 599, 640, 645).
//
// These predicates are BEHAVIORAL: each shells `go test` against the real
// go/internal/topngate package (the system under test) rather than grepping
// source, so a predicate greens only when the gate actually blocks/approves
// the right builds — not when a magic string is present. The white-box unit
// suite the Builder must turn GREEN lives at
// go/internal/topngate/{gate_test.go,reviewer_test.go,builder_authority_test.go}
// (copied forward from the preserved cycle-645 worktree per the escalation
// note "the next attempt should START from those tests").
```

### `go/acs/cycle652/predicates_test.go:46` — above `func TestC652_001_OutOfLaneBuildAdvisory(t *testing.T) {`

```text
// TestC652_001_OutOfLaneBuildBlocked binds AC-1: a build report claiming a
// slug outside triage ## top_n is surfaced as a LOUD ADVISORY (logged reason,
// approved) — the committed set is the binding authority.
// POLICY CHANGE 2026-07-22 (operator-directed, cycles 916 + 1012): the
// out-of-lane label check is now ADVISORY — both recorded fatal rejections
// discarded CORRECT work over label drift between two LLM strings, with zero
// true-fraud catches. The binding authority is triage's committed set; scope
// verification (deliverable files vs committed item scope) is the queued
// fraud guard. These predicates now bind the ADVISORY contract.
```

### `go/acs/cycle652/predicates_test.go:75` — above `func TestC652_003_ReplayCycle640ShapeAdvisory(t *testing.T) {`

```text
// TestC652_003_ReplayCycle640ShapeBlocksBeforeAudit binds AC-3: replaying the
// cycle-640 shape (triage=statefile task, build=token-resolver task) passes
// with a loud drift advisory — see the 2026-07-22 policy note on AC-1; the
// replay test itself documents why the fatal form was retired.
```
