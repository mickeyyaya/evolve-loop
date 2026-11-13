# Build Explanation — Cycle 2350

## Build Binding
- Cycle: 2350
- Base SHA: 59d9e50ba5e7d38bc03b345e8d5c71c499b7f91b

## Summary
The submit-verify re-send wait now ends when the context is cancelled. The wait loop of the nudge site now counts the re-send window as elapsed time.

## Rationale
The wait used `deps.Sleep`, which cannot see the context. A cancelled run stayed in the backoff for up to 30 seconds and can send one more Enter. The window also took wall time that the loop never counted, so the artifact timeout ran late.

## Changed Areas
- `go/internal/bridge/driver_tmux_submitverify.go` — adds `sleepUnlessCancelled`, a context check before each re-send, and a `Waited` field on the outcome.
- `go/internal/bridge/driver_tmux_wait_disposition.go` — stores the re-send window of the nudge in the wait state.
- `go/internal/bridge/driver_tmux_wait_state.go` — adds the window field and `takeResendWindowS`.
- `go/internal/bridge/driver_tmux_wait.go` — adds the taken window to the elapsed counter of each loop step.
- `go/internal/bridge/driver_tmux_submitverify_cancel_test.go` — tests the cancel latency, the no-send-after-cancel rule and the elapsed accounting.
- `go/acs/cycle2350/predicates_test.go` — predicates that run those tests.

## Design Decisions
The sleep runs in a goroutine and the caller selects on the context. This keeps the injected `Sleep` and its recorded durations unchanged. A step that completes after a cancel still returns not_verified.

## Verification
The four predicates pass, and the ACS suite reports red=0. The bridge package has 9 failing tests that concern codex and agy models, the seat config directory and a sandboxed `ps`. All 9 fail on the base commit too, and none touch this change.

## Compatibility
The signature of `verifySubmitted` is unchanged. The outcome gains one field.

## Limitations
A cancelled sleep goroutine ends only when its sleep ends, at most 30 seconds later. The window counts whole seconds.
