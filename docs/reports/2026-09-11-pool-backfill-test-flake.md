# Pool backfill test flake investigation

Date: 2026-09-11

## Outcome

The Ubuntu race-and-coverage job for pull request #550 exposed a pre-existing
synchronization defect in
`TestDispatchPoolIteration_BackfillsReplacementWhileSiblingStillRunning`. The
rolling pool implementation preserved its width, ownership, and backfill
invariants. The test inferred the pool's serial admission decisions from the
arrival order of concurrent launch callbacks, which Go does not guarantee.

The repaired fixture holds both initially admitted lanes until their callbacks
have been observed. It then releases B and verifies that C replaces B while A
remains blocked. This directly tests rolling backfill without depending on
scheduler order. No runtime behavior changed.

## Failure evidence

GitHub Actions run `34560302348`, attempt 1, failed in the Ubuntu
`test (race + cover, incl. integration tier)` step:

```text
--- FAIL: TestDispatchPoolIteration_BackfillsReplacementWhileSiblingStillRunning
cmd_loop_pool_test.go:194: initial fill dispatched map[B:true C:true], want exactly {A,B}
```

The bridge refactor in pull request #550 does not modify `go/cmd/evolve` or
`go/internal/fleet`, so the failure was classified separately before any retry.
The same failure class had already been corrected in the direct fleet test on
2026-09-09, but the dispatcher-level copy retained the old fixture.

## Root cause

`RunPool` reserves initial lanes in backlog order. For each lane it claims the
files and increments the running count before starting the launch goroutine.
The goroutine then signals an unbuffered `started` channel immediately before
calling the injected launch function.

That channel creates a happens-before edge only through the send and receive.
It does not order the launch function's first operation. `runtime.Gosched` is a
scheduling hint and creates no additional synchronization. Ubuntu therefore
exercised this legal interleaving:

1. The pool reserves A; A signals `started` and is paused before its callback.
2. The pool reserves B; B's callback reports B and returns.
3. The pool observes B's completion and reserves replacement C.
4. C's callback reports C before A's callback reports A.

A is still counted as running and its files remain claimed throughout. When C
is admitted, B has completed, so the target width is still respected. The
observed callback order does not show an admission, ownership, or backfill bug.

## Contract-preserving repair

The dispatcher wiring test now mirrors the direct `internal/fleet` test:

- A and B each report callback entry and then block.
- The test observes the complete initial set `{A, B}`.
- The test releases B while A stays blocked.
- The test requires C to start before A is released.
- Cleanup-safe, idempotent release functions prevent blocked goroutines when an
  assertion fails.

The assertion still distinguishes `RunPool` from the wave barrier: a wave
implementation cannot launch C until A is released. Production comments now
describe callback entry as scheduler-dependent and no longer claim that
`runtime.Gosched` creates an ordering guarantee.

## Review and verification

Architecture and Go concurrency reviewers independently classified the change
as a fixture repair and recommended the B release gate. They also identified
the overstated production comments. The focused corrected test passed 1,000
race-enabled repetitions with `GOMAXPROCS=8`; repository gates are recorded in
the pull request before merge.
