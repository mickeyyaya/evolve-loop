# Build Explanation — Cycle 2492

## Build Binding
- Cycle: 2492
- Base SHA: 5db58d6b91d602b28eea4674bfd2dcbb3d18e095

## Summary
The shadow-batch overlap test now proves overlap with a rendezvous. It no longer uses a fixed sleep.

## Rationale
The old runner slept 40 ms and relied on its peer entering Run in that window. A slow scheduler made the enforce control fail. A rendezvous makes the first member wait until its peer enters Run, with a bound of 10 s.

## Changed Areas
- `go/internal/core/evaluate_batch_shadow_test.go` — each runner closes an entered channel and waits for its peer, so overlap no longer depends on timing.

## Design Decisions
The enforce control waits up to 10 s, which is above the 5 s floor. The shadow run waits 50 ms. Serial dispatch cannot overlap at any wait length, so the short wait cannot cause a false failure. A concurrent dispatch makes the peer enter during the wait, and the test turns red.

## Verification
The shadow tests pass with -race -count=50 at GOMAXPROCS=1. The ACS predicates of cycle 2492 cover the slow peer, the sleep ban, the concurrent mutant and the file scope.

## Compatibility
Only a test file changed. No production behavior changes.

## Limitations
Detection of a concurrent shadow dispatch needs the peer to enter within 50 ms. A slower peer can only hide the mutant. It cannot fail a correct run.
