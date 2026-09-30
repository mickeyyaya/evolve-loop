# Comment history: `acs/cycle1064`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1064/predicates_test.go:3` — above `package cycle1064`

```text
// Package cycle1064 materialises the cycle-1064 acceptance criteria for the two
// fleet-scoped tasks pinned to this lane (inbox item `ship-stage-explicit-paths`,
// operator_note items 2 and 3):
//
//   - dedicated-manifest-gate-error-code → the enforce-mode manifest-gate block
//     must carry a DEDICATED core.CodeManifestGate, not the generic
//     CodeGitStageFailed a real failing `git add` also emits (which is classified
//     TRANSIENT, so an integrity block currently inherits a retry-friendly class).
//   - manifest-gate-policy-wiring → `.evolve/policy.json` gates.manifest_gate must
//     resolve through policy.GatesConfig() AND thread into the ship phase's
//     Options.ManifestGate, so the dial is activatable without a code edit.
//     Today Options.ManifestGate is never assigned at the sole production
//     construction site (ship.go runNative) — the gate is permanently shadow.
//
// Predicate strategy — every predicate EXERCISES the system under test (the
// cycle-85 degenerate-predicate ban): 001/003 call the real constructors and the
// real policy resolver in-process; 002/004 shell the package's behavioural unit
// tests, which drive reconcileManifest and the PhaseRequest→Options translation
// through their production paths. No predicate asserts on source text.
```
