# Comment history: `acs/cycle1166`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1166/predicates_test.go:3` — above `package cycle1166`

```text
// Package cycle1166 holds the cycle-1166 ACS predicates.
//
// Cycle-1166 committed three tasks (triage `## top_n`):
//
//  1. evaluate-batch-retry-parity            (inbox weight 0.87)
//  2. infra-teardown-predicate-single-source (inbox weight 0.86)
//  3. spine-failopen-telemetry               (inbox weight 0.85)
//
// The deferred item (tokenopt-session-resume-on-retry) gets ZERO predicates per
// R9.3: predicates bind only to triage-committed work.
//
// Each predicate below EXERCISES the system under test by running the cycle's
// RED unit tests against the worktree build — no source-grep gaming. The unit
// tests themselves carry the negative / anti-no-op assertions (a degenerate
// "always degrade", a widened transient-only predicate, an always-firing
// counter each fail their own negative twin), so a predicate that greens here
// implies the real behavior, not a magic string.
```

### `go/acs/cycle1166/predicates_test.go:86` — above `"TestOptionalInfraSkip_GateAgreesWithIsOptionalSkippableError|" +`

```text
// (renamed 2026-08-24: gate now proves equivalence to the widened
// single-source IsOptionalSkippableError — cycle-1551)
```
