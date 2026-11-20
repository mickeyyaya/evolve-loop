# Build Explanation — Cycle 2568

## Build Binding
- Cycle: 2568
- Base SHA: aa909f8688805f68950c2d1be8eb02d7330842ce

## Summary
A pane that is idle at its prompt with no deliverable now gets its review checkpoint and nudge after a short idle grace. It no longer waits for the full artifact interval.

## Rationale
The wait loop gated the checkpoint on the artifact interval only. An idle seat waited 300 s (1200 s for build) before the one-shot nudge. An idle clock that restarts on every busy tick is the smallest change. It reuses the existing busy read and the existing checkpoint path.

## Changed Areas
- `go/internal/bridge/driver_tmux_wait_idle.go` — holds the idle clock and the combined checkpoint-due test.
- `go/internal/bridge/driver_tmux_wait.go` — replaces the interval-only test with the combined `checkpointDue` test.
- `go/internal/bridge/driver_tmux_wait_interaction.go` — reads the busy state of each tick pane once per tick and feeds the idle clock.
- `go/internal/bridge/driver_tmux_wait_state.go` — adds the idle grace and the idle-since fields.
- `go/internal/bridge/engine.go` — adds `Deps.IdleCheckpointGraceS`.
- `go/internal/policy/dispatch.go` — adds `bridge.idle_checkpoint_grace_s`.
- `go/internal/bridge/transient_dwell_shortcircuit_test.go` — keeps the grace equal to the interval so the test still reaches the full interval.
- `docs/architecture/packages/internal-bridge.md` — describes the idle checkpoint.
- `docs/architecture/interaction-protocol.md` — describes the idle checkpoint.
- `docs/operations/runtime-reference.md` — documents the policy key.

## Design Decisions
The nudge stays one-shot: the early trigger is off after a nudge. An extend verdict restarts the idle clock. A grace above the interval never delays the checkpoint. The default is 120 s.

## Verification
The `TestIdleCheckpoint_*` tests in `go/internal/bridge` and the `TestIdleCheckpointGrace_*` tests in `go/internal/policy` cover the idle, busy, default, long-grace and in-grace cases.

## Compatibility
No policy key changes meaning. An unset key selects the default. A busy pane behaves as before.

## Limitations
A pane that shows idle for the whole grace while it still thinks can get an early nudge.

The key `bridge.idle_checkpoint_grace_s` is not wired to production yet. The adapter `productionEngineDeps` in `go/internal/adapters/bridge/bridge.go` does not pass it to `Deps.IdleCheckpointGraceS`, because that file is a protected path. The key has no effect until the console adds that one line. Production runs the compiled 120 s default.
