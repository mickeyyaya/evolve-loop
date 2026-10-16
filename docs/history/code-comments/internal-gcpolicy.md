# Comment history: `internal/gcpolicy`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d13

### `go/internal/gcpolicy/gcpolicy.go:1` — above `package gcpolicy`

```text
// Package gcpolicy is the zero-dependency leaf holding the
// `.evolve/policy.json` `gc` block — the retention engine's CONFIG vocabulary,
// split from the engine itself (internal/gc).
//
// Why the split (cycle-1141, decoupling campaign — same shape as cyclestate
// and shiperr): internal/policy, the config SSOT, held `GC *gc.Policy`, i.e.
// config depended on an ENGINE. That inverted edge closed a real cycle the
// moment gc needed the phasecontract registry
// (policy → gc → phasecontract → phasespec → policy). Config now points at a
// leaf that points at nothing, and the engine re-exports these types as
// aliases so every existing `gc.Policy` call site is untouched.
```
