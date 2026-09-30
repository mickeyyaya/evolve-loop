# Comment history: `acs/cycle1159`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1159/predicates_test.go:3` — above `package cycle1159`

```text
// Package cycle1159 encodes the cycle-1159 acceptance criteria: three
// "landed test-green but never wired into the real call site" defects from the
// fleet-scoped backlog.
//
//	001/002 — workspace-hygiene S5: runGCHook must default an absent gc.mode to
//	          shadow and must actually invoke the S4 worktree/branch sweep.
//	003     — menu pass must preserve an already-committed id prefix.
//	004     — the cycle-962 carry-forward classifier must stay wired into the
//	          fleet-rebase recovery path AND keep its three-verdict semantics.
//
// Every predicate here exercises the system under test (runs the hook, calls the
// selector, classifies a real git repo) — no source-grep-only assertions.
```
