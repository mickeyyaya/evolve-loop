# Comment history: `acs/cycle1168`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1168/predicates_test.go:3` — above `package cycle1168`

```text
// Package cycle1168 holds the cycle-1168 ACS predicates.
//
// Cycle-1168 committed ONE task (triage `## top_n`):
//
//  1. close-out-evaluate-batch-retry-parity-tracker (fleet_scope: evaluate-batch-retry-parity)
//
// The deferred item (reevaluate-paralleleval-stageoff-soak) gets ZERO predicates
// per R9.3: predicates bind only to triage-committed work.
//
// The task is a tracker correction: the code-audit README still lists
// `evaluate-batch-retry-parity` as an OPEN blocker ("missing optionalInfraSkip
// — blocks the enforce flip") although the parity hooks landed in cycle-1166
// (retry_opts.go: evaluateBatchRetryOpts wires both degrade predicates and
// dispatchRunnerWithRetry delegates to the shared retry core). A stale row keeps
// re-surfacing resolved work into future fleet scopes.
//
// Predicate shape. The doc row is the deliverable, so its two predicates assert
// the artifact directly (positive row state + the anti-no-op negative that RED-
// fails on the untouched repo). The two claim-substantiation predicates EXERCISE
// the system under test by running the cycle's parity unit tests against the
// worktree build — the tracker may only be marked resolved if the behavior it
// tracked is actually green, so "annotate the row" alone cannot green this suite.
```

### `go/acs/cycle1168/predicates_test.go:66` — above `func TestC1168_001_TrackerRowRecordsResolution(t *testing.T) {`

```text
// TestC1168_001_TrackerRowRecordsResolution — AC-1, positive half.
//
// The item's table row must read as RESOLVED and must cite where the resolution
// lives, so the next scout that reads this tracker can verify the claim without
// re-deriving it. Any of the accepted evidence tokens satisfies the citation
// (the wiring is in retry_opts.go, reached from evaluate_batch.go, landed by
// cycle-1166 and closed out by cycle-1168) — the predicate pins that a citation
// EXISTS on the row, not one particular spelling of it.
```

### `go/acs/cycle1168/predicates_test.go:97` — above `func TestC1168_002_TrackerNoLongerClaimsOpenBlocker(t *testing.T) {`

```text
// TestC1168_002_TrackerNoLongerClaimsOpenBlocker — AC-1, NEGATIVE half
// (the anti-no-op signal: this predicate FAILS on the untouched repo, so a
// no-op cycle cannot green the suite).
//
// Two stale claims must be gone: the row's defect description ("missing
// optionalInfraSkip" / "blocks the enforce flip") and the sequencing paragraph's
// open constraint ("<item> gates the parallel-evaluate enforce flip"). Absence
// is asserted with FileNotContains, never an inverted FileContains (the
// cycle-352 broken-predicate incident).
```

### `go/acs/cycle1168/predicates_test.go:121` — above `func TestC1168_003_ResolutionClaimIsTrue_ParitySuiteGreen(t *testing.T) {`

```text
// TestC1168_003_ResolutionClaimIsTrue_ParitySuiteGreen — substantiates AC-1's
// claim behaviorally, and is AC-3's guard against a re-fix.
//
// The tracker may be marked RESOLVED only if the retry parity it tracked really
// holds. This runs the item's own RED contract (the three dispatch-parity tests,
// including the NON-skippable-error-still-fatal negative) plus the amplify suite
// (wrapped-error match, non-infra never matches, mandatory overrides optional,
// floor phases never skip, pre-ship/ship-itself never skip) and the cycle-1166
// hook-set parity pins. A doc edit alone cannot make these pass; a regression in
// the parity code makes the RESOLVED annotation red-fail here.
```

### `go/acs/cycle1168/predicates_test.go:153` — above `func TestC1168_004_NoRefixOfParityWiring(t *testing.T) {`

```text
// TestC1168_004_NoRefixOfParityWiring — AC-3 (doc-only task: no code re-fix).
//
// The parity must remain a SINGLE delegated wiring, exactly as cycle-1166 left
// it: evaluateBatchRetryOpts wires each degrade predicate once, and
// dispatchRunnerWithRetry keeps ZERO inline hook calls (it delegates to the
// shared retry core). Counts are function-scoped AST reads, so they cannot be
// satisfied by adding text elsewhere in the file, and the ==0 arm fails LOUDLY
// on a renamed function rather than passing vacuously. A cycle that "re-fixed"
// the item by hand-rolling a second retry loop trips this.
```
