---
score_cap:
  - criterion: "Timeout arm never closes the events sink under a still-running watcher goroutine"
    max_if_missing: 7
    evidence: "cd go && go test -race -count=1 -run '^TestCoreAdapter_NoSinkCloseRaceOnTimeout$' ./internal/adapters/observer/"
  - criterion: "Normal <-done arm still closes the sink exactly once (no fd leak on the healthy path)"
    max_if_missing: 6
    evidence: "cd go && go test -race -count=1 -run '^TestCoreAdapter_SinkClosedOnNormalDone$' ./internal/adapters/observer/"
  - criterion: "closeSinkAfterWait preserves the nil-closer contract Start relies on"
    max_if_missing: 5
    evidence: "cd go && go test -race -count=1 -run '^TestCoreAdapter_CloseSinkAfterWait_NilCloserSafe$' ./internal/adapters/observer/"
  - criterion: "Observer adapter package is race-detector clean"
    max_if_missing: 6
    evidence: "cd go && go test -race -count=1 ./internal/adapters/observer/"
---

# Eval: Observer cancel path must never close the events sink on the watcher-timeout arm

> Pins the fix for the observer-sink-close-race inbox item (fable5 deep-scan,
> weight 0.92): `CoreAdapter.Start`'s cancel closure waits ≤10s for the watcher
> goroutine, and previously closed `sinkCloser` UNCONDITIONALLY after the
> select — so a watcher wedged past 10s (e.g. `newestActivity` walking a slow
> disk) could `emit()` → `Write` into a closed/reused fd (use-after-close
> race, `-race`-reportable). The fix (`closeSinkAfterWait`, landed on main via
> the cycle-669 commit `1d0e23ac`) closes only on the `<-done` arm and accepts
> a documented fd leak on the timeout arm. Cycle 688's triage re-committed
> this item as top_n; TDD verification found the contract already GREEN and
> pinned it here as a permanent regression entry. Source incidents: cycle-618
> scout finding; cycle-688 audit binding.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| timeout-no-close | Wedged watcher's sink never closed under it | 7/10 | `go test -race -run TestCoreAdapter_NoSinkCloseRaceOnTimeout` |
| done-closes-once | Healthy path closes exactly once | 6/10 | `go test -race -run TestCoreAdapter_SinkClosedOnNormalDone` |
| nil-closer-safe | Nil closer is a no-op, not a panic | 5/10 | `go test -race -run TestCoreAdapter_CloseSinkAfterWait_NilCloserSafe` |
| package-race-clean | Whole observer package `-race` green | 6/10 | `go test -race ./internal/adapters/observer/` |
