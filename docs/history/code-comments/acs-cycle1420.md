# Comment history: `acs/cycle1420`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1420/predicates_test.go:3` — above `package cycle1420`

```text
// Package cycle1420 encodes the cycle-1420 ACS predicates for
// verify-disposition-contract-fix-and-retire-inbox-item.
//
// The task is a VERIFICATION-AND-CONSUME step (the cycle-1410 shape) for the
// inbox item defect-disposition-contract-unsatisfiable: cycles 1397/1399/1400
// FAILed because agents authoring <workspace>/defect-dispositions.json had only
// a PROSE schema to work from, guessed `evidence` as a JSON array against a
// `string`-typed field, and the gate could not read the claims it was grading.
// PR #422 (5f405e92) and PR #426 (59579452) landed the three-part fix: a
// literal legal example in agents/evolve-auditor.md single-sourced with
// docs/architecture/continuation-defect-ledger.md, tolerant string-OR-array
// evidence unmarshal, and fail-closed rejection of every other shape.
//
// Predicates 001/002 re-prove that behavior live by driving the production
// reader (they are pre-existing GREEN by design — the fix is already merged and
// this cycle's job is to VERIFY, not re-implement). Predicates 003/004/005 are
// the RED contract: the durable consumed record carrying the verification
// evidence does not exist yet, and the item is still drawable from the live
// inbox.
```

### `go/acs/cycle1420/predicates_test.go:35` — above `const auditPkg = "./internal/phases/audit"`

```text
// auditPkg is the ONE named package holding the disposition contract's reader
// and its regression pins. Deliberately not `./...` and not `./internal/core`:
// a multi-package sweep or a known-slow suite inside a cycle predicate is the
// banned flaky shape (cycles 1173/1175/1178 false-REDs under fleet load). Every
// invocation below is additionally narrowed with -run.
```

### `go/acs/cycle1420/predicates_test.go:90` — above `func stateRoot(t *testing.T) string {`

```text
// stateRoot resolves the STATE root (MAIN, even from a worktree — issue #12),
// where .evolve/ runtime data such as the live inbox lives. The acs suite
// exports EVOLVE_PROJECT_ROOT for exactly this. Absent it, a live-inbox
// assertion would read the worktree's committed copy and pass vacuously, so
// this skips loudly rather than asserting on the wrong tree.
```

### `go/acs/cycle1420/predicates_test.go:249` — above `sha := strings.Fields(raw)[0]`

```text
// Take the first whitespace-delimited token so a "59579452 (#426)" style
// value still resolves.
```
