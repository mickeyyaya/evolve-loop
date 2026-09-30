# Comment history: `acs/cycle1717`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1717/predicates_test.go:3` — above `package cycle1717`

```text
// Package cycle1717 materialises the acceptance criteria for
// task-contract-refs-parity: the Task Contract's refs (core.taskItemRefs) and
// the committed set (core.ContractTaskIDs) are ONE projection — committedset —
// and ctx fleet_scope_paths only places a committed member's inbox record.
//
// 001 drives the real core.Orchestrator.RunCycle with a request-context fleet
// scope and no lane pin while triage decides a different id, and asserts every
// dispatched tdd/build/audit Task Contract names exactly ContractTaskIDs.
// 002-004 bind the frozen in-package core tests (taskItemRefs is unexported)
// by their `--- PASS:` markers from ONE -race, -run-narrowed run of
// ./internal/core; 003 also walks taskItemRefs's static call graph; 004 runs
// go vet.
//
// Reachability probe (cycle-644 rule): internal/core already imports
// committedset (task_contract.go), and acs/cycle1717 -> core/fixtures is a
// leaf import, so no pin here can demand an import cycle.
```
