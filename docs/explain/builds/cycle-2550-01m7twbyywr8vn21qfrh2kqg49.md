# Build Explanation — Cycle 2550

## Build Binding
- Cycle: 2550
- Base SHA: f25d49e4ae89c9d1c528d1c4fbc1687743635999

## Summary
`verifySubmitted` now reads its resend deadlines from `Deps.SubmitResendSchedule` instead of the hardcoded `submitVerifyBackoff` array and `submitVerifyMaxResends` constant. When `Deps.SubmitResendSchedule` is empty, it defaults to offsets of 2, 5, 10, 20, and 30 seconds from the start of verification.

## Rationale
The old backoff array sent an immediate resend at 0 s and its last Enter at 19.5 s. A Claude Code pane that ignored Enter for 20 s after a large paste therefore ended `submit_wedged`. Deadlines from verification start let a slow-booting seat accept a resend at or after 20 s. Injecting the slice through `Deps` also avoids mutating package globals in tests.

## Changed Areas
- `go/internal/bridge/engine.go` — adds `SubmitResendSchedule []time.Duration` to `Deps`. Callers and tests can now inject resend offsets without global state.
- `go/internal/bridge/driver_tmux_submitverify.go` — replaces `submitVerifyBackoff` and `submitVerifyMaxResends` with `defaultSubmitResendSchedule` (2, 5, 10, 20, 30 s) and `submitResendScheduleFor`. `verifySubmitted` waits until each offset before it sends Enter, then sleeps `submitVerifySettle` (500 ms) before it captures the pane.
- `docs/architecture/packages/internal-bridge.md` — updates the submit-verification package notes in STE. The note records `Deps.SubmitResendSchedule`, the 2, 5, 10, 20, 30 s default, the `submitVerifySettle` post-resend settle, `submitted_after_resend`, the `bridge.submit_resend_schedule_s` console follow-up, and the test evidence.
- `go/internal/bridge/driver_tmux_submitverify_cancel_test.go` — replaces the `withBackoff` global swap with `Deps.SubmitResendSchedule` and enables `t.Parallel()` on the three cancel tests.
- `go/internal/bridge/parallel_serial_tests_test.go` — removes those three cancel tests from `serialTests` now that they no longer mutate package state.
- `go/internal/bridge/driver_tmux_submit_settle_test.go` — updates `TestVerifySubmitted_OutwaitsALateEnterUnderBootLoad` to match the 5-resend 30 s default schedule and drops the obsolete `submitVerifyBackoff` loop from `TestPasteTiming_NeverEqualsTheArtifactWaitInterval`.
- `go/internal/bridge/driver_tmux_repl_submitwedge_shortcircuit_test.go` — updates the wedge reason to `len(defaultSubmitResendSchedule)` and gates `artifactWaitPolls` on `STILL unsubmitted`. It also adds `TestEngineLaunch_SubmitResendScheduleReachesVerifySubmitted`.
- `go/internal/bridge/driver_tmux_repl_submitverify_test.go` — replaces `submitVerifyMaxResends` with `len(defaultSubmitResendSchedule)`.
- `go/internal/bridge/driver_tmux_delivery_failure_test.go` — replaces `submitVerifyMaxResends` with `len(defaultSubmitResendSchedule)`.

## Design Decisions
`verifySubmitted` interprets `Deps.SubmitResendSchedule` as cumulative offsets from verification start (`delay := schedule[resends] - waited`) rather than inter-resend deltas. After each `SendKeys`, it waits `submitVerifySettle` (500 ms) before `CapturePane`. That settle lets a pane with render latency show an accepted submit before the next resend or `submit_wedged`. Policy key wiring (`bridge.submit_resend_schedule_s` in `go/internal/policy` and `go/internal/adapters/bridge`) is outside lane authority and remains a console follow-up.

## Verification
- Targeted `internal/bridge` tests (`TestVerifySubmitted|TestTmuxREPL_PromptSubmitWedged|TestNudgeSite_ResendWindow|TestPasteTiming|TestEngineLaunch_SubmitResendSchedule|TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial`) pass.
- `go test -tags acs -count=1 ./acs/cycle2550 ./acs/cycle2350 ./acs/cycle1562` passes.
- `go test -count=1 ./internal/testaudit/` passes.
- `evolve docs ste-lint --strict --changed f25d49e4ae89c9d1c528d1c4fbc1687743635999 --added` reports 0 findings.

## Compatibility
`verifySubmitted` keeps its function signature and returns `submitVerifyOutcome` with the same result vocabulary. An empty `Deps.SubmitResendSchedule` preserves default behavior with the new 2, 5, 10, 20, 30 s schedule.

## Limitations
`bridge.submit_resend_schedule_s` is not wired from `policy.json` in `go/internal/adapters/bridge` because that package is a protected path for lane builds. The live 4-seat 55 KB claude run requires a live bridge on the console.
