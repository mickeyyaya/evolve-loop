# Comment history: `acs/cycle1019`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1019/predicates_test.go:3` — above `package cycle1019`

```text
// Package cycle1019 materialises the acceptance criteria for the single
// fleet-scoped task adr0072-s5-task-quarantine (ADR-0072 S5: task-level failure
// memory + quarantine). S5 is the missing task-half of the ADR-0072 failure
// policy: the system-level halt (S3) and the orchestrator-judgment layer (S4)
// already shipped; S5 stops the loop from re-picking a poison inbox task
// forever by routing a task that has failed `task_retry_ceiling` times to
// `.evolve/inbox/quarantine/` (a sibling dir the triage scanner never walks)
// instead of releasing it back to the inbox root every cycle.
//
// Predicate strategy — every predicate EXERCISES the system under test (calls
// the real inboxmover/inboxbatch functions and asserts on the emitted files,
// ledger, and return values), never a source-grep of production code (the
// cycle-85 degenerate-predicate ban). Each fails RED on the current tree:
//
//   - 001 drives inboxmover.Promote(..., "quarantine", ...) and asserts the
//     item physically lands under .evolve/inbox/quarantine/ (AC1 routing). RED
//     now: "quarantine" is not yet a validStates target, so Promote errors and
//     never moves the file.
//   - 002 exercises the pure decision inboxmover.ShouldQuarantine at/above/below
//     the ceiling (AC1 count trigger). RED now: the function does not exist
//     (compile failure IS a valid RED per go/acs/README.md).
//   - 003 places a healthy item AND a poison item at the inbox ROOT (the S5
//     "released back every cycle" shape), quarantines the poison, and asserts
//     the real triage scanner inboxbatch.LoadDir(root) returns the healthy item
//     but NOT the poison (AC1 invisibility + AC2 siblings-keep-flowing). RED
//     now: the poison stays at root and LoadDir still returns it.
//   - 004 reads the emitted .evolve/ledger.jsonl after a quarantine and asserts
//     a durable record identifying WHY the item was quarantined (AC3 failure
//     diagnostic). RED now: no quarantine promote, no ledger line.
//   - 005 exercises ShouldQuarantine with the system-level (S3 floor) flag set
//     and asserts it NEVER quarantines even past the ceiling — the S3 halt
//     takes precedence over task quarantine (AC4). RED now: function absent.
//
// AC5 (go vet / -race on internal/inboxmover / no regression) is a
// manual+checklist disposition verified by the audit lane and CI, not a
// per-behavior predicate (see test-report.md).
```
