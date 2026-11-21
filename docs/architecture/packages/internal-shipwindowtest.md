# internal/shipwindowtest

## Purpose

`internal/shipwindowtest` owns the test doubles of the ship-window heartbeat ([internal-shipwindow](internal-shipwindow.md), D4). The tests of `internal/shipwindow`, `internal/phases/ship` and `internal/core` share them.

## Design

- `Clock` is a fake clock. `Advance` moves it. The lease reads it through `Options.Now`.
- `Beats` is a manual ticker. `Ticker` fits `Options.Ticker`. `Tick` hands one tick to the heartbeat goroutine and returns when the goroutine takes it. `WaitStopped` waits until the goroutine stops its ticker, which proves the stop on `Release` and on cancel.
- `Probe` counts the polls of a waiter. `Alive` fits `Options.Alive`. `WaitPolls` waits for a number of polls. A test then knows that a waiter looked at the lease after a clock move, and it needs no sleep.
- The package does not import `internal/shipwindow`, so a test inside that package can use it.
- The test-duplicate ratchet counts equal helper bodies. One package avoids a copy in each of the three test packages.

## Invariants

- Each double fails the test with a message after five seconds of waiting, and it never blocks for ever. `TestWaits_GiveUpAfterTheLimitWithAMessage` pins it.
- `Tick` fails the test when no ticker exists, so a lease that starts no heartbeat is a red test. `TestBeats_TickWithoutATickerFailsLoudly` pins it.
- Every exported symbol is named in a real assertion: `TestClock_AdvanceMovesNowByTheStep`, `TestBeats_TickerHandsOutTicksAndReportsItsStopAndPeriod` and `TestProbe_CountsPollsThatAWaiterMakes`.

## Findings

None.
