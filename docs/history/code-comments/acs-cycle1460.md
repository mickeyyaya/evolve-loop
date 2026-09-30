# Comment history: `acs/cycle1460`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1460/predicates_test.go:3` — above `package cycle1460`

```text
// Package cycle1460 holds the cycle-1460 ACS predicates for the fleet-assigned
// inbox item tokenopt-role-scoped-instruction-digests.
//
// Two tasks (see .evolve/runs/cycle-1460/scout-report.md and api-contract.md):
//
//   - digest-materialize-role-instructions: the pure cycle-1391 projector
//     (digest.ProjectDigest) has no production caller. This task adds
//     digest.Materialize/Result/Outcome plus the runner-side integration seam
//     so BaseRunner.Run derives the dispatched instruction body from the
//     role-tagged SSOT source, excludes untagged and other-role content, and
//     fails BEFORE bridge.Launch on an unterminated marker.
//   - digest-shadow-size-parity: digest.ShadowRecord/NewShadowRecord plus
//     runner.FormatDigestShadowLog record full-versus-digest byte counts and a
//     parity verdict for every dispatch, and no empty, malformed, or
//     non-reducing projection may claim a saving or replace the live prompt.
//     No profile default flip happens until that baseline exists.
//
// Every predicate below drives real production code — BaseRunner.Run, the
// digest package's exported functions, or the real profiles loader. None is a
// source grep (the cycle-85 degenerate-predicate failure mode); predicate 006
// carries an explicit config-check waiver for its declarative half.
```
