# Comment history: `acs/cycle1526`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1526/predicates_test.go:3` — above `package cycle1526`

```text
// Package cycle1526 encodes the acceptance criteria for cycle-1526's single
// committed task, `submit-verify-retro-paste` (triage-decision.json top_n[0]).
//
// The task's root-cause evidence — three recorded panes (cycles 1505, 1510,
// 1517) in which the driver's one-shot nudge sat UNSUBMITTED at the `❯` input
// line while every interaction record read "result":"no_effect" — is fixed by a
// submit-verify step on the driver's send path: verify the input line cleared,
// re-send Enter when it did not, bounded and loud.
//
// Every predicate here is BEHAVIORAL: it runs the real driver through
// Engine.LaunchArgs (the production entry point) inside the default Go suite and
// asserts on the `--- PASS:` line, never on source text. A source grep would go
// green the moment someone typed the word "submit-verify" into a comment.
```

### `go/acs/cycle1526/predicates_test.go:101` — above `func TestC1526_006_RedContractTracked(t *testing.T) {`

```text
// TestC1526_006_RedContractTracked — AC-6. The RED contract must be committed,
// not merely present on disk: an untracked (or gitignored) test file is silently
// dropped at ship and the whole gate evaporates (the cycle-92/93 lesson).
```
